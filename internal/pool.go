package internal

import (
	"bytes"
	"sync"
)

const (
	maxChunkBufSize  = 65535
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
	chunkBufPool = sync.Pool{
		New: func() any {
			return bytes.NewBuffer(make([]byte, 0, maxChunkBufSize))
		},
	}
)

func NewBatchBuf() *bytes.Buffer {
	buf := batchPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func PutBatchBuf(buf *bytes.Buffer) {
	if buf == nil || buf.Cap() != maxBatchBufSize {
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
	if buf == nil || buf.Cap() != maxPacketBufSize {
		return
	}
	packetPool.Put(buf)
}

func NewChunkBuf() *bytes.Buffer {
	buf := chunkBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

func PutChunkBuf(buf *bytes.Buffer) {
	if buf == nil || buf.Cap() != maxChunkBufSize {
		return
	}
	chunkBufPool.Put(buf)
}
