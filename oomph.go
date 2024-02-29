package oomph

import (
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/oomph-ac/oomph/detection"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/simulation"
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
	err := sentry.Init(sentry.ClientOptions{
		Dsn:           "https://06f2165840f341138a676b52eacad19c@o1409396.ingest.sentry.io/6747367",
		EnableTracing: os.Getenv("SENTRY_TRACE") == "true",
		TracesSampler: sentry.TracesSampler(func(ctx sentry.SamplingContext) float64 {
			if ctx.Span.Name != "oomph:handle_client" && ctx.Span.Name != "oomph:handle_server" && ctx.Span.Name != "oomph:tick" {
				return 0.0
			}

			delta := time.Since(ctx.Span.StartTime).Milliseconds()
			if delta >= 10 {
				return 1.0
			}

			return 0.0
		}),
	})

	if err != nil {
		panic("failed to init sentry: " + err.Error())
	}

	deadlock.Opts.Disable = true
	if os.Getenv("DEADLOCK_DEBUG") == "true" {
		deadlock.Opts.Disable = false
		deadlock.Opts.DeadlockTimeout = time.Second * 5
		deadlock.Opts.DisableLockOrderDetection = true
	}
}

type Oomph struct {
	log     *logrus.Logger
	players chan *player.Player

	settings OomphSettings
}

type OomphSettings struct {
	LocalAddress   string
	RemoteAddress  string
	Authentication bool

	ReadBatchMode     bool
	LatencyReportType handler.LatencyReportType

	StatusProvider *minecraft.ServerStatusProvider

	ResourcePath             string
	RequirePacks             bool
	FetchRemoteResourcePacks bool

	Protocols []minecraft.Protocol
}

