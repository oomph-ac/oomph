package ackfunc

import (
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

// OPTS: n/a
func PlayerSetInitalized(p *player.Player, data ...interface{}) {
	p.Ready = true
}

// OPTS: int32
func PlayerUpdateGamemode(p *player.Player, data ...interface{}) {
	p.GameMode = data[0].(int32)
}

// OPTS: float32
func PlayerUpdateSimulationRate(p *player.Player, data ...interface{}) {
	mul := data[0].(float32)
	if mgl32.FloatEqual(mul, 1) {
		p.Tps = 20
		return
	}

	p.Tps *= mul
}

// OPTS: time.Time, int64
func PlayerUpdateLatency(p *player.Player, data ...interface{}) {
	h := p.Handler(handler.HandlerIDLatency).(*handler.LatencyHandler)
	h.StackLatency = p.Time().Sub(data[0].(time.Time)).Milliseconds()
	h.LatencyUpdateTick = data[1].(int64) + 10
	h.Responded = true

	p.ClientTick = data[1].(int64)
}

// OPTS: map[uint32]interface{}
func PlayerUpdateActorData(p *player.Player, data ...interface{}) {
	metadata := data[0].(map[uint32]interface{})
	h := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)

	width, widthExists := metadata[entity.DataKeyBoundingBoxWidth]
	height, heightExists := metadata[entity.DataKeyBoundingBoxHeight]
	if !widthExists {
		width = h.Width
	}
	if !heightExists {
		height = h.Height
	}

	h.Width = width.(float32)
	h.Height = height.(float32)

	f, ok := metadata[entity.DataKeyFlags]
	if !ok {
		return
	}

	flags := f.(int64)
	h.Sprinting = utils.HasDataFlag(entity.DataFlagSprinting, flags)
	h.Sneaking = utils.HasDataFlag(entity.DataFlagSneaking, flags)
	h.Immobile = utils.HasDataFlag(entity.DataFlagImmobile, flags)
}

// OPTS: []protocol.AbilityLayer
func PlayerUpdateAbilities(p *player.Player, data ...interface{}) {
	abilities := data[0].([]protocol.AbilityLayer)
	h := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)

	for _, l := range abilities {
		h.NoClip = utils.HasFlag(uint64(l.Values), protocol.AbilityNoClip)
		h.Flying = utils.HasFlag(uint64(l.Values), protocol.AbilityFlying) || h.NoClip
		mayFly := utils.HasFlag(uint64(l.Values), protocol.AbilityMayFly)

		if h.ToggledFly {
			// If the player toggled flight, but the server did not allow it, we longer trust
			// their flight status. This is done to ensure players that have permission to fly
			// are able to do so w/o any movement corrections, but players that do not have permission
			// to do so aren't able to bypass movement predictions with it.
			h.TrustFlyStatus = h.Flying || mayFly
		}
		h.ToggledFly = false
	}
}

// OPTS: []protocol.Attribute
func PlayerUpdateAttributes(p *player.Player, data ...interface{}) {
	attributes := data[0].([]protocol.Attribute)
	h := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)

	h.HandleAttribute("minecraft:movement", attributes, func(attr protocol.Attribute) {
		h.HasServerSpeed = true
		h.MovementSpeed = float32(attr.Value)
	})
	h.HandleAttribute("minecraft:health", attributes, func(attr protocol.Attribute) {
		p.Alive = attr.Value > 0
	})
}

// OPTS: mgl32.Vec3
func PlayerUpdateKnockback(p *player.Player, data ...interface{}) {
	p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler).SetKnockback(data[0].(mgl32.Vec3))
}

// OPTS: mgl32.Vec3, bool, bool
func PlayerTeleport(p *player.Player, data ...interface{}) {
	p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler).Teleport(
		data[0].(mgl32.Vec3), // TeleportPos
		data[1].(bool),       // OnGround
		data[2].(bool),       // Smooth
	)
}

// OPTS: n/a
func PlayerRecieveCorrection(p *player.Player, data ...interface{}) {
	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	mDat.OutgoingCorrections--
	mDat.RecievedCorrection = true
}
