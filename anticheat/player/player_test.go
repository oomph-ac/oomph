package player

import (
	"log/slog"
	"testing"
	"time"
)

func TestBlockAddressWithoutRakNetListener(t *testing.T) {
	p := New(slog.Default(), MonitoringState{CurrentTime: time.Now()}, nil)
	p.BlockAddress(time.Second)
}
