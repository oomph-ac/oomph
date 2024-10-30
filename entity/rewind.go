package entity

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/assert"
)

// HistoricalPosition is a position of an entity that was recorded at a certain tick.
// TODO: Add more fields (such as teleport).
type HistoricalPosition struct {
	Position     mgl32.Vec3
	PrevPosition mgl32.Vec3

	Teleport bool
	Tick     int64
}

// Rewind looks back in the position history of the entity, and returns the position at the given tick.
func (e *Entity) Rewind(tick int64) (HistoricalPosition, bool) {
	if len(e.PositionHistory) == 0 {
		return HistoricalPosition{}, false
	}

	var (
		result HistoricalPosition
		delta  int64 = 1_000_000
	)

	for _, hp := range e.PositionHistory {
		if hp.Tick == tick {
			return hp, true
		}

		cDelta := hp.Tick - tick
		if cDelta < 0 {
			cDelta *= -1
		}

		if cDelta < delta {
			result = hp
			delta = cDelta
		}
	}

	assert.IsTrue(delta != 1_000_000, "result for rewind at end-of-function should be found, but is not")
	return result, true
}
