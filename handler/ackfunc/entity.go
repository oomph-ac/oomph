package ackfunc

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
)

// OPTS: uint64, int64, mgl32.Vec3, bool,
func UpdateEntityPosition(p *player.Player, opts ...interface{}) {
	p.Handler(handler.HandlerIDEntities).(*handler.EntitiesHandler).MoveEntity(
		opts[0].(uint64),     // RuntimeID
		opts[1].(int64),      // Tick
		opts[2].(mgl32.Vec3), // Pos
		opts[3].(bool),       // Teleport
	)
}
