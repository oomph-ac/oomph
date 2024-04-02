package internal

import (
	"bytes"
	"sync"
)

var BufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer([]byte{})
	},
}

var MapPool = sync.Pool{
	New: func() interface{} {
		return make(map[string]interface{})
	},
}
