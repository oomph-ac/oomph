package event

import (
	"bytes"

	"github.com/oomph-ac/oomph/internal"
	"github.com/oomph-ac/oomph/utils"
)

type TickEvent struct {
	NopEvent

	Tick int64
}

func (TickEvent) ID() byte {
	return EventIDServerTick
}

func (ev TickEvent) Encode() []byte {
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer internal.BufferPool.Put(buf)

	WriteEventHeader(ev, buf)
	utils.WriteLInt64(buf, ev.Tick)

	return buf.Bytes()
}
