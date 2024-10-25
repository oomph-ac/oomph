package oomph

import (
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
	"github.com/oomph-ac/oomph/detection"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/sasha-s/go-deadlock"
	"github.com/sirupsen/logrus"
)

func init() {
	deadlock.Opts.Disable = true
	if os.Getenv("DEADLOCK_DEBUG") == "true" {
		deadlock.Opts.Disable = false
		deadlock.Opts.DeadlockTimeout = time.Second * 5
		deadlock.Opts.DisableLockOrderDetection = true
		deadlock.Opts.PrintAllCurrentGoroutines = true
	}
}

type Oomph struct {
	Log      *logrus.Logger
	Listener *minecraft.Listener

	settings OomphSettings
	players  chan *player.Player
}

type OomphSettings struct {
	Logger         *logrus.Logger
	StatusProvider *minecraft.ServerStatusProvider

	LocalAddress   string
	RemoteAddress  string
	Authentication bool

	ResourcePath             string
	RequirePacks             bool
	FetchRemoteResourcePacks bool
	EncryptionKey            string

	Protocols []minecraft.Protocol

	EnableSentry bool
	SentryOpts   *sentry.ClientOptions
}

// New creates and returns a new Oomph instance.
func New(s OomphSettings) *Oomph {
	var length = len(s.EncryptionKey)
	if length != 0 && length != 32 {
		panic("encryption key must be an empty string or a 32 byte string")
	}
	return &Oomph{
		Log:     s.Logger,
		players: make(chan *player.Player),

		settings: s,
	}
}

// Start will start Oomph! remoteAddr is the address of the target server, and localAddr is the address that players will connect to.
// Addresses should be formatted in the following format: "ip:port" (ex: "127.0.0.1:19132").
// If you're using dragonfly, use Listen instead of Start.
func (o *Oomph) Start() {
	defer func() {
		if r := recover(); r != nil {
			hub := sentry.CurrentHub().Clone()
			hub.Scope().SetTag("func", "oomph.Start()")
			hub.Recover(oerror.New(fmt.Sprintf("%v", r)))

			sentry.Flush(time.Second * 5)
		}
	}()

	s := o.settings
	if !s.Authentication {
		o.Log.Warn("XBOX authentication is disabled.")
	}

	if s.SentryOpts == nil {
		s.SentryOpts = &sentry.ClientOptions{
			Dsn:                "https://06f2165840f341138a676b52eacad19c@o1409396.ingest.sentry.io/6747367",
			EnableTracing:      os.Getenv("SENTRY_TRACE") == "true",
			ProfilesSampleRate: 1.0,
			TracesSampler: sentry.TracesSampler(func(ctx sentry.SamplingContext) float64 {
				return 0.125
			}),
		}
	}

	if s.EnableSentry {
		if err := sentry.Init(*s.SentryOpts); err != nil {
			panic("failed to init sentry: " + err.Error())
		} else {
			o.Log.Info("Sentry initialized")
		}
	}

	var statusProvider minecraft.ServerStatusProvider
	if s.StatusProvider == nil {
		p, err := minecraft.NewForeignStatusProvider(s.RemoteAddress)
		if err != nil {
			o.Log.Errorf("unable to make status provider: %v", err)
		}

		statusProvider = p
	} else {
		statusProvider = *s.StatusProvider
	}

	var resourcePacks []*resource.Pack
	if s.FetchRemoteResourcePacks {
		resourcePackFetch, err := minecraft.Dialer{
			ClientData:   login.ClientData{DefaultInputMode: 2, CurrentInputMode: 2, DeviceOS: 1, ServerAddress: s.RemoteAddress}, // fill in some missing client data
			IdentityData: login.IdentityData{DisplayName: "OomphPackFetch", XUID: "0"},
			IPAddress:    "0.0.0.0:0",
		}.Dial("raknet", s.RemoteAddress)
		if err != nil {
			o.Log.Errorf("unable to fetch resource packs: %v", err)
			resourcePacks = utils.ResourcePacks(s.ResourcePath) // default to resource pack folder if fetching packs from the remote server fails
		} else {
			resourcePacks = resourcePackFetch.ResourcePacks()
			resourcePackFetch.Close()
		}
	} else {
		resourcePacks = utils.ResourcePacks(s.ResourcePath)
	}

	if s.EncryptionKey != "" {
		for _, pack := range resourcePacks {
			if pack.Encrypted() {
				continue
			}
			utils.Encrypt(pack, s.EncryptionKey)
		}
	}

	lCfg := minecraft.ListenConfig{
		StatusProvider:         statusProvider,
		AuthenticationDisabled: !s.Authentication,
		ResourcePacks:          resourcePacks,
		TexturePacksRequired:   s.RequirePacks,
		AcceptedProtocols:      s.Protocols,
		FlushRate:              -1,

		AllowInvalidPackets: false,
		AllowUnknownPackets: true,

		ReadBatches: true,
	}

	if os.Getenv("PACKET_FUNC") != "" {
		lCfg.PacketFunc = func(header packet.Header, payload []byte, src, dst net.Addr) {
			fmt.Printf("%s -> %s: %d\n", src.String(), dst.String(), header.PacketID)
		}
	}

	l, err := lCfg.Listen("raknet", s.LocalAddress)

	if err != nil {
		o.Log.Errorf("unable to start oomph: %v", err)
		return
	}
	o.Listener = l
	defer l.Close()
	o.Log.Printf("Oomph is now listening on %v and directing connections to %v!\n", s.LocalAddress, s.RemoteAddress)
	for {
		c, err := l.Accept()
		if err != nil {
			panic(err)
		}

		go o.handleConn(c.(*minecraft.Conn), l, s.RemoteAddress)
	}
}

