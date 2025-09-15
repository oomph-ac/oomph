package packet

import "github.com/sandertv/gophertunnel/minecraft/protocol"

func init() {
	Register(IDUpdateEntityDimensions, func() Packet { return &UpdateEntityDimensions{} })
}

const (
	UpdateEntityDimensionsFlagUpdatedWidth = 1 << iota
	UpdateEntityDimensionsFlagUpdatedHeight
	UpdateEntityDimensionsFlagUpdatedScale
)

type UpdateEntityDimensions struct {
	XUID      string  // 1-5 bytes + len(XUID)
	Flags     uint8   // 1 byte
	RuntimeId uint64  // 1-9 bytes
	Width     float32 // 0-4 bytes
	Height    float32 // 0-4 bytes
	Scale     float32 // 0-4 bytes
}

func (*UpdateEntityDimensions) ID() uint32 {
	return IDUpdateEntityDimensions
}

func (pk *UpdateEntityDimensions) Marshal(io protocol.IO, cloudProto uint32) {
	io.String(&pk.XUID)
	io.Uint8(&pk.Flags)
	io.Varuint64(&pk.RuntimeId)
	if pk.CheckFlag(UpdateEntityDimensionsFlagUpdatedWidth) {
		io.Float32(&pk.Width)
	}
	if pk.CheckFlag(UpdateEntityDimensionsFlagUpdatedHeight) {
		io.Float32(&pk.Height)
	}
	if pk.CheckFlag(UpdateEntityDimensionsFlagUpdatedScale) {
		io.Float32(&pk.Scale)
	}
}

func (pk *UpdateEntityDimensions) SetWidth(width float32) {
	pk.Width = width
	pk.Flags |= UpdateEntityDimensionsFlagUpdatedWidth
}

func (pk *UpdateEntityDimensions) SetHeight(height float32) {
	pk.Height = height
	pk.Flags |= UpdateEntityDimensionsFlagUpdatedHeight
}

func (pk *UpdateEntityDimensions) SetScale(scale float32) {
	pk.Scale = scale
	pk.Flags |= UpdateEntityDimensionsFlagUpdatedScale
}

func (pk *UpdateEntityDimensions) CheckFlag(flag uint8) bool {
	return pk.Flags&flag == flag
}
