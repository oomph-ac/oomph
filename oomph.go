package oomph

import (
	"encoding/json"
	"errors"
	"runtime"
	"sync"
	"time"

	"github.com/oomph-ac/oomph/utils"

	"github.com/go-gl/mathgl/mgl32"

	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
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
	clientDat := conn.ClientData()
	clientDat.ServerAddress = remoteAddr

	serverConn, err := minecraft.Dialer{
		IdentityData: conn.IdentityData(),
		ClientData:   clientDat,
		FlushRate:    -1,

		DisconnectOnUnknownPackets: false,
		DisconnectOnInvalidPackets: false,
	}.DialTimeout("raknet", remoteAddr, time.Second*10)

	if err != nil {
		conn.WritePacket(&packet.Disconnect{
			Message: err.Error(),
		})
		conn.Close()
		o.log.Error("unable to reach server: " + err.Error())
		return
	}

	data := serverConn.GameData()
	data.PlayerMovementSettings.MovementType = protocol.PlayerMovementModeServerWithRewind
	data.PlayerMovementSettings.RewindHistorySize = 100

	p := player.NewPlayer(logrus.New(), conn, serverConn)
	p.MovementInfo().ServerPosition = data.PlayerPosition.Sub(mgl32.Vec3{0, 1.62})
	p.MovementInfo().OnGround = true

	var g sync.WaitGroup
	g.Add(2)
	go func() {
		if err := p.Conn().StartGameTimeout(data, time.Second*15); err != nil {
			o.log.Error("oomph conn.StartGame(): " + err.Error())
			conn.WritePacket(&packet.Disconnect{
				Message: err.Error(),
			})
			conn.Flush()
			p.Close()
			p = nil
			return
		}
		enc, _ := json.Marshal(map[string]string{
			"xuid":     p.IdentityData().XUID,
			"uuid":     p.IdentityData().Identity,
			"ip":       p.Conn().RemoteAddr().String(),
			"username": p.IdentityData().DisplayName,
		})
		p.ServerConn().WritePacket(&packet.ScriptMessage{
			Identifier: "oomph:authentication",
			Data:       enc,
		})
		g.Done()
	}()
	go func() {
		if err := p.ServerConn().DoSpawnTimeout(time.Second * 15); err != nil {
			o.log.Error("oomph serverConn.DoSpawn(): " + err.Error())
			conn.WritePacket(&packet.Disconnect{
				Message: err.Error(),
			})
			conn.Flush()
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
			if !handleConn(p, listener) {
				return
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
			if !handleServerConn(p, listener) {
				return
			}

			//p.SendAck()
			//p.Conn().Flush()
		}
	}()
	g.Wait()
	p.Close()

	p = nil
	conn = nil
	serverConn = nil
	runtime.GC()
}

func handleConn(p *player.Player, listener *minecraft.Listener) bool {
	if p == nil {
		return false
	}

	pk, err := p.Conn().ReadPacket()
	if err != nil {
		p.Log().Error(err)
		return false
	}

	p.StartHandlePacket()
	defer p.EndHandlePacket()

	/* if p.UsesPacketBuffer() {
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
	} */

	if p.ClientProcess(pk) {
		return true
	}

	err = p.ServerConn().WritePacket(pk)
	if err != nil {
		p.Log().Error("serverConn.WritePacket() error: " + err.Error())
		if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
			listener.Disconnect(p.Conn(), disconnect.Error())
		}

		return false
	}

	return true
}

func handleServerConn(p *player.Player, listener *minecraft.Listener) bool {
	if p == nil {
		return false
	}

	pk, err := p.ServerConn().ReadPacket()

	if err != nil {
		p.Log().Error("serverConn.ReadPacket() error: " + err.Error())
		if disconnect, ok := errors.Unwrap(err).(minecraft.DisconnectError); ok {
			_ = listener.Disconnect(p.Conn(), disconnect.Error())
		}

		return false
	}

	if d, ok := pk.(*packet.Disconnect); ok {
		p.Conn().WritePacket(d)
		p.Conn().Flush()
		p.Close()

		return false
	}

	p.StartHandlePacket()
	defer p.EndHandlePacket()

	if p.ServerProcess(pk) {
		return true
	}

	if err := p.Conn().WritePacket(pk); err != nil {
		p.Log().Error("conn.WritePacket() error: " + err.Error())
		return false
	}

	return true
}
