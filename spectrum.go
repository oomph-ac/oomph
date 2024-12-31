package oomph

import (
	"time"

	"github.com/cooldogedev/spectrum/session"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component"
	"github.com/oomph-ac/oomph/player/detection"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/text"
	"github.com/sirupsen/logrus"
)

var _ session.Processor = &Processor{}

type Processor struct {
	session.NopProcessor

	identity login.IdentityData
	registry *session.Registry
	pl       *player.Player
}

func NewProcessor(
	s *session.Session,
	registry *session.Registry,
	listener *minecraft.Listener,
	log *logrus.Logger,
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
	return &Processor{
		identity: s.Client().IdentityData(),
		registry: registry,
		pl:       pl,
	}
}

func (p *Processor) Player() *player.Player {
	return p.pl
}

func (p *Processor) ProcessStartGame(ctx *session.Context, gd *minecraft.GameData) {
	gd.PlayerMovementSettings.MovementType = protocol.PlayerMovementModeServerWithRewind
	gd.PlayerMovementSettings.RewindHistorySize = 20
}

func (p *Processor) ProcessServer(ctx *session.Context, pk packet.Packet) {
	if p.pl != nil {
		ctx.Cancel()
		if err := p.pl.HandleServerPacket(pk); err != nil {
			p.disconnect(text.Colourf("error while processing server packet: %s", err.Error()))
		}
	}
}

func (p *Processor) ProcessClient(ctx *session.Context, pk packet.Packet) {
	if p.pl != nil {
		ctx.Cancel()
		if err := p.pl.HandleClientPacket(pk); err != nil {
			p.disconnect(text.Colourf("error while processing client packet: %s", err.Error()))
		}
	}
}

func (p *Processor) ProcessPostTransfer(_ *session.Context, _ *string, _ *string) {
	if s := p.registry.GetSession(p.identity.XUID); s != nil && p.pl != nil {
		p.pl.SetServerConn(s.Server())
	}
}

func (p *Processor) ProcessDisconnection(_ *session.Context) {
	if p.pl != nil {
		_ = p.pl.Close()
		p.pl = nil
	}
}

func (p *Processor) disconnect(reason string) {
	if s := p.registry.GetSession(p.identity.XUID); s != nil {
		s.Disconnect(reason)
	}
}
