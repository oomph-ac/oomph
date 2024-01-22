package world

import (
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/oomph-ac/oomph/oerror"
)

func init() {
	go handleQueue()
}

type job struct {
	req *AddChunkRequest
	res chan bool
}

const queueSize = 128 * 128

var queuedChunks = make(chan *AddChunkRequest, queueSize)
var jobQueue = make(chan job, queueSize)

func spawnWorkers(n int) {
	for i := 0; i < n; i++ {
		go worker(i)
	}
}

func worker(id int) {
	for j := range jobQueue {
		// Search for a chunk in cache that is equal to the chunk in the request. If a matching
		// chunk is found, we add it to the world.
		req := j.req
		matching := cacheSearchMatch(req.pos, req.c)
		if matching != nil {
			matching.Subscribe(req.w)
			j.res <- true
			fmt.Println("worker", id, "found matching chunk")
			continue
		}

		// Insert the chunk into the cache, and then add it to the world.
		cached := NewCached(req.pos, req.c)
		cached.Subscribe(req.w)
		j.res <- true
		fmt.Println("worker", id, "added chunk to cache")
	}
}

func handleQueue() {
	spawnWorkers(16) // TODO: Make this configurable?

	for {
		req, ok := <-queuedChunks
		if !ok {
			panic(oerror.New("chunk queue closed"))
		}

		if req.w == nil {
			continue
		}

		var finishChan = make(chan bool)
		jobQueue <- job{req: req, res: finishChan}

		select {
		case <-finishChan:
			// OK
		case <-time.After(time.Second * 3):
			sentry.CurrentHub().Clone().CaptureMessage("add action timed out for chunk")
		}
	}
}
