package event

import (
	"bytes"

	"github.com/oomph-ac/oomph/handler/ack"
	"github.com/oomph-ac/oomph/internal"
	"github.com/oomph-ac/oomph/utils"
)

type AckInsertEvent struct {
	NopEvent

	Timestamp int64
	Acks      []ack.Acknowledgement
}

func (AckInsertEvent) ID() byte {
	return EventIDAckInsert
}

func (ev AckInsertEvent) Encode() []byte {
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer internal.BufferPool.Put(buf)

	WriteEventHeader(ev, buf)
	utils.WriteLInt64(buf, ev.Timestamp)

	utils.WriteLInt32(buf, int32(len(ev.Acks)))
	for _, a := range ev.Acks {
		enc := ack.Encode(a)
		utils.WriteLInt32(buf, int32(len(enc)))
		buf.Write(enc)
	}

	return buf.Bytes()
}
