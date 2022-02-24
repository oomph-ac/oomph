package player

import (
	"github.com/go-gl/mathgl/mgl64"
	"github.com/justtaldevelops/oomph/entity"
)

// Entity queries the player for an entity, using the runtime ID specified. The second return value is
// false if the entity is not loaded inside the player memory.
func (p *Player) Entity(rid uint64) (entity.Entity, bool) {
	p.entityMu.Lock()
	e, ok := p.entities[rid]
	p.entityMu.Unlock()
	return e, ok
}

// UpdateEntity updates an entity using the runtime ID and the provided new entity data.
func (p *Player) UpdateEntity(rid uint64, e entity.Entity) {
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

// tickEntityLocations ticks entity locations to simulate what the client would see for the
func (p *Player) tickEntityLocations() {
	for eid := range p.entities {
		e, _ := p.Entity(eid)
		if e.NewPosRotationIncrements > 0 {
			delta := e.RecievedPosition.Sub(e.LastPosition).Mul(1 / float64(e.NewPosRotationIncrements))
			e.LastPosition = e.Position
			e.Position = e.Position.Add(delta)
			e.NewPosRotationIncrements--
		}
		e.TeleportTicks++
		p.UpdateEntity(eid, e)
	}
}

// flushEntityLocations clears the queued entity location map, and sends an acknowledgement to the player
// This allows us to know when the client has received positions of other entities.
func (p *Player) flushEntityLocations() {
	queue := p.queuedEntityLocations
	p.queuedEntityLocations = make(map[uint64]mgl64.Vec3)

	p.Acknowledgement(func() {
		for rid, pos := range queue {
			if e, valid := p.Entity(rid); valid {
				e.RecievedPosition = pos
				e.NewPosRotationIncrements = 3
				p.UpdateEntity(rid, e)
			}
		}
	})
}
