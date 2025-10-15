package detection

import (
	"github.com/chewxy/math32"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type ReachA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_ReachA(p *player.Player) *ReachA {
	d := &ReachA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1.01,
			MaxBuffer:     1.5,
			MaxViolations: 7, // this is lucky number :))))

			TrustDuration: 60 * player.TicksPerSecond,
		},
	}
	p.ClientCombat().Hook(d.run)
	return d
}

func (*ReachA) Type() string {
	return TypeReach
}

func (*ReachA) SubType() string {
	return "A"
}

func (*ReachA) Description() string {
	return "Checks if the player's combat reach exceeds the vanilla value - only applicable for non-touch clients."
}

func (*ReachA) Punishable() bool {
	return true
}

func (d *ReachA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *ReachA) Detect(packet.Packet) {}

func (d *ReachA) run(c player.CombatComponent) {
	if d.mPlayer.InputMode == packet.InputModeTouch || d.mPlayer.Movement().TicksSinceTeleport() <= 20 || d.mPlayer.Movement().InCorrectionCooldown() {
		return
	}
	raycasts := c.Raycasts()
	if len(raycasts) == 0 {
		return
	}

	minReach := float32(math32.MaxFloat32)
	maxReach := float32(0)
	for _, dist := range c.Raycasts() {
		if dist > maxReach {
			maxReach = dist
		}
		if dist < minReach {
			minReach = dist
		}
	}
	if minReach > 2.9 && maxReach > 3 {
		d.mPlayer.FailDetection(d)
		d.mPlayer.Log().Debug("reach(A)", "min", minReach, "max", maxReach, "vl", d.metadata.Violations)
	} else {
		d.mPlayer.PassDetection(d, 0.0015)
	}
}
