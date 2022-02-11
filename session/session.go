package session

import (
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
		// Spawn represents the ticks passed since the player last respawned.
		Spawn uint32
	}
	Movement      *Movement
	EntityData    atomic.Value
	Flags         uint64
	Gamemode      int32
	clickMu       sync.Mutex
	clicks        []uint64
	lastClickTick uint64
	clickDelay    uint64
	cps           int
}

// setFlag sets a bit flag for the session, or unsets if the session already has the flag. A list of flags can be seen in flags.go
func (s *Session) setFlag(flag uint64) {
	s.Flags ^= flag
}

// SetFlag will set or remove a bit flag based on the value of set.
func (s *Session) SetFlag(set bool, flag uint64) {
	if set {
		if !s.HasFlag(flag) {
			s.setFlag(flag)
		}
	} else if s.HasFlag(flag) {
		s.setFlag(flag)
	}
}

// HasFlag returns whether the session has a specified bitflag.
func (s *Session) HasFlag(flag uint64) bool {
	return s.Flags&flag > 0
}

// HasAnyFlag returns whether the session has any of the specified bitflags.
func (s *Session) HasAnyFlag(flags ...uint64) bool {
	for _, flag := range flags {
		if s.HasFlag(flag) {
			return true
		}
	}
	return false
}

// HasAllFlags returns true if the session has all the flags specified.
func (s *Session) HasAllFlags(flags ...uint64) bool {
	for _, flag := range flags {
		if !s.HasFlag(flag) {
			return false
		}
	}
	return true
}

// GetEntityData returns the entity data of the session
func (s *Session) GetEntityData() entity.Entity {
	return s.EntityData.Load().(entity.Entity)
}

// Click will add a click to the players clicks.
func (s *Session) Click(currentTick uint64) {
	s.clickMu.Lock()
	s.SetFlag(true, FlagClicking)
	if len(s.clicks) > 0 {
		s.clickDelay = (currentTick - s.lastClickTick) * 50
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
	s.lastClickTick = currentTick
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
