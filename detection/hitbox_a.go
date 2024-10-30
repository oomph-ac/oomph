package detection

import (
	"github.com/chewxy/math32"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDHitboxA = "oomph:hitbox_a"

type HitboxA struct {
	BaseDetection
}

func NewHitboxA() *HitboxA {
	d := &HitboxA{}
	d.Type = "Hitbox"
	d.SubType = "A"

	d.Description = "Detects if the player interacts with entities outside of their hitbox."
	d.Punishable = true

	d.MaxViolations = 10
	d.trustDuration = -1

	d.FailBuffer = 5
	d.MaxBuffer = 5
	return d
}

func (d *HitboxA) ID() string {
	return DetectionIDHitboxA
}

func (d *HitboxA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if interaction, ok := pk.(*packet.Interact); ok && interaction.ActionType == packet.InteractActionMouseOverEntity && interaction.Position != (mgl32.Vec3{}) {
		entity := p.ClientEntityTracker().FindEntity(interaction.TargetEntityRuntimeID)
		if entity == nil || !entity.IsPlayer {
			return true
		}

		bb1 := entity.Box(entity.PrevPosition).Grow(0.1)
		bb2 := entity.Box(entity.Position).Grow(0.1)

		min := math32.Min(
			interaction.Position.Sub(game.ClosestPointToBBox(interaction.Position, bb1)).Len(),
			interaction.Position.Sub(game.ClosestPointToBBox(interaction.Position, bb2)).Len(),
		)

		if min > 0.004 {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("amt", game.Round32(0.6+(min*2), 3))
			d.Fail(p, data)
		} else {
			d.Debuff(d.FailBuffer)
		}
	}

	return true
}
