package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

func init() {
	Register(IDUpdateEntityDimensions, func() Packet { return &UpdateEntityDimensions{} })
}

type UpdateEntityDimensions struct {
	RuntimeId uint64
	Width     protocol.Optional[float32]
	Height    protocol.Optional[float32]
	Scale     protocol.Optional[float32]
}

func (*UpdateEntityDimensions) ID() uint32 {
	return IDUpdateEntityDimensions
}

func (pk *UpdateEntityDimensions) Marshal(io protocol.IO, cloudProto uint32) {
	io.Uint64(&pk.RuntimeId)
	protocol.OptionalFunc(io, &pk.Width, io.Float32)
	protocol.OptionalFunc(io, &pk.Height, io.Float32)
	protocol.OptionalFunc(io, &pk.Scale, io.Float32)
}
