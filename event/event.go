package event

import (
	"bytes"
	"encoding/base64"
	"encoding/json"

	"github.com/oomph-ac/oomph/internal"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

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

func DefaultEncode(ev Event) []byte {
	enc, err := json.Marshal(ev)
	if err != nil {
		panic(oerror.New("error encoding event: " + err.Error()))
	}

	return append([]byte{ev.ID()}, enc...)
}

func Decode(dat []byte, other ...any) (Event, error) {
	id := dat[0]
	switch id {
	case EventIDPackets:
		if len(other) < 1 {
			return nil, oerror.New("protocol required for decoding packet event")
		}
		proto := other[0].(minecraft.Protocol)

		dec := internal.MapPool.Get().(map[string]interface{})
		defer internal.MapPool.Put(dec)

		// Reset the map.
		for k := range dec {
			delete(dec, k)
		}

		err := json.Unmarshal(dat[1:], &dec)
		if err != nil {
			return nil, err
		}

		ev := PacketEvent{
			Server: dec["Server"].(bool),
		}
		ev.EvTime = int64(dec["EvTime"].(float64))

		packetPool := proto.Packets(!ev.Server)

		pks := dec["Packets"].([]interface{})
		ev.Packets = make([]packet.Packet, 0, len(pks))
		for _, encPk := range pks {
			encPk := encPk.(string)
			b64dec, err := base64.StdEncoding.DecodeString(encPk)
			if err != nil {
				return nil, oerror.New("error decoding base64 packet: %v", err)
			}

			buf := internal.BufferPool.Get().(*bytes.Buffer)
			buf.Reset()
			buf.Write(b64dec)

			h := &packet.Header{}
			if err = h.Read(buf); err != nil {
				return nil, oerror.New("error reading packet header: %v", err)
			}

			pkFunc, ok := packetPool[h.PacketID]
			if !ok {
				ev.Packets = append(ev.Packets, &packet.Unknown{})
				continue
			}

			pk := pkFunc()
			pk.Marshal(protocol.NewReader(buf, 0, false))

			if buf.Len() != 0 {
				panic(oerror.New("packet buffer not empty after reading packet: %d", buf.Len()))
			}

			ev.Packets = append(ev.Packets, pk)
			internal.BufferPool.Put(buf)
		}

		return ev, nil
	case EventIDServerTick:
		ev := TickEvent{}
		err := json.Unmarshal(dat[1:], &ev)
		return ev, err
	case EventIDAck:
		ev := AckEvent{}
		err := json.Unmarshal(dat[1:], &ev)
		return ev, err
	default:
		return nil, oerror.New("unknown event: %d", id)
	}
}

const (
	_ = iota
	EventIDPackets
	EventIDServerTick
	EventIDAck
)
