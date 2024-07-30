package oomph

import (
	"errors"
	"time"

	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/session"
	"github.com/oomph-ac/oomph/detection"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/simulation"
	"github.com/sandertv/gophertunnel/minecraft"

	osession "github.com/oomph-ac/oomph/session"
)

// listener is a Dragonfly listener implementation for direct Oomph.
type listener struct {
	*minecraft.Listener
	o *Oomph
}

// Listen adds the oomph listener in the server config, this should be used instead of Start() for dragonfly servers.
func (o *Oomph) Listen(conf *server.Config, name string, protocols []minecraft.Protocol, requirePacks bool, authDisabled bool) {
	conf.Listeners = nil
	conf.Listeners = append(conf.Listeners, func(_ server.Config) (server.Listener, error) {
		l, err := minecraft.ListenConfig{
			StatusProvider:         minecraft.NewStatusProvider(name, "oomph"),
			AuthenticationDisabled: authDisabled,
			ResourcePacks:          conf.Resources,
			TexturePacksRequired:   requirePacks,
			AcceptedProtocols:      protocols,
			FlushRate:              -1,
		}.Listen("raknet", o.settings.LocalAddress)
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
func (o *Oomph) Accept() (*osession.Session, error) {
	s, ok := <-o.sessions
	if !ok {
		return nil, errors.New("could not accept player: oomph stopped")
	}
	return s, nil
}

// Accept blocks until the next connection is established and returns it. An error is returned if the Listener was
// closed using Close.
func (l listener) Accept() (session.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	s := osession.New(l.o.Log, osession.SessionState{
		IsReplay:    false,
		IsRecording: false,
		DirectMode:  true,
		CurrentTime: time.Now(),
	}, l.Listener)

	p := s.Player
	p.SetConn(c.(*minecraft.Conn))
	p.RuntimeId = 1
	p.ClientRuntimeId = 1

	handler.RegisterHandlers(p)
	detection.RegisterDetections(p)

	p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler).Simulate(&simulation.MovementSimulator{})

	l.o.sessions <- s
	return s, err
}

// Disconnect disconnects a connection from the Listener with a reason.
func (l listener) Disconnect(conn session.Conn, reason string) error {
	return l.Listener.Disconnect(conn.(*osession.Session).Conn(), reason)
}

// Close closes the Listener.
func (l listener) Close() error {
	_ = l.Listener.Close()
	close(l.o.sessions)
	return nil
}
