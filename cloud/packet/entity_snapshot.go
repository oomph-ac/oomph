package packet

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

const (
	SnapshotTypeUpdate byte = iota
	SnapshotTypeRemove
)

const (
	EntitySnapshotRadius float32 = 256
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
	SnapshotType byte       // 1 byte
	IsPlayer     bool       // 1 byte
	RuntimeId    uint64     // 8 bytes
	Width        float32    // 4 bytes
	Height       float32    // 4 bytes
	Scale        float32    // 4 bytes
	Position     mgl32.Vec3 // 12 bytes
}

func (*EntitySnapshot) ID() uint32 {
	return IDEntitySnapshot
}

func (pk *EntitySnapshot) Marshal(io protocol.IO, cloudProto uint32) {
	io.Uint8(&pk.SnapshotType)
	io.Varuint64(&pk.RuntimeId)
	if pk.SnapshotType == SnapshotTypeRemove {
		return
	}
	io.Bool(&pk.IsPlayer)
	io.Float32(&pk.Width)
	io.Float32(&pk.Height)
	io.Float32(&pk.Scale)
	io.Vec3(&pk.Position)
}
