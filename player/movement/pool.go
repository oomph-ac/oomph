package movement

import (
	"sync"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/player"
)

var ctxPool = sync.Pool{
	New: func() any {
		return &movementContext{}
	},
}

func newCtx(p *player.Player) *movementContext {
	ctx := ctxPool.Get().(*movementContext)
	ctx.mPlayer = p
	return ctx
}

func putCtx(ctx *movementContext) {
	ctx.reset()
	ctxPool.Put(ctx)
}

func (ctx *movementContext) reset() {
	ctx.mPlayer = nil
	ctx.preCollideVel = mgl32.Vec3{}
	ctx.blockUnder = nil
	ctx.blockFriction = 0
	ctx.moveRelativeSpeed = 0
	ctx.hasBlockSlowdown = false
	ctx.useSlideOffset = false
	ctx.clientJumpPrevented = false
}
