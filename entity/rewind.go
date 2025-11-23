package entity

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/utils"
)

// HistoricalPosition is a position of an entity that was recorded at a certain tick.
type HistoricalPosition struct {
	Position     mgl32.Vec3
	PrevPosition mgl32.Vec3

	Teleport bool
	Tick     int64
}

// Rewind looks back in the position history of the entity, and returns the position at the given tick.
func (e *Entity) Rewind(tick int64) (HistoricalPosition, bool) {
	if e.PositionHistory.Size() == 0 {
		e.debug("no position history available - attempting to re-create entity buffer", "runtime_id", e.RuntimeId)
		if e.historySize <= 0 {
			panic(oerror.New("entity.Rewind: unable to re-create entity rewind buffer: recorded history size is zero"))
		}
		e.PositionHistory = utils.NewCircularQueue(e.historySize, func() (hp HistoricalPosition) { return })
		return HistoricalPosition{}, false // We can't return anything here because we just re-created the buffer.
	}

	var (
		result HistoricalPosition
		delta  int64 = 1_000_000_000_000
	)

	for hp := range e.PositionHistory.Iter() {
		if hp.Tick == tick {
			return hp, true
		}

		currentDelta := hp.Tick - tick
		if currentDelta < 0 {
			currentDelta *= -1
		}

		if currentDelta <= delta {
			result = hp
			delta = currentDelta
		}
	}

	return result, true
}
