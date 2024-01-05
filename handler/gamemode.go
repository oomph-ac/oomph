package handler

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDGamemode = "oomph:gamemode"

type GamemodeHandler struct {
}

func NewGamemodeHandler() *GamemodeHandler {
	return &GamemodeHandler{}
}

func (GamemodeHandler) ID() string {
	return HandlerIDGamemode
}

func (h *GamemodeHandler) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	return true
}

func (h *GamemodeHandler) HandleServerPacket(pk packet.Packet, p *player.Player) bool {
	gm, ok := pk.(*packet.SetPlayerGameType)
	if !ok {
		return true
	}

	// Wait for the client to acknowledge the gamemode change.
	p.Handler(HandlerIDAcknowledgements).(*AcknowledgementHandler).AddCallback(func() {
		p.GameMode = gm.GameType
	})
	return true
}

func (h *GamemodeHandler) OnTick(p *player.Player) {}
