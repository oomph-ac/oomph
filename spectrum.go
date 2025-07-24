package oomph

import (
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/cooldogedev/spectrum/session"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component"
	"github.com/oomph-ac/oomph/player/context"
	"github.com/oomph-ac/oomph/player/detection"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var _ session.Processor = &Processor{}

type Processor struct {
	session.NopProcessor

	identity     login.IdentityData
	registry     *session.Registry
	pl           atomic.Pointer[player.Player]
	transferring atomic.Bool

	dbgTransfer bool
}

func NewProcessor(
	s *session.Session,
	registry *session.Registry,
	listener *minecraft.Listener,
	log *slog.Logger,
) *Processor {
	pl := player.New(log, player.MonitoringState{
		IsReplay:    false,
		IsRecording: false,
		CurrentTime: time.Now(),
	}, listener)
	pl.SetConn(s.Client())

	component.Register(pl)
	detection.Register(pl)

	go pl.StartTicking()
	p := &Processor{identity: s.Client().IdentityData(), registry: registry}
	p.pl.Store(pl)
	return p
}

func (p *Processor) ProcessStartGame(ctx *session.Context, gd *minecraft.GameData) {
	//gd.PlayerMovementSettings.MovementType = protocol.PlayerMovementModeServerWithRewind
	gd.PlayerMovementSettings.RewindHistorySize = 100
}

func (p *Processor) ProcessServer(ctx *session.Context, pk *packet.Packet) {
	pl := p.pl.Load()
	if pl == nil || p.transferring.Load() {
		return
	}

	pkCtx := context.NewHandlePacketContext(pk)
	pl.HandleServerPacket(pkCtx)

	if pkCtx.Cancelled() {
		ctx.Cancel()
		return
	}
}

func (p *Processor) ProcessClient(ctx *session.Context, pk *packet.Packet) {
	if os.Getenv("DBG") != "" {
		if txt, ok := (*pk).(*packet.Text); ok && txt.Message == "transferme" {
			if !p.dbgTransfer {
				p.registry.GetSession(p.identity.XUID).Transfer("127.0.0.1:20002")
				p.dbgTransfer = true
			} else {
				p.registry.GetSession(p.identity.XUID).Transfer("127.0.0.1:20000")
				p.dbgTransfer = false
			}
		}
	}

	pl := p.pl.Load()
	if pl == nil || pl.Conn() == nil || p.transferring.Load() {
		return
	}

	pkCtx := context.NewHandlePacketContext(pk)
	pl.HandleClientPacket(pkCtx)

	if pkCtx.Cancelled() {
		ctx.Cancel()
		return
	}
}

func (p *Processor) ProcessFlush(ctx *session.Context) {
	pl := p.pl.Load()
	if pl == nil || p.transferring.Load() {
		return
	}

	pl.PauseProcessing()
	defer pl.ResumeProcessing()

	// We want Oomph to flush the connection whilst processing is stopped to prevent it from handling other packets before the connection
	// is actually flushed.
	ctx.Cancel()

	if acks := pl.ACKs(); acks != nil {
		acks.Flush()
		if err := pl.Conn().Flush(); err != nil {
			pl.Log().Error("error flushing client connection", "error", err)
		}
	}
}

func (p *Processor) ProcessPreTransfer(*session.Context, *string, *string) {
	p.transferring.Store(true)
	if pl := p.pl.Load(); pl != nil {
		pl.PauseProcessing()
	}
}

func (p *Processor) ProcessPostTransfer(_ *session.Context, _ *string, _ *string) {
	p.transferring.Store(false)
	if s, pl := p.registry.GetSession(p.identity.XUID), p.pl.Load(); s != nil && pl != nil {
		pl.SetServerConn(s.Server())
		// Remove save-the-world state from the player's world, we can start removing chunks once this ACK is processed.
		if w := pl.World(); w != nil {
			w.SetSTWTicks(300) // 15 seconds
		}
		pl.Effects().RemoveAll()
		pl.ACKs().Invalidate() // Invalidate all pending UpdateBlock acknowledgments.
		pl.ResumeProcessing()
	}
}

func (p *Processor) ProcessTransferFailure(_ *session.Context, origin *string, target *string) {
	p.transferring.Store(false)
	if s, pl := p.registry.GetSession(p.identity.XUID), p.pl.Load(); s != nil && pl != nil {
		pl.ResumeProcessing()
	}
}

func (p *Processor) ProcessDisconnection(_ *session.Context, _ *string) {
	if pl := p.pl.Load(); pl != nil {
		_ = pl.Close()
		p.pl.Store(nil)
	}
}

func (p *Processor) Player() *player.Player {
	return p.pl.Load()
}
