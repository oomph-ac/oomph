package oomph

import (
	"errors"
	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/session"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/justtaldevelops/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"
)

// listener is a Dragonfly listener implementation for direct Oomph.
type listener struct {
	*minecraft.Listener
	log *logrus.Logger
	o   *Oomph
}

// Listen listens for oomph connections, this should be used instead of Start for dragonfly servers.
func (o *Oomph) Listen(s *server.Server, log *logrus.Logger, mainAddr, oomphAddr string) error {
	p, err := minecraft.NewForeignStatusProvider(mainAddr)
	if err != nil {
		panic(err)
	}
	l, err := minecraft.ListenConfig{
		StatusProvider: p,
	}.Listen("raknet", oomphAddr)
	if err != nil {
		return err
	}
	log.Infof("Oomph is now listening on %v and directing connections to %v!\n", oomphAddr, mainAddr)
	s.Listen(listener{
		Listener: l,
		log:      log,
		o:        o,
	})
	return nil
}

// Accept accepts an incoming player into the server. It blocks until a player connects to the server.
// Accept returns an error if the Server is no longer available.
func (o *Oomph) Accept() (*player.Player, error) {
	p, ok := <-o.players
	if !ok {
		return nil, errors.New("oomph shutdown")
	}
	return p, nil
}

// Accept blocks until the next connection is established and returns it. An error is returned if the Listener was
// closed using Close.
func (l listener) Accept() (session.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	p := player.NewPlayer(l.log, world.Overworld, 8, c.(*minecraft.Conn), nil)
	l.o.players <- p
	return p, err
}

// Disconnect disconnects a connection from the Listener with a reason.
func (l listener) Disconnect(conn session.Conn, reason string) error {
	return l.Listener.Disconnect(conn.(*minecraft.Conn), reason)
}

// Close closes the Listener.
func (l listener) Close() error {
	_ = l.Listener.Close()
	close(l.o.players)
	return nil
}
