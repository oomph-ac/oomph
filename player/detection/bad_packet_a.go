package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketA struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	prevFrame uint64
}

func New_BadPacketA(p *player.Player) *BadPacketA {
	return &BadPacketA{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer: 1,
			MaxBuffer:  1,

			MaxViolations: 1,
		},
	}
}

func (*BadPacketA) Type() string {
	return TypeBadPacket
}

func (*BadPacketA) SubType() string {
	return "A"
}

func (*BadPacketA) Description() string {
	return "Checks if a player's simulation frame is valid."
}

func (*BadPacketA) Punishable() bool {
	return true
}

func (d *BadPacketA) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketA) Detect(pk packet.Packet) {
	if i, ok := pk.(*packet.PlayerAuthInput); ok {
		if d.prevFrame != 0 && i.Tick == 0 {
			d.mPlayer.FailDetection(d)
		}
		d.prevFrame = i.Tick
	}
}
