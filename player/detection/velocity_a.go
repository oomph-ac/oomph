package detection

import (
	"fmt"

	"github.com/chewxy/math32"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type VelocityA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_VelocityA(p *player.Player) *VelocityA {
	return &VelocityA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    2,
			MaxBuffer:     4,
			TrustDuration: player.TicksPerSecond * 30,
			MaxViolations: 10,
		},
	}
}

func (*VelocityA) Type() string {
	return TYPE_VELOCITY
}

func (*VelocityA) SubType() string {
	return "A"
}

func (*VelocityA) Description() string {
	return "Checks if the player is taking vertical knockback as expected."
}

func (*VelocityA) Punishable() bool {
	return true
}

func (d *VelocityA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

// We don't need to do anything here, we can wait for the movement component to call HandleKnockback
func (d *VelocityA) Detect(pk packet.Packet) {}

func (d *VelocityA) HandleKnockback() {
	movement := d.mPlayer.Movement()
	if movement.Gliding() {
		return
	}

	clientVel := movement.Client().Mov().Y()
	serverVel := movement.Mov().Y()
	if math32.Abs(serverVel) < 0.005 {
		return
	}

	fmt.Println(clientVel/serverVel, clientVel, serverVel)
}
