package oomph

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/oomph-ac/oomph/detection"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

func init() {
	err := sentry.Init(sentry.ClientOptions{
		Dsn: "https://06f2165840f341138a676b52eacad19c@o1409396.ingest.sentry.io/6747367",
	})

	if err != nil {
		panic("failed to init sentry: " + err.Error())
	}
}

type Oomph struct {
	log     *logrus.Logger
	address string

	players chan *player.Player
}

// New creates and returns a new Oomph instance.
func New(log *logrus.Logger, address string) *Oomph {
	return &Oomph{
		log:     log,
		address: address,
		players: make(chan *player.Player),
	}
}

// Start will start Oomph! remoteAddr is the address of the target server, and localAddr is the address that players will connect to.
// Addresses should be formatted in the following format: "ip:port" (ex: "127.0.0.1:19132").
// If you're using dragonfly, use Listen instead of Start.
func (o *Oomph) Start(remoteAddr string, resourcePackPath string, protocols []minecraft.Protocol, requirePacks bool, authDisabled bool) error {
	p, err := minecraft.NewForeignStatusProvider(remoteAddr)
	if err != nil {
		return err
	}
	l, err := minecraft.ListenConfig{
		StatusProvider:         p,
		AuthenticationDisabled: authDisabled,
		ResourcePacks:          utils.ResourcePacks(resourcePackPath),
		TexturePacksRequired:   requirePacks,
		AcceptedProtocols:      protocols,
		FlushRate:              -1,

		AllowInvalidPackets: false,
		AllowUnknownPackets: true,
	}.Listen("raknet", o.address)

	if err != nil {
		return err
	}
	defer l.Close()
	o.log.Printf("Oomph is now listening on %v and directing connections to %v!\n", o.address, remoteAddr)
	for {
		c, err := l.Accept()
		if err != nil {
			return err
		}

		go o.handleConn(c.(*minecraft.Conn), l, remoteAddr)
	}
}

// Accept returns a player selected from the channel.
func (o *Oomph) Accept() (*player.Player, error) {
	p, ok := <-o.players
	if !ok {
		return nil, fmt.Errorf("unable to accept player: channel closed")
	}

	return p, nil
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
			sentryHub.Recover(oerror.NewOomphError(fmt.Sprintf("%v", err)))
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
	}.Dial("raknet", remoteAddr)

	if err != nil {
		conn.WritePacket(&packet.Disconnect{
			Message: err.Error(),
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
				Message: err.Error(),
			})
			success = false
		}

		g.Done()
	}()
	go func() {
		if err := serverConn.DoSpawn(); err != nil {
			conn.WritePacket(&packet.Disconnect{
				Message: err.Error(),
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

	p := player.New(o.log, conn, serverConn)
	handler.RegisterHandlers(p)
	detection.RegisterDetections(p)

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
		})

		defer func() {
			conn.Close()
			serverConn.Close()

			if err := recover(); err != nil {
				o.log.Errorf("handleConn() panic: %v", err)
				localHub.Recover(oerror.NewOomphError(fmt.Sprintf("%v", err)))
				localHub.Flush(time.Second * 5)

				listener.Disconnect(conn, "The proxy encountered an error.")
				return
			}

			listener.Disconnect(conn, "client connection lost: unknown")
		}()
		defer g.Done()

		for {
			pk, err := conn.ReadPacket()
			if err != nil {
				o.log.Errorf("error reading packet from client: %v", err)
				return
			}

			if err := p.HandleFromClient(pk); err != nil {
				o.log.Errorf("error handling packet from client: %v", err)
				return
			}
		}
	}()
	go func() {
		localHub := sentry.CurrentHub().Clone()
		localHub.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetTag("conn_type", "serverConn")
		})

		defer func() {
			conn.Close()
			serverConn.Close()

			if err := recover(); err != nil {
				o.log.Errorf("handleConn() panic: %v", err)
				localHub.Recover(err)
				localHub.Flush(time.Second * 5)

				listener.Disconnect(conn, "The proxy encountered an error.")
				return
			}

			listener.Disconnect(conn, "server connection lost: unknown")
		}()
		defer g.Done()

		for {
			pk, err := serverConn.ReadPacket()
			if err != nil {
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					conn.WritePacket(&packet.Disconnect{
						Message: disconnect.Error(),
					})
					listener.Disconnect(conn, disconnect.Error())
					return
				}

				o.log.Errorf("error reading packet from server: %v", err)
				return
			}

			if d, ok := pk.(*packet.Disconnect); ok {
				conn.WritePacket(d)
				conn.Flush()
				p.Close()

				return
			}

			if err := p.HandleFromServer(pk); err != nil {
				o.log.Errorf("error handling packet from server: %v", err)
				return
			}
		}
	}()

	g.Wait()
	p.Close()
}
