package acknowledgement

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/player"
)

// UpdateEntityPositionACK is an acknowledgment that is ran when the player recieves
// the position of another entity.
type UpdateEntityPositionACK struct {
	mPlayer *player.Player

	pos       mgl32.Vec3
	runtimeId uint64
	tick      int64
	teleport  bool
	instant   bool
}

func NewUpdateEntityPositionACK(p *player.Player, pos mgl32.Vec3, runtimeId uint64, tick int64, teleport, instant bool) *UpdateEntityPositionACK {
	return &UpdateEntityPositionACK{
		mPlayer: p,

		pos:       pos,
		runtimeId: runtimeId,
		tick:      tick,
		teleport:  teleport,
		instant:   instant,
	}
}

func (ack *UpdateEntityPositionACK) Run() {
	if ack.instant {
		ack.mPlayer.EntityTracker().MoveEntity(ack.runtimeId, ack.tick, ack.pos, ack.teleport)
	} else {
		ack.mPlayer.ClientEntityTracker().MoveEntity(ack.runtimeId, ack.tick, ack.pos, ack.teleport)
	}
}
