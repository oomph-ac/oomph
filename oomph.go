package oomph

import (
	"os"
	"time"

	"github.com/sasha-s/go-deadlock"
)

func init() {
	deadlock.Opts.Disable = true
	if os.Getenv("DEADLOCK_DEBUG") == "true" {
		deadlock.Opts.Disable = false
		deadlock.Opts.DeadlockTimeout = time.Second * 5
		deadlock.Opts.DisableLockOrderDetection = true
		deadlock.Opts.PrintAllCurrentGoroutines = true
	}
}
