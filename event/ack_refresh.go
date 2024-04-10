package event

import (
	"bytes"

	"github.com/oomph-ac/oomph/internal"
	"github.com/oomph-ac/oomph/utils"
)

type AckRefreshEvent struct {
	NopEvent

	SendTimestamp      int64
	RefreshedTimestmap int64
}

func (AckRefreshEvent) ID() byte {
	return EventIDAckRefresh
}

func (ev AckRefreshEvent) Encode() []byte {
	buf := internal.BufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer internal.BufferPool.Put(buf)

	WriteEventHeader(ev, buf)
	utils.WriteLInt64(buf, ev.SendTimestamp)
	utils.WriteLInt64(buf, ev.RefreshedTimestmap)

	return buf.Bytes()
}
