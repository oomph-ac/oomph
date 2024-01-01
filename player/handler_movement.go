package player

import "github.com/sandertv/gophertunnel/minecraft/protocol/packet"

const HandlerIDMovement = "oomph:movement"

type MovementHandler struct {
}

func (MovementHandler) ID() string {
	return HandlerIDMovement
}

func (h *MovementHandler) HandleClientPacket(pk packet.Packet, p *Player) bool {
	input, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	// Update client tick and simulation frame.
	p.clientFrame = int64(input.Tick)
	p.clientTick++

	return true
}

func (MovementHandler) HandleServerPacket(pk packet.Packet, p *Player) bool {
	return true
}

func (MovementHandler) OnTick(p *Player) {}
