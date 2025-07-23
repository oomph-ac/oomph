package acknowledgement

import "github.com/oomph-ac/oomph/player"

type SaveTheWorld struct {
	mPlayer  *player.Player
	stwTicks uint16
}

func NewSaveTheWorldACK(mPlayer *player.Player, stwTicks uint16) *SaveTheWorld {
	return &SaveTheWorld{
		mPlayer:  mPlayer,
		stwTicks: stwTicks,
	}
}

func (ack *SaveTheWorld) Run() {
	if w := ack.mPlayer.World(); w != nil {
		w.SetSTWTicks(ack.stwTicks)
	}
}
