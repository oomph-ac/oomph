package acknowledgement

import (
	"github.com/go-gl/mathgl/mgl32"
	cloudpacket "github.com/oomph-ac/oomph/cloud/packet"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type EntitySize struct {
	mPlayer *player.Player
	mEntity *entity.Entity

	width  float32
	height float32
	scale  float32
}

func NewEntitySizeACK(p *player.Player, e *entity.Entity, width, height, scale float32) *EntitySize {
	return &EntitySize{
		mEntity: e,

		width:  width,
		height: height,
		scale:  scale,
	}
}

func (ack *EntitySize) Run() {
	prevWidth, prevHeight, prevScale := ack.mEntity.Width, ack.mEntity.Height, ack.mEntity.Scale
	ack.mEntity.Width = ack.width
	ack.mEntity.Height = ack.height
	ack.mEntity.Scale = ack.scale

	entitySnapshot := &cloudpacket.EntitySnapshot{
		SnapshotType: cloudpacket.EntitySnapshotTypeUpdate,
		RuntimeId:    ack.mEntity.RuntimeId,
		IsPlayer:     ack.mEntity.IsPlayer,
	}
	// We only want to send the snapshot if the size has actually changed - and the server isn't just doing some weird logic
	// where it wastes bandwidth re-sending this without needing to.
	sendSnapshot := false
	if prevWidth != ack.width {
		entitySnapshot.Width = protocol.Option(ack.width)
		sendSnapshot = true
	}
	if prevHeight != ack.height {
		entitySnapshot.Height = protocol.Option(ack.height)
		sendSnapshot = true
	}
	if prevScale != ack.scale {
		entitySnapshot.Scale = protocol.Option(ack.scale)
		sendSnapshot = true
	}
	if sendSnapshot {
		ack.mPlayer.WriteToCloud(entitySnapshot)
	}
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
