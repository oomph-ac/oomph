package entity

import "github.com/go-gl/mathgl/mgl32"

// HistoricalPosition is a position of an entity that was recorded at a certain tick.
// TODO: Add more fields (such as teleport).
type HistoricalPosition struct {
	Position mgl32.Vec3
	Teleport bool
	Tick     int64
}

// Rewind looks back in the position history of the entity, and returns the position at the given tick.
func (e *Entity) Rewind(tick int64) (HistoricalPosition, bool) {
	if len(e.PositionHistory) == 0 {
		return HistoricalPosition{}, false
	}

	for _, hp := range e.PositionHistory {
		if hp.Tick == tick {
			return hp, true
		}
	}
	return HistoricalPosition{}, false
}
