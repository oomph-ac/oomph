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
	XUID       string     // 1-5 bytes + len(XUID)
	Flags      uint8      // 1 byte
	RuntimeId  uint64     // 1-9 bytes
	Position   mgl32.Vec3 // 0-12 bytes
	Dimensions mgl32.Vec3 // 0-12 bytes ([w, h, s])
	EntityType string     // 0-12 bytes
}

func (*UpdateEntityStatus) ID() uint32 {
	return IDUpdateEntityStatus
}

func (pk *UpdateEntityStatus) Marshal(io protocol.IO, cloudProto uint32) {
	io.String(&pk.XUID)
	io.Uint8(&pk.Flags)
	io.Varuint64(&pk.RuntimeId)

	if !pk.CheckFlag(UpdateEntityStatusFlagIsRemoval) {
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
