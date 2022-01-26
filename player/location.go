package player

import (
	"github.com/justtaldevelops/oomph/entity"
)

// EntityLocation queries the player for the location of entity, using the runtime ID specified. The second return value is
// false if the entity is not loaded inside the player memory.
func (p *Player) EntityLocation(rid uint64) (entity.Location, bool) {
	p.entityMu.Lock()
	location, _ := p.entities[rid]
	p.entityMu.Unlock()
	return location, true
}

// UpdateLocation updates the location of an entity using the runtime ID and the provided new location.
func (p *Player) UpdateLocation(rid uint64, location entity.Location) {
	p.entityMu.Lock()
	defer p.entityMu.Unlock()
	p.entities[rid] = location
}
