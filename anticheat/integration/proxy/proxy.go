// Package proxy integrates Oomph with the standalone Bedrock proxy module.
package proxy

import (
	"context"
	"log/slog"
	"time"

	"github.com/oomph-ac/oomph/anticheat/player"
	"github.com/oomph-ac/oomph/anticheat/player/component"
	playercontext "github.com/oomph-ac/oomph/anticheat/player/context"
	"github.com/oomph-ac/oomph/anticheat/player/detection"
	proxycore "github.com/oomph-ac/oomph/transferproxy"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Proxy is a running standalone proxy.
type Proxy = proxycore.Proxy

// Backend is the connection surface required from a backend server.
type Backend = proxycore.Backend

// DialFunc establishes and logs into a backend, stopping before DoSpawn.
type DialFunc = proxycore.DialFunc

// PacketContext carries a packet through handler processing.
type PacketContext = proxycore.PacketContext

// Config configures a standalone Oomph proxy.
type Config struct {
	LocalAddress  string
	RemoteAddress string
	Log           *slog.Logger
	Listen        minecraft.ListenConfig
	DialTimeout   time.Duration
	Configure     func(*player.Player)

	// Dial may be set to customise backend connections. Backend connections must be dialed with
	// batch reading enabled (minecraft.Dialer.EnableBatchReading), as the Oomph proxy always
	// reads packets in batches.
	Dial DialFunc
}

// Listen starts a native Oomph proxy.
func Listen(ctx context.Context, cfg Config) (*Proxy, error) {
	// Oomph always processes packets in batches so that each read batch is handled under a
	// single processing lock acquisition.
	cfg.Listen.EnableBatchReading = true
	return proxycore.Listen(ctx, proxycore.Config{
		LocalAddress: cfg.LocalAddress, RemoteAddress: cfg.RemoteAddress,
		Log: cfg.Log, Listen: cfg.Listen, DialTimeout: cfg.DialTimeout, Dial: cfg.Dial,
		NewHandler: func(ctx proxycore.HandlerContext) proxycore.Handler {
			pl := player.New(ctx.Log, player.MonitoringState{CurrentTime: time.Now()}, ctx.Listener)
			pl.SetConn(ctx.Client)
			component.Register(pl)
			detection.Register(pl)
			if cfg.Configure != nil {
				cfg.Configure(pl)
			}
			return &oomphHandler{player: pl}
		},
	})
}

type oomphHandler struct {
	player *player.Player
}

func (h *oomphHandler) Start(backend Backend) error {
	h.player.SetServerConn(backend)
	go h.player.StartTicking()
	return nil
}

func (h *oomphHandler) HandleClientPacket(pkCtx *PacketContext) {
	adaptPacketContexts([]*PacketContext{pkCtx}, h.player.HandleClientPackets)
}

func (h *oomphHandler) HandleServerPacket(pkCtx *PacketContext) {
	adaptPacketContexts([]*PacketContext{pkCtx}, h.player.HandleServerPackets)
}

func (h *oomphHandler) HandleClientBatch(batch []*PacketContext) {
	adaptPacketContexts(batch, h.player.HandleClientPackets)
}

func (h *oomphHandler) HandleServerBatch(batch []*PacketContext) {
	adaptPacketContexts(batch, h.player.HandleServerPackets)
}

// adaptPacketContexts bridges proxy packet contexts to Oomph's handling contexts, runs handle on
// the converted batch and copies cancellation and modification results back.
func adaptPacketContexts(batch []*PacketContext, handle func([]*playercontext.HandlePacketContext)) {
	pks := make([]packet.Packet, len(batch))
	converted := make([]*playercontext.HandlePacketContext, len(batch))
	for i, pkCtx := range batch {
		pks[i] = pkCtx.Packet()
		converted[i] = playercontext.NewHandlePacketContext(&pks[i])
	}
	handle(converted)
	for i, ctx := range converted {
		if ctx.Cancelled() {
			batch[i].Cancel()
		}
		if ctx.Modified() {
			batch[i].SetPacket(pks[i])
		}
	}
}

func (h *oomphHandler) TransferBackend(backend Backend) error {
	_, err := h.player.TransferServerConn(backend)
	return err
}

func (h *oomphHandler) ChunkRadius() int32 {
	return h.player.WorldUpdater().ChunkRadius()
}

func (h *oomphHandler) TransferFailed(address string, _ error) {
	h.player.Message("<red>Unable to transfer to %s.</red>", address)
}

func (h *oomphHandler) Close() error {
	return h.player.Close()
}
