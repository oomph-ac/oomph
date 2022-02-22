package oomph

import (
	"fmt"
	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/session"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/justtaldevelops/oomph/player"
	"github.com/justtaldevelops/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sirupsen/logrus"
)

type listener struct {
	*minecraft.Listener
	o *Oomph
}

// Listen listens for oomph connections, this should be used instead of Start for dragonfly servers.
func (o *Oomph) Listen(s *server.Server, remoteAddr, localAddr string) error {
	p, err := minecraft.NewForeignStatusProvider(remoteAddr)
	if err != nil {
		panic(err)
	}
	l, err := minecraft.ListenConfig{
		StatusProvider: p,
		ResourcePacks:  utils.GetResourcePacks(),
	}.Listen("raknet", localAddr)

	if err != nil {
		return err
	}
	fmt.Printf("Oomph is now listening on %v and directing connections to %v!\n", localAddr, remoteAddr)
	s.Listen(listener{
		Listener: l,
		o:        o,
	})

	return nil
}

// Accept blocks until the next connection is established and returns it. An error is returned if the Listener was
// closed using Close.
func (l listener) Accept() (session.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	lg := logrus.New()
	lg.Formatter = &logrus.TextFormatter{ForceColors: true}
	lg.Level = logrus.DebugLevel

	p := player.NewPlayer(lg, world.Overworld, 8, c.(*minecraft.Conn), nil)
	l.o.playerChan <- p
	return p, err
}

// Disconnect disconnects a connection from the Listener with a reason.
func (l listener) Disconnect(conn session.Conn, reason string) error {
	l.o.playerMutex.Lock()
	if l.o.closer != nil {
		l.o.closer.Close(conn.IdentityData())
	}
	_ = conn.WritePacket(&packet.Disconnect{
		HideDisconnectionScreen: reason == "",
		Message:                 reason,
	})
	return conn.Close()
}
