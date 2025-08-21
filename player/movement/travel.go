package movement

import (
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func (ctx *movementContext) travelNormal() {
	movement := ctx.mPlayer.Movement()
	newVel := movement.Vel()
	if eff, ok := ctx.mPlayer.Effects().Get(packet.EffectLevitation); ok {
		levSpeed := game.LevitationGravityMultiplier * float32(eff.Amplifier)
		newVel[1] += (levSpeed - newVel[1]) * 0.2
	} else {
		newVel[1] -= movement.Gravity()
		newVel[1] *= game.NormalGravityMultiplier
	}
	newVel[0] *= ctx.blockFriction
	newVel[2] *= ctx.blockFriction
	movement.SetVel(newVel)

	sPos, cPos := movement.Pos(), movement.Client().Pos()
	ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[travelNormal] endOfFrameVel=%v", newVel)
	ctx.mPlayer.Dbg.Notify(player.DebugModeMovementSim, true, "[travelNormal] serverPos=%v clientPos=%v, diff=%v", sPos, cPos, sPos.Sub(cPos))
}
