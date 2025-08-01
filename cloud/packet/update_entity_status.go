package packet

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

const (
	UpdateEntityStatusFlagIsRemoval uint8 = 1 << iota
	UpdateEntityStatusFlagIsPlayer
)

func init() {
	Register(IDUpdateEntityStatus, func() Packet { return &UpdateEntityStatus{} })
}

type UpdateEntityStatus struct {
	RuntimeId uint64

	NetPosition protocol.Optional[mgl32.Vec3]
	Position    mgl32.Vec3
	Dimensions  mgl32.Vec3 // [width, height, scale]
	EntityType  string

	Flags uint8
}

func (*UpdateEntityStatus) ID() uint32 {
	return IDUpdateEntityStatus
}

func (pk *UpdateEntityStatus) Marshal(io protocol.IO, cloudProto uint32) {
	io.Uint64(&pk.RuntimeId)
	io.Uint8(&pk.Flags)

	if !pk.CheckFlag(UpdateEntityStatusFlagIsRemoval) {
		protocol.OptionalFunc(io, &pk.NetPosition, io.Vec3)
		io.Vec3(&pk.Position)
		io.Vec3(&pk.Dimensions)
		io.String(&pk.EntityType)
	}
}

func (pk *UpdateEntityStatus) CheckFlag(flag uint8) bool {
	return pk.Flags&flag == flag
}

func (pk *UpdateEntityStatus) AddFlag(flag uint8) {
	pk.Flags |= flag
}

func (pk *UpdateEntityStatus) RemoveFlag(flag uint8) {
	pk.Flags &= ^flag
}
