package world

import (
	"runtime"

	"github.com/sirupsen/logrus"
)

func init() {
	for i := 0; i < runtime.NumCPU(); i++ {
		go worker(i)
	}
}

var queuedChunks = make(chan *AddChunkRequest)

func worker(id int) {
	logrus.Infof("chunk worker %v started", id)
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
