package internal

import (
	"bytes"
	"sync"
)

const (
	maxBatchBufSize  = 65535
	maxPacketBufSize = 256
)

var (
	batchPool = sync.Pool{
		New: func() any {
			return bytes.NewBuffer(make([]byte, 0, maxBatchBufSize))
		},
	}
	packetPool = sync.Pool{
		New: func() any {
			return bytes.NewBuffer(make([]byte, 0, maxPacketBufSize))
		},
	}
)

func NewBatchBuf() *bytes.Buffer {
	buf := batchPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func PutBatchBuf(buf *bytes.Buffer) {
	if buf == nil || buf.Cap() > maxBatchBufSize {
		return
	}
	batchPool.Put(buf)
}

func NewPacketBuf() *bytes.Buffer {
	buf := packetPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func PutPacketBuf(buf *bytes.Buffer) {
	if buf == nil || buf.Cap() > maxPacketBufSize {
		return
	}
	packetPool.Put(buf)
}
