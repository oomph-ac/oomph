package detection

import (
	"github.com/chewxy/math32"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type HitboxA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_HitboxA(p *player.Player) *HitboxA {
	return &HitboxA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer: 5,
			MaxBuffer:  5,

			MaxViolations: 10,
		},
	}
}

func (*HitboxA) Type() string {
	return TYPE_HITBOX
}

func (*HitboxA) SubType() string {
	return "A"
}

func (*HitboxA) Description() string {
	return "Detects if the player is attacking an entity without looking at their hitbox."
}

func (*HitboxA) Punishable() bool {
	return true
}

func (d *HitboxA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *HitboxA) Detect(pk packet.Packet) {
	if interaction, ok := pk.(*packet.Interact); ok && interaction.ActionType == packet.InteractActionMouseOverEntity && interaction.Position != (mgl32.Vec3{}) {
		entity := d.mPlayer.ClientEntityTracker().FindEntity(interaction.TargetEntityRuntimeID)
		if entity == nil || !entity.IsPlayer {
			return
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
			d.mPlayer.FailDetection(d, data)
		} else {
			d.mPlayer.PassDetection(d, d.metadata.FailBuffer)
		}
	}
}
