package handler

import (
	"time"

	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDLatency = "oomph:latency"

// LatencyHandler updates the latency and client tick of the player, which is used for synchronization.
type LatencyHandler struct {
	StackLatency      int64
	LatencyUpdateTick int64

	Responded bool
}

func NewLatencyHandler() *LatencyHandler {
	return &LatencyHandler{}
}

func (LatencyHandler) ID() string {
	return HandlerIDLatency
}

func (h *LatencyHandler) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if _, ok := pk.(*packet.PlayerAuthInput); ok && p.ServerTick%5 == 0 {
		p.SendRemoteEvent(player.NewUpdateLatencyEvent(
			p.Conn().Latency().Milliseconds()*2,
			h.StackLatency,
		))
	}

	return true
}

func (h *LatencyHandler) HandleServerPacket(pk packet.Packet, p *player.Player) bool {
	if p.Ready {
		return true
	}

	switch pk.(type) {
	case *packet.LevelChunk, *packet.SubChunk:
		p.Ready = true
		h.Responded = true
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

	currentTime := time.Now()
	currentTick := p.ServerTick

	p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
		h.StackLatency = time.Since(currentTime).Milliseconds()
		p.ClientTick = currentTick

		h.LatencyUpdateTick = currentTick + 10
		h.Responded = true
	})
}

func (*LatencyHandler) Defer() {
}
