package event

import (
	"bytes"
	"encoding/binary"

	"github.com/oomph-ac/oomph/internal"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type PacketEvent struct {
	NopEvent

	Packets []packet.Packet
	Server  bool
}

func (PacketEvent) ID() byte {
	return EventIDPackets
}

func (ev PacketEvent) Encode() []byte {
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer internal.BufferPool.Put(buf)

	WriteEventHeader(ev, buf)

	// Write the server flag to the buffer
	if ev.Server {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}

	// Write the number of packets to the buffer
	binary.Write(buf, binary.LittleEndian, uint32(len(ev.Packets)))

	// Write each packet to the buffer
	for _, pk := range ev.Packets {
		dat := utils.EncodePacketToBytes(pk)
		binary.Write(buf, binary.LittleEndian, uint32(len(dat)))
		buf.Write(dat)
	}

	return buf.Bytes()
}
