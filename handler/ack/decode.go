package ack

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func Decode(buf *bytes.Buffer) Acknowledgement {
	id := utils.LInt32(buf.Next(4))
	a := New(AckID(id))

	switch a.ID {
	case AckWorldSetBlock:
		// OPTS: cube.Pos, world.Block
		pos := cube.Pos{
			int(utils.LInt32(buf.Next(4))),
			int(utils.LInt32(buf.Next(4))),
			int(utils.LInt32(buf.Next(4))),
		}

		len := utils.LInt32(buf.Next(4))
		bName := buf.Next(int(len))

		var dat map[string]interface{}
		len = utils.LInt32(buf.Next(4))
		err := json.Unmarshal(buf.Next(int(len)), &dat)
		if err != nil {
			panic(oerror.New("decode block properties: %v", err))
		}

		b, ok := world.BlockByName(string(bName), dat)
		if !ok {
			panic(oerror.New("block %s not found", bName))
		}

		a.Data = append(a.Data, pos, b)
	case AckWorldUpdateChunks:
		// OPTS: packet.Packet
		pkLen := utils.LInt32(buf.Next(4))
		pk, err := utils.DecodePacketFromBytes(buf.Next(int(pkLen)), true)

		if err != nil {
			panic(oerror.New("decode packet: %v", err))
		}

		a.Data = append(a.Data, pk)
	case AckEntityUpdatePosition:
		// OPTS: uint64, int64, mgl32.Vec3, bool
		eid := uint64(utils.LInt64(buf.Next(8)))
		tick := utils.LInt64(buf.Next(8))
		pos := utils.ReadVec32(buf.Next(12))
		onGround := false
		if buf.Len() > 0 {
			// ??????? what the fuck
			onGround = utils.Bool(buf.Next(1))
		}

		a.Data = append(a.Data, eid, tick, pos, onGround)
	case AckPlayerInitalized:
		// OPTS: none
	case AckPlayerUpdateGamemode:
		// OPTS: int32
		gamemode := utils.LInt32(buf.Next(4))
		a.Data = append(a.Data, gamemode)
	case AckPlayerUpdateSimulationRate:
		// OPTS: float32
		simRate := utils.LFloat32(buf.Next(4))
		a.Data = append(a.Data, simRate)
	case AckPlayerUpdateLatency:
		// OPTS: time.Time, int64
		tN := utils.LInt64(buf.Next(8))
		t := time.Unix(0, tN)
		clientTick := utils.LInt64(buf.Next(8))

		a.Data = append(a.Data, t, clientTick)
	case AckPlayerUpdateActorData:
		// OPTS: map[uint32]interface{}
		len := utils.LInt32(buf.Next(4))
		var metadata map[uint32]interface{}
		err := json.Unmarshal(buf.Next(int(len)), &metadata)
		if err != nil {
			panic(oerror.New("parse metadata from JSON: %v", err))
		}

		a.Data = append(a.Data, metadata)
	case AckPlayerUpdateAbilities:
		// OPTS: []protocol.Layer
		amt := utils.LInt32(buf.Next(4))
		abilities := make([]protocol.AbilityLayer, amt)

		for i := 0; i < int(amt); i++ {
			abilities[i] = protocol.AbilityLayer{
				Type:      uint16(utils.LInt32(buf.Next(4))),
				Abilities: uint32(utils.LInt32(buf.Next(4))),
				Values:    uint32(utils.LInt32(buf.Next(4))),
				FlySpeed:  utils.LFloat32(buf.Next(4)),
				WalkSpeed: utils.LFloat32(buf.Next(4)),
			}
		}

		a.Data = append(a.Data, abilities)
	case AckPlayerUpdateAttributes:
		// OPTS: []protocol.Attribute
		len := utils.LInt32(buf.Next(4))
		var attributes []protocol.Attribute

		err := json.Unmarshal(buf.Next(int(len)), &attributes)
		if err != nil {
			panic(oerror.New("parse attributes from JSON: %v", err))
		}

		a.Data = append(a.Data, attributes)
	case AckPlayerUpdateKnockback:
		// OPTS: mgl32.Vec3
		vel := utils.ReadVec32(buf.Next(12))
		a.Data = append(a.Data, vel)
	case AckPlayerTeleport:
		// OPTS: mgl32.Vec3, bool, bool
		pos := utils.ReadVec32(buf.Next(12))
		ground := utils.Bool(buf.Next(1))
		smooth := utils.Bool(buf.Next(1))

		a.Data = append(a.Data, pos, ground, smooth)
	case AckPlayerRecieveCorrection:
		// OPTS: n/a
	default:
		panic(oerror.New("acknowledgement id %d not found", a.ID))
	}

	a.f = FuncMap[a.ID]
	return a
}
