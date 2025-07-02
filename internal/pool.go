package internal

import (
	"bytes"
	"sync"
)

var BufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 256))
	},
}
