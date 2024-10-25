package acknowledgement

import "github.com/oomph-ac/oomph/player"

// UpdateGamemode is an acknowledgment that is ran when the player recieves an update to their current gamemode.
type UpdateGamemode struct {
	mPlayer  *player.Player
	gamemode int32
}

func NewUpdateGamemodeACK(p *player.Player, gamemode int32) *UpdateGamemode {
	return &UpdateGamemode{
		mPlayer:  p,
		gamemode: gamemode,
	}
}

func (ack *UpdateGamemode) Run() {
	ack.mPlayer.GameMode = ack.gamemode
}
