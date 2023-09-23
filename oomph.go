package oomph

import (
	"errors"
	"sync"

	"github.com/oomph-ac/oomph/utils"

	"github.com/go-gl/mathgl/mgl32"

	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sirupsen/logrus"
)

// Oomph represents an instance of the Oomph proxy.
type Oomph struct {
	players chan *player.Player
	log     *logrus.Logger
	addr    string
}

// New returns a new Oomph instance.
// If your server is using Dragonfly, be sure to use the Listener function instead.
func New(log *logrus.Logger, localAddr string) *Oomph {
	return &Oomph{
		players: make(chan *player.Player),
		addr:    localAddr,
		log:     log,
	}
}

// Start will start Oomph! remoteAddr is the address of the target server, and localAddr is the address that players will connect to.
// Addresses should be formatted in the following format: "ip:port" (ex: "127.0.0.1:19132").
// If you're using dragonfly, use Listen instead of Start.
func (o *Oomph) Start(remoteAddr string, resourcePackPath string, protocols []minecraft.Protocol, requirePacks bool, authDisabled bool) error {
	p, err := minecraft.NewForeignStatusProvider(remoteAddr)
	if err != nil {
		panic(err)
	}
	l, err := minecraft.ListenConfig{
		StatusProvider:         p,
		AuthenticationDisabled: authDisabled,
		ResourcePacks:          utils.ResourcePacks(resourcePackPath),
		TexturePacksRequired:   requirePacks,
		AcceptedProtocols:      protocols,
		FlushRate:              -1,
		ReadBatches:            true,
		AllowInvalidPackets:    true,
		AllowUnknownPackets:    true,
	}.Listen("raknet", o.addr)
	if err != nil {
		return err
	}
	defer l.Close()
	o.log.Printf("Oomph is now listening on %v and directing connections to %v!\n", o.addr, remoteAddr)
	for {
		c, err := l.Accept()
		if err != nil {
			panic(err)
		}

		go o.handleConn(c.(*minecraft.Conn), l, remoteAddr)
	}
}

// handleConn handles a new incoming minecraft.Conn from the minecraft.Listener passed.
func (o *Oomph) handleConn(conn *minecraft.Conn, listener *minecraft.Listener, remoteAddr string) {
	serverConn, err := minecraft.Dialer{
		IdentityData: conn.IdentityData(),
		ClientData:   conn.ClientData(),
		FlushRate:    -1,
		ReadBatches:  true,
	}.Dial("raknet", remoteAddr)

	if err != nil {
		o.log.Error(err)
		return
	}

	data := serverConn.GameData()
	data.PlayerMovementSettings.MovementType = protocol.PlayerMovementModeServerWithRewind
	data.PlayerMovementSettings.RewindHistorySize = 40

	p := player.NewPlayer(logrus.New(), conn, serverConn)
	p.MovementInfo().ServerPosition = data.PlayerPosition.Sub(mgl32.Vec3{0, 1.62})
	p.MovementInfo().OnGround = true

	var g sync.WaitGroup
	g.Add(2)
	go func() {
		if err := p.Conn().StartGame(data); err != nil {
			o.log.Error("oomph conn.StartGame(): " + err.Error())
			p.Close()
			p = nil
			return
		}
		g.Done()
	}()
	go func() {
		if err := p.ServerConn().DoSpawn(); err != nil {
			o.log.Error("oomph serverConn.DoSpawn(): " + err.Error())
			p.Close()
			p = nil
			return
		}
		g.Done()
	}()
	g.Wait()

	go func() {
		o.players <- p
	}()

	g.Add(2)
	go func() {
		defer func() {
			_ = listener.Disconnect(p.Conn(), "client connection lost")
			_ = p.ServerConn().Close()
			g.Done()
		}()
		for {
			pks, err := p.Conn().ReadBatch()
			if err != nil || p == nil {
				o.log.Error(err)
				return
			}

			if len(pks) == 0 {
				return
			}

			for _, pk := range pks {
				if p.UsesPacketBuffer() {
					if !p.QueuePacket(pk) {
						continue
					}

					err = p.SendPacketToServer(pk)
					if err == nil {
						continue
					}

					if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
						_ = listener.Disconnect(p.Conn(), disconnect.Error())
					}
				}

				if p.ClientProcess(pk) {
					continue
				}

				err = p.ServerConn().WritePacket(pk)
				if err != nil {
					p.Log().Error("serverConn.WritePacket() error: " + err.Error())
					if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
						_ = listener.Disconnect(p.Conn(), disconnect.Error())
					}
					return
				}
			}

			//p.ServerConn().Flush()
		}
	}()
	go func() {
		defer func() {
			_ = p.ServerConn().Close()
			_ = listener.Disconnect(p.Conn(), "server connection lost")
			g.Done()
		}()
		for {
			pks, err := p.ServerConn().ReadBatch()

			if len(pks) == 0 {
				return
			}

			if err != nil {
				p.Log().Error("serverConn.ReadBatch() error: " + err.Error())
				if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
					_ = listener.Disconnect(p.Conn(), disconnect.Error())
				}
				return
			}

			for _, pk := range pks {
				if p.ServerProcess(pk) {
					continue
				}

				if err := p.Conn().WritePacket(pk); err != nil {
					p.Log().Error("conn.WritePacket() error: " + err.Error())
					return
				}
			}

			//p.SendAck()
			//p.Conn().Flush()
		}
	}()
	g.Wait()
	p.Close()
}
