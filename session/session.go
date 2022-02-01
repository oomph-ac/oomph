package session

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/justtaldevelops/oomph/entity"
	"sync"
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
	ServerSentMotion mgl32.Vec3 // todo: handle this in other places
	EntityData       atomic.Value
	Flags            uint64
	Gamemode         int32
	clickMu          sync.Mutex
	clicks           []uint64
	clickDelay       uint64
	cps              int
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
func (s *Session) GetEntityData() entity.Entity {
	return s.EntityData.Load().(entity.Entity)
}

// Click will add a click to the players clicks.
func (s *Session) Click(currentTick uint64) {
	s.clickMu.Lock()
	if !s.HasFlag(FlagClicking) {
		s.SetFlag(FlagClicking)
	}
	if len(s.clicks) > 0 {
		var max uint64
		for _, tick := range s.clicks {
			if tick > max {
				max = tick
			}
		}
		s.clickDelay = currentTick - max*50
	} else {
		s.clickDelay = 0
	}
	s.clicks = append(s.clicks, currentTick)
	var clicks []uint64
	for _, clickTick := range s.clicks {
		if currentTick-clickTick <= 20 {
			clicks = append(clicks, clickTick)
		}
	}
	s.clicks = clicks
	s.cps = len(s.clicks)
	s.clickMu.Unlock()
}

// CPS returns the clicks per second of the session.
func (s *Session) CPS() int {
	return s.cps
}

// ClickDelay returns the delay between the current click and the last one.
func (s *Session) ClickDelay() uint64 {
	return s.clickDelay
}
