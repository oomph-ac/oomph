// Package dragonfly integrates Oomph directly into a Dragonfly server without
// an intermediate proxy or downstream transport.
package dragonfly

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/session"
	"github.com/oomph-ac/oomph/anticheat/player"
	"github.com/oomph-ac/oomph/anticheat/player/component"
	"github.com/oomph-ac/oomph/anticheat/player/detection"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Config configures the native Oomph listener used by Dragonfly.
type Config struct {
	// Address is the local RakNet address to listen on, such as ":19132".
	Address string
	// AcceptedProtocols optionally restricts the Bedrock protocol versions that
	// may connect. An empty slice accepts gophertunnel's current protocol.
	AcceptedProtocols []minecraft.Protocol
	// Configure is called after Oomph's default components and detections have
	// been registered for a player.
	Configure func(*player.Player)
}

// Listener returns a Dragonfly listener factory backed by an Oomph player
// connection. The surrounding server.Config remains authoritative for status,
// authentication, resource packs, explicitly configured compression, and
// player limits. Compression defaults to Snappy when left unset.
func Listener(ctx context.Context, cfg Config) func(server.Config) (server.Listener, error) {
	return func(conf server.Config) (server.Listener, error) {
		if cfg.Address == "" {
			return nil, fmt.Errorf("dragonfly integration: listener address is required")
		}
		log := conf.Log
		if log == nil {
			log = slog.Default()
		}
		listenCfg := minecraft.ListenConfig{
			MaximumPlayers:         conf.MaxPlayers,
			StatusProvider:         conf.StatusProvider,
			AuthenticationDisabled: conf.AuthDisabled,
			ResourcePacks:          conf.Resources,
			TexturePacksRequired:   conf.ResourcesRequired,
			Compression:            listenerCompression(conf.Compression),
			AcceptedProtocols:      cfg.AcceptedProtocols,
			FlushRate:              -1,
		}
		if log.Enabled(ctx, slog.LevelDebug) {
			listenCfg.ErrorLog = log.With("net_origin", "gophertunnel")
		}
		raw, err := listenCfg.Listen("raknet", cfg.Address)
		if err != nil {
			return nil, fmt.Errorf("dragonfly integration: listen: %w", err)
		}
		log.Info("Dragonfly with Oomph listening", "addr", raw.Addr())
		return newListener(ctx, nil, raw, log, cfg.Configure), nil
	}
}

// Wrap adds Oomph packet processing to a Dragonfly listener factory backed by
// a gophertunnel RakNet or NetherNet listener.
func Wrap(ctx context.Context, next func(server.Config) (server.Listener, error), configure func(*player.Player)) func(server.Config) (server.Listener, error) {
	return func(conf server.Config) (server.Listener, error) {
		if next == nil {
			return nil, fmt.Errorf("dragonfly integration: wrapped listener factory is required")
		}
		base, err := next(conf)
		if err != nil {
			return nil, fmt.Errorf("dragonfly integration: create wrapped listener: %w", err)
		}
		if base == nil {
			return nil, fmt.Errorf("dragonfly integration: wrapped listener factory returned nil")
		}
		log := conf.Log
		if log == nil {
			log = slog.Default()
		}
		return newListener(ctx, base, nil, log, configure), nil
	}
}

func newListener(ctx context.Context, base server.Listener, raw *minecraft.Listener, log *slog.Logger, configure func(*player.Player)) *listener {
	l := &listener{base: base, raw: raw, log: log, configure: configure, done: make(chan struct{})}
	go func() {
		select {
		case <-ctx.Done():
			_ = l.Close()
		case <-l.done:
		}
	}()
	return l
}

func listenerCompression(compression packet.Compression) packet.Compression {
	if compression == nil {
		return packet.SnappyCompression
	}
	return compression
}

type listener struct {
	base      server.Listener
	raw       *minecraft.Listener
	log       *slog.Logger
	configure func(*player.Player)
	done      chan struct{}
	closeOnce sync.Once
	closeErr  error
}

func (l *listener) Accept() (session.Conn, error) {
	var accepted session.Conn
	if l.base != nil {
		conn, err := l.base.Accept()
		if err != nil {
			return nil, err
		}
		accepted = conn
	} else {
		raw, err := l.raw.Accept()
		if err != nil {
			return nil, err
		}
		conn, ok := raw.(*minecraft.Conn)
		if !ok {
			_ = raw.Close()
			return nil, fmt.Errorf("dragonfly integration: unexpected connection type %T", raw)
		}
		accepted = conn
	}
	conn, ok := accepted.(*minecraft.Conn)
	if !ok {
		_ = accepted.Close()
		return nil, fmt.Errorf("dragonfly integration: unexpected connection type %T", accepted)
	}
	p := player.New(l.log.With(
		"name", conn.IdentityData().DisplayName,
		"xuid", conn.IdentityData().XUID,
	), player.MonitoringState{
		CurrentTime: time.Now(),
	}, l.raw)
	p.SetConn(conn)
	component.Register(p)
	detection.Register(p)
	if l.configure != nil {
		l.configure(p)
	}
	return newSessionConn(conn, p), nil
}

func (l *listener) Disconnect(conn session.Conn, reason string) error {
	c, ok := conn.(*sessionConn)
	if !ok {
		return fmt.Errorf("dragonfly integration: unexpected session connection type %T", conn)
	}
	var disconnectErr error
	if l.base != nil {
		disconnectErr = l.base.Disconnect(c.Conn, reason)
	} else if raw, ok := c.Conn.(*minecraft.Conn); ok && l.raw != nil {
		disconnectErr = l.raw.Disconnect(raw, reason)
	}
	return errors.Join(disconnectErr, c.Close())
}

func (l *listener) Close() error {
	l.closeOnce.Do(func() {
		close(l.done)
		if l.base != nil {
			l.closeErr = l.base.Close()
		} else if l.raw != nil {
			l.closeErr = l.raw.Close()
		}
	})
	return l.closeErr
}

var _ server.Listener = (*listener)(nil)