// handleConn handles initates a connection between the client and the server, and handles packets from both.
func (o *Oomph) handleConn(conn *minecraft.Conn, listener *minecraft.Listener, remoteAddr string) {
	sentryHub := sentry.CurrentHub().Clone()
	sentryHub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("func", "oomph.handleConn()")
	})

	p := player.New(o.Log, player.MonitoringState{
		IsReplay:    false,
		IsRecording: false,
		CurrentTime: time.Now(),
	}, listener)

	p.SetConn(conn)

	handler.RegisterHandlers(p)
	detection.RegisterDetections(p)
	component.RegisterAll(p)

	defer p.Close()
	defer func() {
		if err := recover(); err != nil {
			o.Log.Errorf("oomph.handleConn() panic: %v", err)
			sentryHub.Recover(oerror.New(fmt.Sprintf("%v", err)))
			sentryHub.Flush(time.Second * 5)
		}
	}()

	clientDat := conn.ClientData()
	clientDat.ServerAddress = remoteAddr

	serverConn, err := minecraft.Dialer{
		IdentityData: conn.IdentityData(),
		ClientData:   clientDat,
		FlushRate:    -1,

		DisconnectOnUnknownPackets: false,
		DisconnectOnInvalidPackets: true,
		IPAddress:                  conn.RemoteAddr().String(),

		ReadBatches: true,

		DownloadResourcePack: func(id uuid.UUID, version string, current, total int) bool {
			return false
		},
	}.Dial("raknet", remoteAddr)

	if err != nil {
		msg := unwrapNetError(err)
		if msg == "context deadline exceeded" {
			msg = "Proxy unable to reach server (no response)"
		}

		conn.WritePacket(&packet.Disconnect{
			Message: msg,
		})
		conn.Close()

		o.Log.Errorf("unable to reach server: %v", err)
		return
	}
	p.SetServerConn(serverConn)

	data := serverConn.GameData()
	data.PlayerMovementSettings.MovementType = protocol.PlayerMovementModeServerWithRewind
	data.PlayerMovementSettings.RewindHistorySize = 100
	data.GameRules = append(data.GameRules, protocol.GameRule{
		Name:                  "doimmediaterespawn",
		CanBeModifiedByPlayer: false,
		Value:                 true,
	})

	var g sync.WaitGroup
	g.Add(2)

	success := true
	go func() {
		if err := conn.StartGame(data); err != nil {
			conn.WritePacket(&packet.Disconnect{
				Message: "startGame(): " + unwrapNetError(err),
			})
			success = false
		}

		g.Done()
	}()
	go func() {
		if err := serverConn.DoSpawn(); err != nil {
			conn.WritePacket(&packet.Disconnect{
				Message: "doSpawn(): " + unwrapNetError(err),
			})
			success = false
		}

		g.Done()
	}()
	g.Wait()

	if !success {
		conn.Close()
		serverConn.Close()
		return
	}

	select {
	case o.players <- p:
		break
	case <-time.After(time.Second * 3):
		conn.WritePacket(&packet.Disconnect{
			Message: "Oomph timed out: please try re-logging.",
		})
		p.Close()

		hub := sentry.CurrentHub().Clone()
		hub.CaptureMessage("Oomph timed out accepting player into channel")
		hub.Flush(time.Second * 5)
		return
	}

	g.Add(2)
	go func() {
		localHub := sentry.CurrentHub().Clone()
		localHub.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetTag("conn_type", "clientConn")
			scope.SetTag("player", p.IdentityDat.DisplayName)
		})

		defer func() {
			if err := recover(); err != nil {
				o.Log.Errorf("handleConn() panic: %v", err)
				localHub.Recover(oerror.New(fmt.Sprintf("%v", err)))
				localHub.Flush(time.Second * 5)

				listener.Disconnect(conn, text.Colourf("<red><bold>Internal proxy error (report to admin):</red></bold>\n%v", err))
				return
			}

			if rsn := p.ServerConn().DisconnectReason(); rsn != "" {
				listener.Disconnect(conn, rsn)
			} else {
				listener.Disconnect(conn, "Unexpected disconnect (you shouldn't be able to see this).")
			}
			p.ServerConn().Close()
		}()
		defer g.Done()

		p.ClientPkFunc = func(pks []packet.Packet) error {
			p.ProcessMu.Lock()
			defer p.ProcessMu.Unlock()

			if err := p.DefaultHandleFromClient(pks); err != nil {
				return err
			}
			return p.ServerConn().Flush()
		}

		for {
			if p.Closed {
				return
			}

			pks, err := conn.ReadBatch()
			if err != nil {
				return
			}

			if err := p.ClientPkFunc(pks); err != nil {
				o.Log.Errorf("error handling packet from client: %v", err)
				return
			}
		}
	}()
	go func() {
		localHub := sentry.CurrentHub().Clone()
		localHub.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetTag("conn_type", "serverConn")
			scope.SetTag("player", p.IdentityDat.DisplayName)
		})

		defer func() {
			if err := recover(); err != nil {
				o.Log.Errorf("handleConn() panic: %v", err)
				localHub.Recover(err)
				localHub.Flush(time.Second * 5)

				listener.Disconnect(conn, text.Colourf("<red><bold>Internal proxy error (report to admin):</red></bold>\n%v", err))
				return
			}

			if rsn := p.ServerConn().DisconnectReason(); rsn != "" {
				listener.Disconnect(conn, rsn)
			} else {
				listener.Disconnect(conn, "Remote server disconnected unexpectedly from proxy.")
			}
			p.ServerConn().Close()
		}()
		defer g.Done()

		p.ServerPkFunc = func(pks []packet.Packet) error {
			p.ProcessMu.Lock()
			defer p.ProcessMu.Unlock()

			if err := p.DefaultHandleFromServer(pks); err != nil {
				return err
			}
			p.ACKs().Flush()
			return p.Conn().Flush()
		}

		for {
			if p.Closed {
				return
			}

			pks, err := p.ServerConn().ReadBatch()
			if err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					conn.WritePacket(&packet.Disconnect{
						Message: disconnect.Error(),
					})
					listener.Disconnect(conn, disconnect.Error())
				}

				return
			}

			if err := p.ServerPkFunc(pks); err != nil {
				o.Log.Errorf("error handling packet from server: %v", err)
				return
			}
		}
	}()

	g.Wait()
}

func unwrapNetError(err error) string {
	if netErr, ok := err.(*net.OpError); ok {
		if nErr, ok := netErr.Err.(*net.OpError); ok {
			return unwrapNetError(nErr)
		}

		return netErr.Err.Error()
	}

	return err.Error()
}
