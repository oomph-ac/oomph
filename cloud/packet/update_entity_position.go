package packet

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func init() {
	Register(IDUpdateEntityPosition, func() Packet { return &UpdateEntityPosition{} })
}

type UpdateEntityPosition struct {
	RuntimeId    uint64
	Position     mgl32.Vec3
	IsClientView bool
}

func (*UpdateEntityPosition) ID() uint32 {
	return IDUpdateEntityPosition
}

func (pk *UpdateEntityPosition) Marshal(io protocol.IO, cloudProto uint32) {
	io.Uint64(&pk.RuntimeId)
	io.Vec3(&pk.Position)
	io.Bool(&pk.IsClientView)
}
