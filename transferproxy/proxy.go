// Package proxy provides a standalone Minecraft Bedrock RakNet proxy.
package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Proxy accepts Bedrock clients and keeps them connected while their backend
// connection is replaced during packet.Transfer handoffs.
type Proxy struct {
	cfg      Config
	listener *minecraft.Listener
	done     chan struct{}
	closeOne sync.Once

	batchReading bool
}

// Listen starts a standalone Bedrock proxy.
func Listen(ctx context.Context, cfg Config) (*Proxy, error) {
	if cfg.LocalAddress == "" || cfg.RemoteAddress == "" {
		return nil, fmt.Errorf("proxy: local and remote addresses are required")
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 10 * time.Second
	}
	if cfg.Dial == nil {
		cfg.Dial = defaultDial(cfg.DialTimeout, cfg.Listen.EnableBatchReading)
	}
	if cfg.Listen.Compression == nil {
		cfg.Listen.Compression = packet.SnappyCompression
	}
	l, err := cfg.Listen.Listen("raknet", cfg.LocalAddress)
	if err != nil {
		return nil, fmt.Errorf("proxy: listen: %w", err)
	}
	p := &Proxy{cfg: cfg, listener: l, done: make(chan struct{}), batchReading: cfg.Listen.EnableBatchReading}
	go func() {
		select {
		case <-ctx.Done():
			_ = p.Close()
		case <-p.done:
		}
	}()
	return p, nil
}

// Serve accepts clients until the proxy is closed.
func (p *Proxy) Serve(ctx context.Context) error {
	for {
		raw, err := p.listener.Accept()
		if err != nil {
			select {
			case <-p.done:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			default:
				return fmt.Errorf("proxy: accept: %w", err)
			}
		}
		conn := raw.(*minecraft.Conn)
		go func() {
			if err := p.serveClient(ctx, conn); err != nil {
				p.cfg.Log.Error("proxy session closed", "player", conn.IdentityData().DisplayName, "err", err)
			}
		}()
	}
}

func (p *Proxy) serveClient(ctx context.Context, conn *minecraft.Conn) error {
	clientData := conn.ClientData()
	clientData.ThirdPartyName = conn.IdentityData().DisplayName
	backend, err := p.cfg.Dial(ctx, p.cfg.RemoteAddress, conn.IdentityData(), clientData, conn.RemoteAddr().String())
	if err != nil {
		reason := "Unable to connect to the backend server."
		if backendReason, ok := backendDisconnectMessage(err); ok {
			reason = backendReason
		}
		_ = p.listener.Disconnect(conn, reason)
		return err
	}
	handler := Handler(NopHandler{})
	if p.cfg.NewHandler != nil {
		handler = p.cfg.NewHandler(HandlerContext{
			Client: conn, Listener: p.listener,
			Log: p.cfg.Log.With("player", conn.IdentityData().DisplayName),
		})
		if handler == nil {
			handler = NopHandler{}
		}
	}
	s := newSession(p, handler, conn, backend, conn.IdentityData(), clientData, conn.RemoteAddr().String())
	defer s.close()
	return s.run(ctx)
}

// Close stops accepting new clients.
func (p *Proxy) Close() error {
	var err error
	p.closeOne.Do(func() {
		close(p.done)
		err = p.listener.Close()
	})
	return err
}
