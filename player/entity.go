package player

import (
	"github.com/oomph-ac/oomph/entity"
)

// SearchEntity queries the player for an entity, using the runtime ID specified. The second return value is false if
// the entity is not loaded inside the player memory.
func (p *Player) SearchEntity(rid uint64) (*entity.Entity, bool) {
	if rid == p.rid {
		// We got our own runtime ID, so we can return ourself.
		return p.Entity(), true
	}

	v, ok := p.entities.Load(rid)
	if !ok {
		return nil, false
	}

	return v.(*entity.Entity), ok
}

// AddEntity creates a new entity using the runtime ID and the provided data.
func (p *Player) AddEntity(rid uint64, e *entity.Entity) {
	p.entities.Store(rid, e)
}

// RemoveEntity removes an entity from the entity map using the runtime ID
func (p *Player) RemoveEntity(rid uint64) {
	p.entities.Delete(rid)
}
