package world

import (
	"runtime"
	"time"

	"github.com/sasha-s/go-deadlock"
	"github.com/sirupsen/logrus"
)

func init() {
	for i := 0; i < runtime.NumCPU(); i++ {
		go worker(i)
	}

	go checkWorkers()
}

type workerHeartbeat struct {
	timestamp time.Time
	req       *AddChunkRequest
}

var currentWorkerID = 0
var workerHeartbeats = make(map[int]workerHeartbeat, runtime.NumCPU())
var workerHeartbeatMu deadlock.RWMutex

var queuedChunks = make(chan *AddChunkRequest)

func checkWorkers() {
	t := time.NewTicker(time.Second * 5)
	for {
		<-t.C
		workerHeartbeatMu.RLock()
		for i, h := range workerHeartbeats {
			if time.Since(h.timestamp) >= time.Second*5 {
				logrus.Warnf("worker %v failed on req [chunk: %v, world ID: %v]", i, h.req.pos, h.req.w.id)
				go worker(currentWorkerID)
			}
		}
		workerHeartbeatMu.RUnlock()
	}
}

func worker(id int) {
	currentWorkerID++
	logrus.Infof("chunk worker %v started", id)
	for req := range queuedChunks {
		workerHandleRequest(id, req)
	}
}

func workerHandleRequest(id int, req *AddChunkRequest) {
	workerHeartbeatMu.Lock()
	workerHeartbeats[id] = workerHeartbeat{
		timestamp: time.Now(),
		req:       req,
	}
	workerHeartbeatMu.Unlock()

	defer func() {
		workerHeartbeatMu.Lock()
		delete(workerHeartbeats, id)
		workerHeartbeatMu.Unlock()
	}()

	// Search for a chunk in cache that is equal to the chunk in the request. If a matching
	// chunk is found, we add it to the world.
	matching := cacheSearchMatch(req.pos, req.c)
	if matching != nil {
		matching.Subscribe(req.w)
		return
	}

	// Insert the chunk into the cache, and then add it to the world.
	cached := NewCached(req.pos, req.c)
	cached.Subscribe(req.w)
}
