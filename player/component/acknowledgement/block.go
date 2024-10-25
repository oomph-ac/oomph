package acknowledgement

import (
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/oomph/player"
)

// UpdateBlock is an acknowledgment that is ran when a single block is updated
// in the world of the player.
// DEPRECATED: Not needed since block updates are done instantly.
type UpdateBlock struct {
	mPlayer *player.Player
	b       world.Block
	pos     df_cube.Pos
}

func NewUpdateBlockACK(p *player.Player, pos df_cube.Pos, b world.Block) *UpdateBlock {
	return &UpdateBlock{mPlayer: p, pos: pos, b: b}
}

func (ack *UpdateBlock) Run() {
	ack.mPlayer.World.SetBlock(ack.pos, ack.b, nil)
}
