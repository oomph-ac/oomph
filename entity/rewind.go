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
// History is ordered by tick, so once the delta stops improving we can stop scanning.
func (e *Entity) Rewind(tick int64) (HistoricalPosition, bool) {
	if e.PositionHistory == nil || e.PositionHistory.Size() == 0 {
		e.debug("no position history available - attempting to re-create entity buffer", "runtime_id", e.RuntimeId)
		if e.historySize <= 0 {
			panic(oerror.New("entity.Rewind: unable to re-create entity rewind buffer: recorded history size is zero"))
		}
		e.PositionHistory = utils.NewCircularQueue(e.historySize, func() (hp HistoricalPosition) { return })
		return HistoricalPosition{}, false
	}

	buf, head, size := e.PositionHistory.Items()
	bufLen := len(buf)

	var (
		result    HistoricalPosition
		found     bool
		bestDelta int64
	)

	for i := 0; i < size; i++ {
		hp := buf[(head+i)%bufLen]

		// uninitialized slots.
		if hp.Tick == 0 {
			continue
		}

		if hp.Tick == tick {
			return hp, true
		}

		currentDelta := hp.Tick - tick
		if currentDelta < 0 {
			currentDelta = -currentDelta
		}

		if !found || currentDelta < bestDelta {
			bestDelta = currentDelta
			result = hp
			found = true
			continue
		}

		// Delta started increasing, so we are past the closest match.
		break
	}

	return result, found
}
