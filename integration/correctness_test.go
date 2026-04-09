package integration_test

import (
	"testing"

	"github.com/oomph-ac/oomph/player/component"
)

func TestDetectionBufferAndViolationFlow(t *testing.T) {
	p := newReplayPlayer(t)
	d := newMockDetection("Integration", "A", 3, 5, 10)

	// Fails below the threshold should only increase the buffer.
	p.FailDetection(d, "signal", "a")
	p.FailDetection(d, "signal", "b")
	if got := d.Metadata().Violations; got != 0 {
		t.Fatalf("expected no violations before fail buffer threshold, got %.2f", got)
	}
	if got := d.Metadata().Buffer; got != 2 {
		t.Fatalf("expected buffer 2 after two fails, got %.2f", got)
	}

	// Reaching the threshold should start increasing violations.
	p.FailDetection(d, "signal", "c")
	if got := d.Metadata().Violations; got != 1 {
		t.Fatalf("expected one violation after third fail, got %.2f", got)
	}
	if got := d.Metadata().Buffer; got != 3 {
		t.Fatalf("expected buffer 3 after threshold fail, got %.2f", got)
	}

	// Passing should decay only the buffer, not violations.
	p.PassDetection(d, 1.5)
	if got := d.Metadata().Buffer; got != 1.5 {
		t.Fatalf("expected buffer decay to 1.5, got %.2f", got)
	}
	if got := d.Metadata().Violations; got != 1 {
		t.Fatalf("expected violations to remain 1 after pass, got %.2f", got)
	}
}

func TestDetectionSequenceDeterministic(t *testing.T) {
	runScript := func() (buffer, violations float64) {
		p := newReplayPlayer(t)
		d := newMockDetection("Integration", "Deterministic", 2, 4, 10)

		// Mixed sequence representative of noisy gameplay signal.
		p.FailDetection(d, "frame", 1)
		p.PassDetection(d, 0.5)
		p.FailDetection(d, "frame", 2)
		p.FailDetection(d, "frame", 3)
		p.PassDetection(d, 1.0)
		p.FailDetection(d, "frame", 4)
		p.FailDetection(d, "frame", 5)

		return d.Metadata().Buffer, d.Metadata().Violations
	}

	b1, v1 := runScript()
	b2, v2 := runScript()
	if b1 != b2 || v1 != v2 {
		t.Fatalf("expected deterministic scoring, run1=(%.2f, %.2f), run2=(%.2f, %.2f)", b1, v1, b2, v2)
	}
}

type countingACK struct {
	count *int
}

func (a *countingACK) Run() {
	*a.count++
}

func TestACKExecuteRunsBatchesInOrder(t *testing.T) {
	p := newReplayPlayer(t)
	component.Register(p)
	ack := component.NewACKComponent(p)
	ack.SetLegacy(true)
	p.SetACKs(ack)

	executed := 0
	ack.Add(&countingACK{count: &executed})
	firstTS := ack.Timestamp()
	ack.Flush()

	ack.Add(&countingACK{count: &executed})
	secondTS := ack.Timestamp()
	ack.Flush()

	if !ack.Execute(secondTS) {
		t.Fatalf("expected ack execute to return true for second timestamp")
	}
	if executed != 2 {
		t.Fatalf("expected two ACK callbacks to run in-order, got %d", executed)
	}
	if ack.Responsive() != true {
		t.Fatalf("expected ACK component to remain responsive")
	}

	// First timestamp was already consumed when second was executed.
	if ack.Execute(firstTS) {
		t.Fatalf("expected first timestamp to no longer be pending")
	}
}
