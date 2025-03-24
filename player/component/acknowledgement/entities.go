package acknowledgement

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/player"
)

type EntityPosition struct {
	mPlayer *player.Player

	position  mgl32.Vec3
	runtimeID uint64
	teleport  bool
}

func NewEntityPositionACK(p *player.Player, pos mgl32.Vec3, rid uint64, teleport bool) *EntityPosition {
	return &EntityPosition{
		mPlayer:   p,
		position:  pos,
		runtimeID: rid,
		teleport:  teleport,
	}
}

func (ack *EntityPosition) Run() {
	ack.mPlayer.EntityTracker().MoveEntity(
		ack.runtimeID,
		0,
		ack.position,
		ack.teleport,
	)
	ack.mPlayer = nil
}