// New creates and returns a new Oomph instance.
func New(log *logrus.Logger, s OomphSettings) *Oomph {
	return &Oomph{
		log:     log,
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

	var statusProvider minecraft.ServerStatusProvider
	if s.StatusProvider == nil {
		p, err := minecraft.NewForeignStatusProvider(s.RemoteAddress)
		if err != nil {
			o.log.Errorf("unable to make status provider: %v", err)
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
			o.log.Errorf("unable to fetch resource packs: %v", err)
			resourcePacks = utils.ResourcePacks(s.ResourcePath) // default to resource pack folder if fetching packs from the remote server fails
		} else {
			resourcePacks = resourcePackFetch.ResourcePacks()
			resourcePackFetch.Close()
		}
	} else {
		resourcePacks = utils.ResourcePacks(s.ResourcePath)
	}

	l, err := minecraft.ListenConfig{
		StatusProvider:         statusProvider,
		AuthenticationDisabled: !s.Authentication,
		ResourcePacks:          resourcePacks,
		TexturePacksRequired:   s.RequirePacks,
		AcceptedProtocols:      s.Protocols,
		FlushRate:              -1,

		AllowInvalidPackets: false,
		AllowUnknownPackets: true,

		ReadBatches: s.ReadBatchMode,
	}.Listen("raknet", s.LocalAddress)

	if err != nil {
		o.log.Errorf("unable to start oomph: %v", err)
		return
	}

	defer l.Close()
	o.log.Printf("Oomph is now listening on %v and directing connections to %v!\n", s.LocalAddress, s.RemoteAddress)
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

	defer func() {
		if err := recover(); err != nil {
			o.log.Errorf("oomph.handleConn() panic: %v", err)
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

		ReadBatches: o.settings.ReadBatchMode,
	}.Dial("raknet", remoteAddr)

	if err != nil {
		conn.WritePacket(&packet.Disconnect{
			Message: unwrapNetError(err),
		})
		conn.Close()

		o.log.Errorf("unable to reach server: %v", err)
		return
	}

	data := serverConn.GameData()
	data.PlayerMovementSettings.MovementType = protocol.PlayerMovementModeServerWithRewind
	data.PlayerMovementSettings.RewindHistorySize = 100

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

	p := player.New(o.log, o.settings.ReadBatchMode, conn, serverConn)
	handler.RegisterHandlers(p)
	detection.RegisterDetections(p)

	p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler).Simulate(&simulation.MovementSimulator{})
	p.Handler(handler.HandlerIDLatency).(*handler.LatencyHandler).ReportType = o.settings.LatencyReportType

	p.ClientPkFunc = func(pks []packet.Packet) error {
		p.ProcessMu.Lock()
		defer p.ProcessMu.Unlock()

		if err := p.DefaultHandleFromClient(pks); err != nil {
			return err
		}

		if o.settings.ReadBatchMode {
			return serverConn.Flush()
		}
		return nil
	}

	p.ServerPkFunc = func(pks []packet.Packet) error {
		p.ProcessMu.Lock()
		defer p.ProcessMu.Unlock()

		if err := p.DefaultHandleFromServer(pks); err != nil {
			return err
		}

		// UGLY HACK: This is to prevent import cycles w/ Player->Handler and Handler->Player.
		// TODO: Refactor this to be more elegant?
		if o.settings.ReadBatchMode {
			p.Handler(handler.HandlerIDAcknowledgements).(*handler.AcknowledgementHandler).Flush(p)
			return conn.Flush()
		}

		return nil
	}

	select {
	case o.players <- p:
		break
	case <-time.After(time.Second * 3):
		conn.WritePacket(&packet.Disconnect{
			Message: "oomph timed out",
		})
		conn.Close()
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
			scope.SetTag("player", p.IdentityData().DisplayName)
		})

		defer func() {
			if err := recover(); err != nil {
				o.log.Errorf("handleConn() panic: %v", err)
				localHub.Recover(oerror.New(fmt.Sprintf("%v", err)))
				localHub.Flush(time.Second * 5)

				listener.Disconnect(conn, text.Colourf("<red><bold>Internal proxy error (report to admin):</red></bold>\n%v", err))
				return
			}

			listener.Disconnect(conn, "Report to admin: unknown cause for disconnect.")
			serverConn.Close()
		}()
		defer g.Done()

		for {
			if p.Closed {
				return
			}

			var pks []packet.Packet
			var err error

			if o.settings.ReadBatchMode {
				pks, err = conn.ReadBatch()
			} else if pk, err2 := conn.ReadPacket(); err2 == nil {
				pks = []packet.Packet{pk}
			} else {
				err = err2
			}

			if err != nil && !p.Closed {
				o.log.Errorf("error reading packets from client: %v", err)
				return
			}

			if err := p.ClientPkFunc(pks); err != nil {
				o.log.Errorf("error handling packets from client: %v", err)
				return
			}
		}
	}()
	go func() {
		localHub := sentry.CurrentHub().Clone()
		localHub.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetTag("conn_type", "serverConn")
			scope.SetTag("player", p.IdentityData().DisplayName)
		})

		defer func() {
			if err := recover(); err != nil {
				o.log.Errorf("handleConn() panic: %v", err)
				localHub.Recover(err)
				localHub.Flush(time.Second * 5)

				listener.Disconnect(conn, text.Colourf("<red><bold>Internal proxy error (report to admin):</red></bold>\n%v", err))
				return
			}

			listener.Disconnect(conn, "Remote server disconnected unexpectedly from proxy.")
			serverConn.Close()
		}()
		defer g.Done()

		for {
			if p.Closed {
				return
			}

			var pks []packet.Packet
			var err error

			if o.settings.ReadBatchMode {
				pks, err = serverConn.ReadBatch()
			} else if pk, err2 := serverConn.ReadPacket(); err2 == nil {
				pks = []packet.Packet{pk}
			} else {
				err = err2
			}

			if err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					conn.WritePacket(&packet.Disconnect{
						Message: disconnect.Error(),
					})
					listener.Disconnect(conn, disconnect.Error())
					return
				}

				o.log.Errorf("error reading packets from server: %v", err)
				return
			}

			if err := p.ServerPkFunc(pks); err != nil {
				o.log.Errorf("error handling packets from server: %v", err)
				return
			}
		}
	}()

	g.Wait()
	p.Close()
}

func unwrapNetError(err error) string {
	if netErr, ok := err.(*net.OpError); ok {
		return netErr.Err.Error()
	}

	return err.Error()
}
