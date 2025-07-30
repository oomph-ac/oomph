package packet

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

const (
	EntitySnapshotTypeUpdate byte = iota
	EntitySnapshotTypeRemove
)

func init() {
	Register(IDEntitySnapshot, func() Packet {
		return &EntitySnapshot{}
	})
}

// EntitySnapshot represents a snapshot of an entity in the game world. Ideally, these snapshots are only sent for
// entities that are within a close range of the player to reduce network load. Furthermore, these snapshots should only
// be sent if there was a change made to the state of the entity.
type EntitySnapshot struct {
	SnapshotType byte   // 1 byte
	IsPlayer     bool   // 1 byte
	RuntimeId    uint64 // 8 bytes

	Width  protocol.Optional[float32] // 1-5 bytes
	Height protocol.Optional[float32] // 1-5 bytes
	Scale  protocol.Optional[float32] // 1-5 bytes

	Position protocol.Optional[mgl32.Vec3] // 1-13 bytes
	NetPos   protocol.Optional[mgl32.Vec3] // 1-13 bytes
}

func (*EntitySnapshot) ID() uint32 {
	return IDEntitySnapshot
}

func (pk *EntitySnapshot) Marshal(io protocol.IO, cloudProto uint32) {
	io.Uint8(&pk.SnapshotType)
	io.Varuint64(&pk.RuntimeId)
	if pk.SnapshotType == EntitySnapshotTypeRemove {
		return
	}
	io.Bool(&pk.IsPlayer)
	protocol.OptionalFunc(io, &pk.Width, io.Float32)
	protocol.OptionalFunc(io, &pk.Height, io.Float32)
	protocol.OptionalFunc(io, &pk.Scale, io.Float32)
	protocol.OptionalFunc(io, &pk.Position, io.Vec3)
}
