package block

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oomph/world/blockmodel"
)

var (
	tripWireHash     = block.NextHash()
	tripWireHookHash = block.NextHash()
)

func init() {
	for _, trip := range allTripwires() {
		world.RegisterBlock(trip)
	}
	for _, hook := range allTripWireHooks() {
		world.RegisterBlock(hook)
	}
}

type TripWire struct {
	Attached  bool
	Disarmed  bool
	Powered   bool
	Suspended bool
}

func (t TripWire) EncodeBlock() (string, map[string]any) {
	return "minecraft:trip_wire", map[string]any{
		"attached_bit":  boolByte(t.Attached),
		"disarmed_bit":  boolByte(t.Disarmed),
		"powered_bit":   boolByte(t.Powered),
		"suspended_bit": boolByte(t.Suspended),
	}
}

func (t TripWire) Hash() (uint64, uint64) {
	return tripWireHash, uint64(boolByte(t.Attached)) | uint64(boolByte(t.Disarmed))<<32 | uint64(boolByte(t.Powered))<<33 | uint64(boolByte(t.Suspended))<<34
}

func (t TripWire) Model() world.BlockModel {
	return blockmodel.NoCollisionSolid{}
}

type TripWireHook struct {
	Attached  bool
	Powered   bool // 3rd
	Direction cube.Direction
}

func (h TripWireHook) EncodeBlock() (string, map[string]any) {
	return "minecraft:tripwire_hook", map[string]any{
		"attached_bit": boolByte(h.Attached),
		"direction":    int32(h.Direction),
		"powered_bit":  boolByte(h.Powered),
	}
}

func (h TripWireHook) Hash() (uint64, uint64) {
	return tripWireHookHash, uint64(boolByte(h.Attached)) | uint64(boolByte(h.Powered))<<32 | uint64(h.Direction)<<33
}

func (h TripWireHook) Model() world.BlockModel {
	return blockmodel.NoCollisionSolid{}
}

func allTripWireHooks() (blocks []world.Block) {
	for _, dir := range cube.Directions() {
		blocks = append(blocks, TripWireHook{Direction: dir, Attached: false, Powered: false})
		blocks = append(blocks, TripWireHook{Direction: dir, Attached: false, Powered: true})
		blocks = append(blocks, TripWireHook{Direction: dir, Attached: true, Powered: false})
		blocks = append(blocks, TripWireHook{Direction: dir, Attached: true, Powered: true})
	}
	return
}

func allTripwires() (blocks []world.Block) {
	for a := byte(0); a <= 1; a++ {
		for b := byte(0); b <= 1; b++ {
			for c := byte(0); c <= 1; c++ {
				for d := byte(0); d <= 1; d++ {
					blocks = append(blocks, TripWire{
						Attached:  a == 1,
						Disarmed:  b == 1,
						Powered:   c == 1,
						Suspended: d == 1,
					})
				}
			}
		}
	}
	return
}
