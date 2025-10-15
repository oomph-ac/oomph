package detection

import (
	"github.com/chewxy/math32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
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
			FailBuffer:    6,
			MaxBuffer:     6,
			MaxViolations: 10,
			//TrustDuration: 60 * player.TicksPerSecond,
		},
	}
}

func (*HitboxA) Type() string {
	return TypeHitbox
}

func (*HitboxA) SubType() string {
	return "A"
}

func (*HitboxA) Description() string {
	return "Checks if the player is using a hitbox modification greater than the one sent by the server."
}

func (*HitboxA) Punishable() bool {
	return true
}

func (d *HitboxA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *HitboxA) Detect(pk packet.Packet) {
	if !d.mPlayer.Opts().Combat.EnableClientEntityTracking {
		return
	}
	switch pk := pk.(type) {
	case *packet.Interact:
		if pk.ActionType != packet.InteractActionMouseOverEntity || pk.Position == utils.EmptyVec32 {
			//d.mPlayer.Message("%d %v", pk.ActionType, pk.Position)
			return
		}
		e := d.mPlayer.ClientEntityTracker().FindEntity(pk.TargetEntityRuntimeID)
		if e == nil || !e.IsPlayer || e.TicksSinceTeleport <= 10 {
			return
		}
		h1 := e.Box(e.PrevPosition).Grow(0.1)
		h2 := e.Box(e.Position).Grow(0.1)
		dist := math32.Min(
			pk.Position.Sub(game.ClosestPointToBBox(pk.Position, h1)).Len(),
			pk.Position.Sub(game.ClosestPointToBBox(pk.Position, h2)).Len(),
		)
		if dist > 0.004 {
			d.mPlayer.FailDetection(d, "amt", game.Round32(0.6+(dist*2), 3))
		} else {
			d.mPlayer.PassDetection(d, d.metadata.Buffer)
		}
	}
}
