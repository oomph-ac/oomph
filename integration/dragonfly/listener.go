// Package dragonfly integrates Oomph directly into a Dragonfly server without
// an intermediate Spectrum proxy or downstream transport.
package dragonfly

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/session"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component"
	"github.com/oomph-ac/oomph/player/detection"
	"github.com/sandertv/gophertunnel/minecraft"
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
// authentication, resource packs, compression, and player limits.
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
			Compression:            conf.Compression,
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
		return &listener{raw: raw, log: log, configure: cfg.Configure}, nil
	}
}

type listener struct {
	raw       *minecraft.Listener
	log       *slog.Logger
	configure func(*player.Player)
}

func (l *listener) Accept() (session.Conn, error) {
	raw, err := l.raw.Accept()
	if err != nil {
		return nil, err
	}
	conn, ok := raw.(*minecraft.Conn)
	if !ok {
		_ = raw.Close()
		return nil, fmt.Errorf("dragonfly integration: unexpected connection type %T", raw)
	}
	p := player.New(l.log.With(
		"name", conn.IdentityData().DisplayName,
		"xuid", conn.IdentityData().XUID,
	), player.MonitoringState{
		CurrentTime: time.Now(),
	}, l.raw)
	p.SetConn(conn)
	p.RuntimeId = 1
	p.EnableDirectMode()
	component.Register(p)
	detection.Register(p)
	if l.configure != nil {
		l.configure(p)
	}
	return p, nil
}

func (l *listener) Disconnect(conn session.Conn, reason string) error {
	p, ok := conn.(*player.Player)
	if !ok {
		return fmt.Errorf("dragonfly integration: unexpected session connection type %T", conn)
	}
	var disconnectErr error
	if p.Conn() != nil && l.raw != nil {
		disconnectErr = l.raw.Disconnect(p.Conn(), reason)
	}
	return errors.Join(disconnectErr, p.Close())
}

func (l *listener) Close() error {
	return l.raw.Close()
}

var _ server.Listener = (*listener)(nil)
var _ session.Conn = (*player.Player)(nil)
