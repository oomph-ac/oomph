package player

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/oomph/game"
)

func (p *Player) SetHeldItems(mainHand, offhand item.Stack) {}

func (p *Player) UsingItem() bool { return false }

func (p *Player) ReleaseItem() {}

func (p *Player) UseItem() {}

func (p *Player) HeldItems() (mainHand, offhand item.Stack) { return }

// As we are not running a Dragonfly server, we don't need to implement entity handles.
func (p *Player) H() *world.EntityHandle {
	return nil
}

func (p *Player) Position() mgl64.Vec3 {
	return game.Vec32To64(p.movement.Pos())
}

func (p *Player) Rotation() cube.Rotation {
	// [yaw, pitch]
	return cube.Rotation{float64(p.movement.Rotation().Z()), float64(p.movement.Rotation().X())}
}
