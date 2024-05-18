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
	"github.com/oomph-ac/oomph/event"
	"github.com/oomph-ac/oomph/handler"
	_ "github.com/oomph-ac/oomph/handler/ackfunc"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/session"
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
	deadlock.Opts.Disable = true
	if os.Getenv("DEADLOCK_DEBUG") == "true" {
		deadlock.Opts.Disable = false
		deadlock.Opts.DeadlockTimeout = time.Second * 5
		deadlock.Opts.DisableLockOrderDetection = true
		deadlock.Opts.PrintAllCurrentGoroutines = true
	}
}

type Oomph struct {
	Log *logrus.Logger

	settings OomphSettings
	sessions chan *session.Session
}

type OomphSettings struct {
	Logger         *logrus.Logger
	StatusProvider *minecraft.ServerStatusProvider

	LocalAddress   string
	RemoteAddress  string
	Authentication bool

	LatencyReportType handler.LatencyReportType

	ResourcePath             string
	RequirePacks             bool
	FetchRemoteResourcePacks bool

	Protocols []minecraft.Protocol

	EnableSentry bool
	SentryOpts   *sentry.ClientOptions
}

// New creates and returns a new Oomph instance.
func New(s OomphSettings) *Oomph {
	return &Oomph{
		Log:      s.Logger,
		sessions: make(chan *session.Session),

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
				return 0.03
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

	l, err := minecraft.ListenConfig{
		StatusProvider:         statusProvider,
		AuthenticationDisabled: !s.Authentication,
		ResourcePacks:          resourcePacks,
		TexturePacksRequired:   s.RequirePacks,
		AcceptedProtocols:      s.Protocols,
		FlushRate:              -1,

		AllowInvalidPackets: false,
		AllowUnknownPackets: true,

		ReadBatches: true,
	}.Listen("raknet", s.LocalAddress)

	if err != nil {
		o.Log.Errorf("unable to start oomph: %v", err)
		return
	}

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

	s := session.New(o.Log, session.SessionState{
		IsReplay:    false,
		IsRecording: false,
		DirectMode:  false,

		CurrentTime: time.Now(),
	})

	p := s.Player
	p.SetConn(conn)

	handler.RegisterHandlers(p)
	detection.RegisterDetections(p)

	defer s.Close()
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

	p.SetServerConn(serverConn)

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

	p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler).Simulate(&simulation.MovementSimulator{})
	p.Handler(handler.HandlerIDLatency).(*handler.LatencyHandler).ReportType = o.settings.LatencyReportType

	select {
	case o.sessions <- s:
		break
	case <-time.After(time.Second * 3):
		conn.WritePacket(&packet.Disconnect{
			Message: "Oomph timed out: please try re-logging.",
		})
		s.Close()

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

			listener.Disconnect(conn, "Report to admin: unknown cause for disconnect.")
			serverConn.Close()
		}()
		defer g.Done()

		for {
			if p.Closed {
				return
			}

			pks, err := conn.ReadBatch()
			if err != nil {
				return
			}

			ev := event.PacketEvent{
				Packets: pks,
				Server:  false,
			}
			ev.EvTime = time.Now().UnixNano()

			if err := s.QueueEvent(ev); err != nil {
				o.Log.Errorf("error handling packets from client: %v", err)
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

			listener.Disconnect(conn, "Remote server disconnected unexpectedly from proxy.")
			serverConn.Close()
		}()
		defer g.Done()

		for {
			if p.Closed {
				return
			}

			pks, err := serverConn.ReadBatch()
			if err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					conn.WritePacket(&packet.Disconnect{
						Message: disconnect.Error(),
					})
					listener.Disconnect(conn, disconnect.Error())
				}

				return
			}

			ev := event.PacketEvent{
				Packets: pks,
				Server:  true,
			}
			ev.EvTime = time.Now().UnixNano()

			if err := s.QueueEvent(ev); err != nil {
				o.Log.Errorf("error handling packets from server: %v", err)
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
