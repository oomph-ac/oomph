package acknowledgement

import (
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oomph/player"
)

// UpdateBlock is an acknowledgment that is ran when a single block is updated in the world of the player.
// TODO: Improve this acknowledgment to be more efficient (store multiple block placements). Since oomph is able to
// control when packets are flushed, we should in theory be able to store all block updates into one packet, which would
// let us run only one world transaction on the Dragonfly world.
type UpdateBlock struct {
	mPlayer *player.Player
	b       world.Block
	pos     df_cube.Pos

	expiresIn int64
	valid     bool
}

func NewUpdateBlockACK(p *player.Player, pos df_cube.Pos, b world.Block, expiresIn int64) *UpdateBlock {
	return &UpdateBlock{mPlayer: p, pos: pos, b: b, valid: true, expiresIn: expiresIn}
}

func (ack *UpdateBlock) Run() {
	if !ack.valid {
		return
	}
	ack.mPlayer.World().SetBlock(ack.pos, ack.b, nil)
	ack.valid = false
}

func (ack *UpdateBlock) Tick() {
	if !ack.valid {
		return
	}
	ack.expiresIn--
	if ack.expiresIn <= 0 {
		ack.mPlayer.Dbg.Notify(player.DebugModeLatency, true, "updateBlock ack for %T at %v lag-compensation expired", ack.b, ack.pos)
		ack.Run()
	}
}

func (ack *UpdateBlock) Invalidate() {
	ack.valid = false
	ack.mPlayer.Dbg.Notify(player.DebugModeChunks, true, "updateBlock ack for %T at %v is invalidated", ack.b, ack.pos)
}
