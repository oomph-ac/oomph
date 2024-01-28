package world

import "runtime"

const queueSize = 128 * 128

func init() {
	for i := 0; i < runtime.NumCPU()*4; i++ {
		go worker(i)
	}
}

var queuedChunks = make(chan *AddChunkRequest, queueSize)

func worker(id int) {
	for req := range queuedChunks {
		// Search for a chunk in cache that is equal to the chunk in the request. If a matching
		// chunk is found, we add it to the world.
		matching := cacheSearchMatch(req.pos, req.c)
		if matching != nil {
			matching.Subscribe(req.w)
			continue
		}

		// Insert the chunk into the cache, and then add it to the world.
		cached := NewCached(req.pos, req.c)
		cached.Subscribe(req.w)
	}
}
