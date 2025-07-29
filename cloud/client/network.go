package client

import (
	"time"
)

type ConnectionInfo struct {
	FlushRate    time.Duration
	CmpThreshold uint32
}
