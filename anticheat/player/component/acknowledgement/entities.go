package acknowledgement

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/anticheat/entity"
	"github.com/oomph-ac/oomph/anticheat/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type EntityData struct {
	mEntity  *entity.Entity
	metadata map[uint32]any
	width    float32
	height   float32
	scale    float32
}

func NewEntityDataACK(e *entity.Entity, metadata map[uint32]any, width, height, scale float32) *EntityData {
	return &EntityData{
		mEntity:  e,
		metadata: metadata,

		width:  width,
		height: height,
		scale:  scale,
	}
}

func (ack *EntityData) Run() {
	ack.mEntity.UpdateMetadata(ack.metadata)
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

type EntityDeltaPosition struct {
	mPlayer *player.Player

	positionX protocol.Optional[float32]
	positionY protocol.Optional[float32]
	positionZ protocol.Optional[float32]
	runtimeID uint64
	teleport  bool
}

func NewEntityDeltaPositionACK(p *player.Player, posX, posY, posZ protocol.Optional[float32], rid uint64, teleport bool) *EntityDeltaPosition {
	return &EntityDeltaPosition{
		mPlayer:   p,
		positionX: posX,
		positionY: posY,
		positionZ: posZ,
		runtimeID: rid,
		teleport:  teleport,
	}
}

func (ack *EntityDeltaPosition) Run() {
	ack.mPlayer.ClientEntityTracker().MoveEntityDelta(
		ack.runtimeID,
		0,
		ack.positionX,
		ack.positionY,
		ack.positionZ,
		ack.teleport,
	)
	ack.mPlayer = nil
}
