package utils

import (
	"bytes"

	"github.com/oomph-ac/oomph/internal"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func EncodePacketToBytes(pk packet.Packet) []byte {
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	defer internal.BufferPool.Put(buf)

	buf.Reset()

	header := &packet.Header{}
	header.PacketID = pk.ID()
	header.Write(buf)

	pk.Marshal(protocol.NewWriter(buf, 0))
	return buf.Bytes()
}

func DecodePacketFromBytes(b []byte, fromServer bool) (packet.Packet, error) {
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	defer internal.BufferPool.Put(buf)

	buf.Reset()
	buf.Write(b)

	packetPool := minecraft.DefaultProtocol.Packets(!fromServer)

	h := &packet.Header{}
	if err := h.Read(buf); err != nil {
		return nil, oerror.New("error reading packet header: %v", err)
	}

	pkFunc, ok := packetPool[h.PacketID]
	if !ok {
		return nil, oerror.New("packet not found in packet pool: %d", h.PacketID)
	}

	pk := pkFunc()
	pk.Marshal(protocol.NewReader(buf, 0, false))
	return pk, nil
}
