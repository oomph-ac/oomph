package player

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oomph/utils"
)

// tickNearbyBlocks is called once every client tick to update block ticks.
func (p *Player) tickNearbyBlocks() {
	var liquids, climbables uint32
	for _, v := range utils.BlocksNearby(p.AABB().Translate(p.Position()).Grow(0.2), p.World()) {
		// TODO: Also check for vines and cobwebs when added in DF.
		switch v.(type) {
		case world.Liquid:
			liquids++
		case block.Ladder:
			climbables++
		}
	}

	p.spawnTicks++
	p.cobwebTicks++
	p.liquidTicks++
	p.motionTicks++
	p.climbableTicks++
	if p.dead {
		p.spawnTicks = 0
	}
	if liquids > 0 {
		p.liquidTicks = 0
	}
	if climbables > 0 {
		p.climbableTicks = 0
	}
}
