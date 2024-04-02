package handler

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDLatency = "oomph:latency"

type LatencyReportType int

const (
	LatencyReportRaknet LatencyReportType = iota
	LatencyReportGameStack
)

// LatencyHandler updates the latency and client tick of the player, which is used for synchronization.
type LatencyHandler struct {
	StackLatency      int64
	LatencyUpdateTick int64
	ReportType        LatencyReportType

	Responded bool
}

func NewLatencyHandler() *LatencyHandler {
	return &LatencyHandler{}
}

func (LatencyHandler) ID() string {
	return HandlerIDLatency
}

func (h *LatencyHandler) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if p.MState.IsReplay {
		return true
	}

	if _, ok := pk.(*packet.PlayerAuthInput); ok && p.ServerTick%5 == 0 {
		latency := p.Conn().Latency().Milliseconds() * 2
		if h.ReportType == LatencyReportGameStack {
			latency = h.StackLatency
		}

		p.SendRemoteEvent(player.NewUpdateLatencyEvent(
			latency,
			-1, // deprecated
		))
	}

	return true
}

func (h *LatencyHandler) HandleServerPacket(pk packet.Packet, p *player.Player) bool {
	switch pk := pk.(type) {
	case *packet.LevelChunk, *packet.SubChunk:
		if p.Ready {
			return true
		}

		h.Responded = true
		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
			p.Ready = true
		})
	case *packet.LevelEvent:
		// TODO: Also account for LevelEventSimTimeStep
		if pk.EventType != packet.LevelEventSimTimeScale {
			return true
		}

		p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
			scale := pk.Position.X()
			if mgl32.FloatEqualThreshold(scale, 1, 1e-5) {
				p.Tps = 20.0
				return
			}

			p.Tps *= scale
		})
	}

	return true
}

func (h *LatencyHandler) OnTick(p *player.Player) {
	if p.ClientTick < h.LatencyUpdateTick {
		return
	}

	if !h.Responded {
		return
	}
	h.Responded = false

	currentTime := p.Time()
	currentTick := p.ServerTick

	p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
		h.StackLatency = p.Time().Sub(currentTime).Milliseconds()
		p.ClientTick = currentTick

		h.LatencyUpdateTick = currentTick + 10
		h.Responded = true
	})
}

func (*LatencyHandler) Defer() {
}
