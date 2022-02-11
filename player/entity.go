package player

import (
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
