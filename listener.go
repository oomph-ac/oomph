package oomph

import (
	"errors"
	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/session"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft"
)

// listener is a Dragonfly listener implementation for direct Oomph.
type listener struct {
	*minecraft.Listener
	o *Oomph
}

// Listen listens for oomph connections, this should be used instead of Start for dragonfly servers.
func (o *Oomph) Listen(srv *server.Server, name string, requirePacks bool) error {
	l, err := minecraft.ListenConfig{
		StatusProvider:       minecraft.NewStatusProvider(name),
		ResourcePacks:        srv.Resources(),
		TexturePacksRequired: requirePacks,
	}.Listen("raknet", o.addr)
	if err != nil {
		return err
	}
	o.log.Infof("Oomph is now listening on %v.\n", o.addr)
	srv.Listen(listener{
		Listener: l,
		o:        o,
	})
	return nil
}

// Accept accepts an incoming player into the server. It blocks until a player connects to the server.
// Accept returns an error if the Server is no longer available.
func (o *Oomph) Accept() (*player.Player, error) {
	p, ok := <-o.players
	if !ok {
		return nil, errors.New("could not accept player: oomph stopped")
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
	p := player.NewPlayer(l.o.log, c.(*minecraft.Conn), nil)
	l.o.players <- p
	return p, err
}

// Disconnect disconnects a connection from the Listener with a reason.
func (l listener) Disconnect(conn session.Conn, reason string) error {
	return l.Listener.Disconnect(conn.(*player.Player).Conn(), reason)
}

// Close closes the Listener.
func (l listener) Close() error {
	_ = l.Listener.Close()
	close(l.o.players)
	return nil
}
