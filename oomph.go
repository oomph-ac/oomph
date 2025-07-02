package oomph

import (
	"os"

	"github.com/getsentry/sentry-go"
)

func init() {
	if dsn := os.Getenv("OOMPH_SENTRY_DSN"); dsn != "" {
		if err := sentry.Init(sentry.ClientOptions{Dsn: dsn}); err != nil {
			panic(err)
		}
	}
}
