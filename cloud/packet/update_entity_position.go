package packet

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func init() {
	Register(IDUpdateEntityPosition, func() Packet { return &UpdateEntityPosition{} })
}

type UpdateEntityPosition struct {
	CloudID uint64 // 1-9 bytes

	RuntimeId    uint64     // 8 bytes
	Position     mgl32.Vec3 // 12 bytes
	IsClientView bool       // 1 byte
}

func (*UpdateEntityPosition) ID() uint32 {
	return IDUpdateEntityPosition
}

func (pk *UpdateEntityPosition) Marshal(io protocol.IO, cloudProto uint32) {
	io.Varuint64(&pk.CloudID)
	io.Uint64(&pk.RuntimeId)
	io.Vec3(&pk.Position)
	io.Bool(&pk.IsClientView)
}
