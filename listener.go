package oomph

import (
	"errors"

	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/session"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sirupsen/logrus"
)

// listener is a Dragonfly listener implementation for direct Oomph.
type listener struct {
	*minecraft.Listener
	o *Oomph
}

// Listen adds the oomph listener in the server config, this should be used instead of Start() for dragonfly servers.
func (o *Oomph) Listen(conf *server.Config, name string, protocols []minecraft.Protocol, requirePacks bool) {
	conf.Listeners = nil
	conf.Listeners = append(conf.Listeners, func(_ server.Config) (server.Listener, error) {
		l, err := minecraft.ListenConfig{
			StatusProvider:       minecraft.NewStatusProvider(name),
			ResourcePacks:        conf.Resources,
			TexturePacksRequired: requirePacks,
			AcceptedProtocols:    protocols,
			FlushRate:            -1,
		}.Listen("raknet", o.addr)
		if err != nil {
			return nil, err
		}

		conf.Log.Infof("Dragonfly + Oomph is listening on %v", l.Addr())

		return listener{
			Listener: l,
			o:        o,
		}, nil
	})
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
	p := player.NewPlayer(logrus.New(), c.(*minecraft.Conn), nil)
	p.SetRuntimeID(1)
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
