package event

import (
	"bytes"
	"encoding/base64"
	"encoding/json"

	"github.com/oomph-ac/oomph/internal"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
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
	dat := map[string]interface{}{
		"EvTime": ev.Time(),
		"Server": ev.Server,
	}

	pks := []string{}
	for _, pk := range ev.Packets {
		buf := internal.BufferPool.Get().(*bytes.Buffer)
		buf.Reset()

		header := &packet.Header{}
		header.PacketID = pk.ID()
		header.Write(buf)

		pk.Marshal(protocol.NewWriter(buf, 0))
		pks = append(pks, base64.StdEncoding.EncodeToString(buf.Bytes()))

		internal.BufferPool.Put(buf)
	}

	dat["Packets"] = pks

	enc, err := json.Marshal(dat)
	if err != nil {
		panic(oerror.New("unable to encode packet event: " + err.Error()))
	}

	// Append the event ID to the start of the encoded event.
	return append([]byte{EventIDPackets}, enc...)
}
