package worker

import (
	"runtime"

	"github.com/getsentry/sentry-go"
)

var workerQueue = make(chan func(), runtime.NumCPU())

func init() {
	for i := 0; i < runtime.NumCPU(); i++ {
		go worker()
	}
}

func worker() {
	defer sentry.Recover()

	for {
		f, ok := <-workerQueue
		if !ok {
			return
		}

		f()
	}
}

// To be used by a function that may be CPU intensive.
func Submit(f func()) {
	workerQueue <- f
}
