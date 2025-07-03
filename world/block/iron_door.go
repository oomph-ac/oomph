package block

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/model"
	"github.com/df-mc/dragonfly/server/world"
)

var hashIronDoor = block.NextHash()

func init() {
	for _, door := range allIronDoors() {
		world.RegisterBlock(door)
	}
}

// IronDoor is a variant of the door made of iron that can only be opened using redstone.
type IronDoor struct {
	Facing           cube.Direction
	Open, Top, Right bool
}

// EncodeBlock ...
func (d IronDoor) EncodeBlock() (name string, properties map[string]any) {
	return "minecraft:iron_door", map[string]any{"minecraft:cardinal_direction": d.Facing.RotateRight().String(), "door_hinge_bit": d.Right, "open_bit": d.Open, "upper_block_bit": d.Top}
}

func (d IronDoor) Hash() (uint64, uint64) {
	return hashIronDoor, uint64(d.Facing) | uint64(boolByte(d.Open))<<2 | uint64(boolByte(d.Top))<<3 | uint64(boolByte(d.Right))<<4
}

// Model ...
func (d IronDoor) Model() world.BlockModel {
	return model.Door{Facing: d.Facing, Open: d.Open, Right: d.Right}
}

// allIronDoors returns a list of all door types
func allIronDoors() (doors []world.Block) {
	for i := cube.Direction(0); i <= 3; i++ {
		doors = append(doors, IronDoor{Facing: i, Open: false, Top: false, Right: false})
		doors = append(doors, IronDoor{Facing: i, Open: false, Top: true, Right: false})
		doors = append(doors, IronDoor{Facing: i, Open: true, Top: true, Right: false})
		doors = append(doors, IronDoor{Facing: i, Open: true, Top: false, Right: false})
		doors = append(doors, IronDoor{Facing: i, Open: false, Top: false, Right: true})
		doors = append(doors, IronDoor{Facing: i, Open: false, Top: true, Right: true})
		doors = append(doors, IronDoor{Facing: i, Open: true, Top: true, Right: true})
		doors = append(doors, IronDoor{Facing: i, Open: true, Top: false, Right: true})
	}
	return
}
