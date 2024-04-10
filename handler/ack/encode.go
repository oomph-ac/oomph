package ack

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"time"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/internal"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func Encode(a Acknowledgement) []byte {
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer internal.BufferPool.Put(buf)

	utils.WriteLInt32(buf, int32(a.ID))
	switch a.ID {
	case AckWorldSetBlock:
		// OPTS: cube.Pos, world.Block
		pos := a.Data[0].(cube.Pos)
		b := a.Data[1].(world.Block)

		utils.WriteLInt32(buf, int32(pos[0]))
		utils.WriteLInt32(buf, int32(pos[1]))
		utils.WriteLInt32(buf, int32(pos[2]))

		n, dat := b.EncodeBlock()
		utils.WriteLInt32(buf, int32(len(n)))
		buf.Write([]byte(n))

		enc, err := json.Marshal(dat)
		if err != nil {
			panic(err)
		}

		utils.WriteLInt32(buf, int32(len(enc)))
		buf.Write(enc)
	case AckWorldUpdateChunks:
		// OPTS: packet.Packet
		pk := a.Data[0].(packet.Packet)
		enc := utils.EncodePacketToBytes(pk)

		utils.WriteLInt32(buf, int32(len(enc)))
		buf.Write(enc)
	case AckEntityUpdatePosition:
		// OPTS: uint64, int64, mgl32.Vec3, bool
		binary.Write(buf, binary.LittleEndian, a.Data[0].(uint64))
		utils.WriteLInt32(buf, int32(a.Data[1].(int64)))

		pos := a.Data[2].(mgl32.Vec3)
		utils.WriteVec32(buf, pos)

		utils.WriteBool(buf, a.Data[3].(bool))
	case AckPlayerInitalized:
		// OPTS: n/a
	case AckPlayerUpdateGamemode:
		// OPTS: int32
		utils.WriteLInt32(buf, a.Data[0].(int32))
	case AckPlayerUpdateSimulationRate:
		// OPTS: float32
		utils.WriteLFloat32(buf, a.Data[0].(float32))
	case AckPlayerUpdateLatency:
		// OPTS: time.Time, int64
		t := a.Data[0].(time.Time)
		clientTick := a.Data[1].(int64)

		utils.WriteLInt64(buf, t.UnixNano())
		utils.WriteLInt64(buf, clientTick)
	case AckPlayerUpdateActorData:
		// OPTS: map[uint32]interface{}
		metadata := a.Data[0].(map[uint32]interface{})
		enc, err := json.Marshal(metadata)
		if err != nil {
			panic(oerror.New("parse metadata to JSON: %v", err))
		}

		utils.WriteLInt32(buf, int32(len(enc)))
		buf.Write(enc)
	case AckPlayerUpdateAbilities:
		// OPTS: []protocol.AbilityLayer
		abilities := a.Data[0].([]protocol.AbilityLayer)
		utils.WriteLInt32(buf, int32(len(abilities)))

		for _, l := range abilities {
			utils.WriteLInt32(buf, int32(l.Type))      // uint16
			utils.WriteLInt32(buf, int32(l.Abilities)) // uint32
			utils.WriteLInt32(buf, int32(l.Values))    // uint32
			utils.WriteLFloat32(buf, l.FlySpeed)
			utils.WriteLFloat32(buf, l.WalkSpeed)
		}
	case AckPlayerUpdateAttributes:
		// OPTS: []protocol.Attribute
		enc, err := json.Marshal(a.Data[0].([]protocol.Attribute))
		if err != nil {
			panic(oerror.New("parse attributes to JSON: %v", err))
		}

		utils.WriteLInt32(buf, int32(len(enc)))
		buf.Write(enc)
	case AckPlayerUpdateKnockback:
		// OPTS: mgl32.Vec3
		vel := a.Data[0].(mgl32.Vec3)
		utils.WriteLFloat32(buf, vel[0])
		utils.WriteLFloat32(buf, vel[1])
		utils.WriteLFloat32(buf, vel[2])
	case AckPlayerTeleport:
		// OPTS: mgl32.Vec3, bool, bool
		pos := a.Data[0].(mgl32.Vec3)
		ground := a.Data[1].(bool)
		smooth := a.Data[2].(bool)

		utils.WriteVec32(buf, pos)
		utils.WriteBool(buf, ground)
		utils.WriteBool(buf, smooth)
	default:
		panic(oerror.New("acknowledgement id %d not found", a.ID))
	}

	return buf.Bytes()
}
