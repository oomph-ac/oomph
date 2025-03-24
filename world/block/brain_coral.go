package block

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oomph/world/blockmodel"
)

var (
	brainCoralHash     = block.NextHash()
	deadBrainCoralHash = block.NextHash()
)

func init() {
	world.RegisterBlock(BrainCoralFan{Direction: cube.FaceUp})
	world.RegisterBlock(BrainCoralFan{Direction: cube.FaceDown})
	world.RegisterBlock(DeadBrainCoralFan{Direction: cube.FaceUp})
	world.RegisterBlock(DeadBrainCoralFan{Direction: cube.FaceDown})
}

type BrainCoralFan struct {
	Direction cube.Face
}

func (b BrainCoralFan) EncodeBlock() (string, map[string]interface{}) {
	return "minecraft:brain_coral_fan", map[string]interface{}{"coral_fan_direction": int32(b.Direction)}
}

func (b BrainCoralFan) Hash() (uint64, uint64) {
	return brainCoralHash, uint64(b.Direction) << 32
}

func (b BrainCoralFan) Model() world.BlockModel {
	return blockmodel.NoCollisionNotSolid{}
}

type DeadBrainCoralFan struct {
	Direction cube.Face
}

func (b DeadBrainCoralFan) EncodeBlock() (string, map[string]interface{}) {
	return "minecraft:dead_brain_coral_fan", map[string]interface{}{"coral_fan_direction": int32(b.Direction)}
}

func (b DeadBrainCoralFan) Hash() (uint64, uint64) {
	return deadBrainCoralHash, uint64(b.Direction) << 32
}

func (b DeadBrainCoralFan) Model() world.BlockModel {
	return blockmodel.NoCollisionNotSolid{}
}
