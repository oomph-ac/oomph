package acknowledgement

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/cloud/packet"
	cloudpacket "github.com/oomph-ac/oomph/cloud/packet"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/player"
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
		mPlayer: p,
		mEntity: e,

		width:  width,
		height: height,
		scale:  scale,
	}
}

func (ack *EntitySize) Run() {
	widthModified, heightModified, scaleModified := ack.width != ack.mEntity.Width, ack.height != ack.mEntity.Height, ack.scale != ack.mEntity.Scale
	ack.mEntity.Width = ack.width
	ack.mEntity.Height = ack.height
	ack.mEntity.Scale = ack.scale

	if widthModified || heightModified || scaleModified {
		pk := &cloudpacket.UpdateEntityDimensions{}
		pk.RuntimeId = ack.mEntity.RuntimeId
		if widthModified {
			pk.SetWidth(ack.width)
		}
		if heightModified {
			pk.SetHeight(ack.height)
		}
		if scaleModified {
			pk.SetScale(ack.scale)
		}
		ack.mPlayer.WriteToCloud(pk)
	}
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
	e := ack.mPlayer.ClientEntityTracker().FindEntity(ack.runtimeID)
	if e == nil || e.PrevRecvPosition == e.RecvPosition {
		return
	}
	ack.mPlayer.WriteToCloud(&packet.UpdateEntityPosition{
		RuntimeId:    ack.runtimeID,
		Position:     e.RecvPosition,
		IsClientView: false,
	})
}
