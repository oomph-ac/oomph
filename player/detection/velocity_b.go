package detection

import (
	"github.com/chewxy/math32"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Please note that this check isn't actually neccessary, and is only used as a mitigation alert. Any type of
// Velocity will be mitigated due to the authoritative movement system.
type VelocityB struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_VelocityB(p *player.Player) *VelocityB {
	return &VelocityB{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    5,
			MaxBuffer:     10,
			TrustDuration: player.TicksPerSecond * 30,
			MaxViolations: 10,
		},
	}
}

func (*VelocityB) Type() string {
	return TYPE_VELOCITY
}

func (*VelocityB) SubType() string {
	return "B"
}

func (*VelocityB) Description() string {
	return "Checks if the player is taking horizontal knockback as expected."
}

func (*VelocityB) Punishable() bool {
	return true
}

func (d *VelocityB) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *VelocityB) Detect(pk packet.Packet) {}

func (d *VelocityB) HandleKnockback() {
	movement := d.mPlayer.Movement()
	if movement.Gliding() {
		return
	}

	clientVel := movement.Client().Mov()
	serverVel := movement.Mov()
	if math32.Abs(serverVel.X()) > 0.005 {
		xPct := (clientVel.X() / serverVel.X()) * 100
		if xPct <= 99.05 || xPct >= 110 {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("pct", game.Round32(xPct, 4))
			d.mPlayer.FailDetection(d, data)
			return
		}
	}

	if math32.Abs(serverVel.Z()) > 0.005 {
		zPct := (clientVel.Z() / serverVel.Z()) * 100
		if zPct <= 99.05 || zPct >= 110 {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("pct", game.Round32(zPct, 4))
			d.mPlayer.FailDetection(d, data)
		}
	}
}
