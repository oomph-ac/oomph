package detection

import (
	"github.com/chewxy/math32"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type ReachB struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_ReachB(p *player.Player) *ReachB {
	d := &ReachB{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1.01,
			MaxBuffer:     3,
			MaxViolations: 15,
			TrustDuration: 20 * player.TicksPerSecond,
		},
	}
	p.ClientCombat().Hook(d.run)
	return d
}

func (*ReachB) Type() string {
	return TypeReach
}

func (*ReachB) SubType() string {
	return "B"
}

func (*ReachB) Description() string {
	return "Checks if the distance between the player and the closest point of the entity is greater than the vanilla limit."
}

func (*ReachB) Punishable() bool {
	return true
}

func (d *ReachB) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *ReachB) Detect(packet.Packet) {}

func (d *ReachB) run(c player.CombatComponent) {
	if d.mPlayer.Movement().TicksSinceTeleport() <= 20 || d.mPlayer.Movement().InCorrectionCooldown() {
		return
	}
	minReach := float32(math32.MaxFloat32)
	for _, dist := range c.Raws() {
		if dist < minReach {
			minReach = dist
		}
	}
	if minReach > 2.9 {
		d.mPlayer.FailDetection(d)
		d.mPlayer.Log().Debug("reach(B)", "min", minReach, "vl", d.metadata.Violations)
	} else {
		d.mPlayer.PassDetection(d, 0.001)
	}
}
