package handler

import (
	"time"

	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDRateLimit = "oomph:rate_limit"

const (
	ResetInterval     = 8
	MaxNormalPackets  = 80 * ResetInterval
	MaxSpammedPackets = 1000 * ResetInterval
)

// RateLimitHandler handles the client packet rate limit.
type RateLimitHandler struct {
	NumNormalPackets  int
	NumSpammedPackets int
	LastReset         int64
	DidKick           bool
}

func NewRateLimitHandler() *RateLimitHandler {
	return &RateLimitHandler{}
}

func (RateLimitHandler) ID() string {
	return HandlerIDRateLimit
}

func (h *RateLimitHandler) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	h.NumSpammedPackets++

	if h.checkRateLimit(h.NumSpammedPackets, MaxSpammedPackets, p) {
		return false
	}

	// ignore packets spammed by clients due to a bug on normal limit
	if pk, ok := pk.(*packet.InventoryTransaction); ok {
		if pk.LegacyRequestID == 0 && len(pk.LegacySetItemSlots) == 0 && len(pk.Actions) == 0 {
			if useItemData, ok := pk.TransactionData.(*protocol.UseItemTransactionData); ok {
				stack := useItemData.HeldItem.Stack
				if len(useItemData.Actions) < 50 && len(useItemData.LegacySetItemSlots) < 50 && len(stack.CanBePlacedOn) < 100 && len(stack.CanBreak) < 100 && len(stack.NBTData) < 100 {
					return true
				}
			}
		}
	}
	if pk, ok := pk.(*packet.Animate); ok {
		if pk.ActionType == packet.AnimateActionSwingArm {
			return true
		}
	}

	// ignore nsl packets on normal limit
	if pk.ID() == packet.IDNetworkStackLatency {
		return true
	}

	h.NumNormalPackets++
	return !h.checkRateLimit(h.NumNormalPackets, MaxNormalPackets, p)
}

func (h *RateLimitHandler) checkRateLimit(count, max int, p *player.Player) bool {
	if count > max {
		if h.DidKick {
			return true
		}
		h.DidKick = true
		p.Disconnect("Packet rate limit exceeded.")
		p.BlockAddress(10 * time.Second)
		p.Log().Warnf("%s was removed from the server due to exceeding packet rate limit.", p.IdentityDat.DisplayName)
		return true
	}
	return false
}

func (h *RateLimitHandler) HandleServerPacket(pk packet.Packet, p *player.Player) bool {
	return true
}

func (h *RateLimitHandler) OnTick(p *player.Player) {
	if p.ServerTick > h.LastReset+(ResetInterval*player.TicksPerSecond) {
		h.LastReset = p.ServerTick
		h.NumNormalPackets = 0
		h.NumSpammedPackets = 0
	}
}

func (*RateLimitHandler) Defer() {
}
