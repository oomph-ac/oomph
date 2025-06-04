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
}

func NewUpdateBlockACK(p *player.Player, pos df_cube.Pos, b world.Block) *UpdateBlock {
	return &UpdateBlock{mPlayer: p, pos: pos, b: b}
}

func (ack *UpdateBlock) Run() {
	ack.mPlayer.World().SetBlock(ack.pos, ack.b, nil)
}
