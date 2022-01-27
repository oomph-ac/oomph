package session

import (
	"github.com/justtaldevelops/oomph/entity"
	"sync/atomic"
)

type Session struct {
	Ticks struct {
		// Climable represents the tick passed since the player was in a climable block.
		Climable uint32
		// Cobweb represents the ticks passed since the player was in a cobweb.
		Cobweb uint32
		// Liquid represents the ticks passed since the player was in liquid.
		Liquid uint32
		// Motion represents the ticks passed since the player has last moved.
		Motion uint32
	}
	EntityData atomic.Value
	Flags      uint64
	Gamemode   int32
}

// SetFlag sets a bit flag for the session, or unsets if the session already has the flag. A list of flags can be seen in flags.go
func (s *Session) SetFlag(flag uint32) {
	s.Flags ^= 1 << flag
}

// HasFlag returns whether the session has a specified bitflag.
func (s *Session) HasFlag(flag uint32) bool {
	return s.Flags&(1<<flag) > 0
}

// GetEntityData returns the entity data of the session
func (s Session) GetEntityData() entity.Entity {
	return s.EntityData.Load().(entity.Entity)
}
