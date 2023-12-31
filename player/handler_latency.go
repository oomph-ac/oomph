package player

import (
	"fmt"
	"time"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDLatency = "oomph:latency"

type LatencyHandler struct {
	StackLatency      int64
	LatencyUpdateTick int64

	Responded bool
}

func (LatencyHandler) ID() string {
	return HandlerIDLatency
}

func (h *LatencyHandler) HandleClientPacket(pk packet.Packet, p *Player) bool {
	if _, ok := pk.(*packet.TickSync); ok {
		h.Responded = true
	}

	return true
}

func (h LatencyHandler) HandleServerPacket(pk packet.Packet, p *Player) bool {
	return true
}

func (h *LatencyHandler) OnTick(p *Player) {
	if p.clientTick < h.LatencyUpdateTick {
		return
	}

	if !h.Responded {
		return
	}
	h.Responded = false

	currentTime := time.Now()
	currentTick := p.serverTick

	p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
		h.StackLatency = time.Since(currentTime).Milliseconds()
		p.clientTick = currentTick

		h.LatencyUpdateTick = currentTick + 10
		h.Responded = true
		p.Message(fmt.Sprintf("Latency: %dms", h.StackLatency))
	})
}
