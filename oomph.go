package oomph

import (
	"os"
	"time"

	"github.com/getsentry/sentry-go"
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

	if dsn := os.Getenv("OOMPH_SENTRY_DSN"); dsn != "" {
		if err := sentry.Init(sentry.ClientOptions{Dsn: dsn}); err != nil {
			panic(err)
		}
	}
}
