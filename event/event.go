package event

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/handler/ack"
	"github.com/oomph-ac/oomph/internal"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const EventsVersion = "2-dev"

type Event interface {
	ID() byte
	Encode() []byte

	Time() int64
}

type NopEvent struct {
	EvTime int64
}

func (n NopEvent) Time() int64 {
	return n.EvTime
}

func WriteEventHeader(ev Event, buf *bytes.Buffer) {
	binary.Write(buf, binary.LittleEndian, uint64(ev.ID()))
	binary.Write(buf, binary.LittleEndian, uint64(ev.Time()))
}

func DecodeEvents(dat []byte) ([]Event, error) {
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	buf.Write(dat)
	defer internal.BufferPool.Put(buf)

	events := []Event{}
	for buf.Len() > 0 {
		ev, err := DecodeEvent(buf)
		if err != nil {
			return events, oerror.New("error decoding event: %v", err)
		}

		events = append(events, ev)
	}

	return events, nil
}

func DecodeEvent(buf *bytes.Buffer) (Event, error) {
	rawID := binary.LittleEndian.Uint64(buf.Next(8))
	id := byte(rawID)
	t := int64(binary.LittleEndian.Uint64(buf.Next(8)))

	fmt.Println(rawID, t)

	var err error
	switch id {
	case EventIDPackets:
		ev := PacketEvent{}
		ev.EvTime = t

		srvByte, err := buf.ReadByte()
		if err != nil {
			return nil, oerror.New("error reading server flag from PacketEvent: %v", err)
		}
		ev.Server = srvByte == 1

		pkCount := binary.LittleEndian.Uint32(buf.Next(4))
		ev.Packets = make([]packet.Packet, 0, pkCount)

		for i := uint32(0); i < pkCount; i++ {
			pkLen := int(binary.LittleEndian.Uint32(buf.Next(4)))
			pk, err := utils.DecodePacketFromBytes(buf.Next(pkLen), ev.Server)

			if err != nil {
				return nil, oerror.New("error decoding packet from PacketEvent: %v", err)
			}

			ev.Packets = append(ev.Packets, pk)
		}

		return ev, nil
	case EventIDServerTick:
		ev := TickEvent{}
		ev.EvTime = t
		ev.Tick = utils.LInt64(buf.Next(8))
		return ev, err
	case EventIDAckRefresh:
		ev := AckRefreshEvent{}
		ev.EvTime = t
		ev.SendTimestamp = utils.LInt64(buf.Next(8))
		ev.RefreshedTimestmap = utils.LInt64(buf.Next(8))
		return ev, err
	case EventIDAckInsert:
		ev := AckInsertEvent{}
		ev.EvTime = t
		ev.Timestamp = utils.LInt64(buf.Next(8))

		ackCount := int(utils.LInt32(buf.Next(4)))
		ev.Acks = make([]ack.Acknowledgement, ackCount)

		b1 := internal.BufferPool.Get().(*bytes.Buffer)
		b1.Reset()
		defer internal.BufferPool.Put(b1)

		for i := 0; i < ackCount; i++ {
			ackLen := int(utils.LInt32(buf.Next(4)))
			b1.Write(buf.Next(ackLen))

			a := ack.Decode(b1)
			ev.Acks[i] = a

			b1.Reset()
		}

		return ev, err
	case EventIDAddChunk:
		ev := AddChunkEvent{}
		ev.EvTime = t

		ev.Position[0] = utils.LInt32(buf.Next(4))
		ev.Position[1] = utils.LInt32(buf.Next(4))

		ev.Range[0] = int(utils.LInt64(buf.Next(8)))
		ev.Range[1] = int(utils.LInt64(buf.Next(8)))

		serialized := chunk.SerialisedData{}
		subCount := int(utils.LInt32(buf.Next(4)))
		serialized.SubChunks = make([][]byte, subCount)

		for i := 0; i < subCount; i++ {
			subLen := int(utils.LInt32(buf.Next(4)))
			serialized.SubChunks[i] = buf.Next(subLen)
		}

		biomeLen := int(utils.LInt32(buf.Next(4)))
		serialized.Biomes = buf.Next(biomeLen)

		ev.Chunk, err = chunk.DiskDecode(serialized, ev.Range)
		if err != nil {
			return nil, oerror.New("error decoding chunk from AddChunkEvent: %v", err)
		}

		return ev, nil
	default:
		return nil, oerror.New("unknown event: %d", id)
	}
}

const (
	_ = iota
	EventIDPackets
	EventIDServerTick
	EventIDAckRefresh
	EventIDAckInsert
	EventIDAddChunk
)
