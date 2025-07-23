package acknowledgement

import "github.com/oomph-ac/oomph/player"

type SaveTheWorld struct {
	mPlayer      *player.Player
	saveTheWorld bool
}

func NewSaveTheWorldACK(mPlayer *player.Player, save bool) *SaveTheWorld {
	return &SaveTheWorld{
		mPlayer:      mPlayer,
		saveTheWorld: save,
	}
}

func (ack *SaveTheWorld) Run() {
	if w := ack.mPlayer.World(); w != nil {
		w.SetSaveTheWorld(ack.saveTheWorld)
	}
}
