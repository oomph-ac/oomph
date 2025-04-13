package block

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oomph/world/blockmodel"
)

var saplingHash = block.NextHash()

func init() {
	woodTypes := []block.WoodType{
		block.OakWood(),
		block.SpruceWood(),
		block.BirchWood(),
		block.JungleWood(),
		block.AcaciaWood(),
		block.DarkOakWood(),
	}

	for _, woodType := range woodTypes {
		for age := uint8(0); age < 2; age++ {
			world.RegisterBlock(Sapling{Wood: woodType, Age: age})
		}
	}
}

type Sapling struct {
	Wood block.WoodType
	Age  uint8
}

func (s Sapling) EncodeBlock() (string, map[string]any) {
	return "minecraft:" + s.Wood.String() + "_sapling", map[string]any{"age_bit": s.Age}
}

func (s Sapling) Hash() (uint64, uint64) {
	return saplingHash, uint64(s.Wood.Uint8()) | uint64(s.Age)<<32
}

func (s Sapling) Model() world.BlockModel {
	return blockmodel.NoCollisionNotSolid{}
}
