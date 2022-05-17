package player

import (
	"math"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/utils"
)

// SearchEntity queries the player for an entity, using the runtime ID specified. The second return value is false if
// the entity is not loaded inside the player memory.
func (p *Player) SearchEntity(rid uint64) (*entity.Entity, bool) {
	if rid == p.rid {
		// We got our own runtime ID, so we can return ourself.
		return p.Entity(), true
	}
	p.entityMu.Lock()
	e, ok := p.entities[rid]
	p.entityMu.Unlock()
	return e, ok
}

// AddEntity creates a new entity using the runtime ID and the provided data.
func (p *Player) AddEntity(rid uint64, e *entity.Entity) {
	p.entityMu.Lock()
	defer p.entityMu.Unlock()
	p.entities[rid] = e
}

// RemoveEntity removes an entity from the entity map using the runtime ID
func (p *Player) RemoveEntity(rid uint64) {
	p.entityMu.Lock()
	delete(p.entities, rid)
	p.entityMu.Unlock()
}

// tickEntityLocations ticks entity locations to simulate what the client would see.
func (p *Player) tickEntityLocations() {
	for _, e := range p.entities {
		if increments := e.NewLocationIncrements(); increments > 0 {
			delta := e.ReceivedPosition().Sub(e.LastPosition()).Mul(1 / float64(e.NewLocationIncrements()))
			if !e.Player() {
				interp := e.InterpolatedMotion()
				fric := 0.98
				if e.OnGround() {
					fric *= 0.6
				}
				if math.Abs(interp[0]) > 0.005 || math.Abs(interp[1]) > 0.005 || math.Abs(interp[2]) > 0.005 {
					sub := interp.Mul(fric)
					delta = delta.Sub(sub)
					e.ClientInterpolation(sub)
				}
				// This is actually an INFERENCE on how the SetActor interpolation works
				// TODO: Abuse Tal to find out how the SetActorMotion interpolation works for non-player entities
			}
			pos := e.Position().Add(delta)
			e.Move(pos, false)
			e.DecrementNewLocationIncrements()
		}
		e.IncrementTeleportationTicks()
	}
}

// flushEntityLocations clears the queued entity location map, and sends an acknowledgement to the player
// This allows us to know when the client has received positions of other entities.
func (p *Player) flushEntityLocations() {
	p.queueMu.Lock()
	defer p.queueMu.Unlock()

	posQueue := p.queuedEntityLocations
	p.queuedEntityLocations = make(map[uint64]utils.LocationData)
	interpolationQueue := p.queuedEntityMotionInterpolations
	p.queuedEntityMotionInterpolations = make(map[uint64]mgl64.Vec3)

	p.Acknowledgement(func() {
		for rid, dat := range posQueue {
			if e, valid := p.SearchEntity(rid); valid {
				e.UpdateReceivedPosition(dat.Position, dat.OnGround, e.Player())
				e.ResetNewLocationIncrements()
			}
		}
		for rid, motion := range interpolationQueue {
			if e, valid := p.SearchEntity(rid); valid {
				e.ClientInterpolation(motion)
			}
		}
	}, true)
}
