package internal

import (
	"bytes"
	"sync"

	df_world "github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/chunk"
	"github.com/oomph-ac/oomph/world"
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

var ChunkPool = sync.Pool{
	New: func() interface{} {
		return chunk.New(world.AirRuntimeID, df_world.Overworld.Range())
	},
}
