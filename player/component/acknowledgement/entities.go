package acknowledgement

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/player"
)

type EntitySize struct {
	mEntity *entity.Entity
	width   float32
	height  float32
	scale   float32
}

func NewEntitySizeACK(e *entity.Entity, width, height, scale float32) *EntitySize {
	return &EntitySize{
		mEntity: e,

		width:  width,
		height: height,
		scale:  scale,
	}
}

func (ack *EntitySize) Run() {
	ack.mEntity.Width = ack.width
	ack.mEntity.Height = ack.height
	ack.mEntity.Scale = ack.scale
	ack.mEntity = nil
}

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
	ack.mPlayer.ClientEntityTracker().MoveEntity(
		ack.runtimeID,
		0,
		ack.position,
		ack.teleport,
	)
	ack.mPlayer = nil
}
