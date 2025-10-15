package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketE struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata
}

func New_BadPacketE(p *player.Player) *BadPacketE {
	return &BadPacketE{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer:    1,
			MaxBuffer:     1,
			MaxViolations: 1,
			TrustDuration: -1,
		},
	}
}

func (*BadPacketE) Type() string {
	return TypeBadPacket
}

func (*BadPacketE) SubType() string {
	return "E"
}

func (*BadPacketE) Description() string {
	return "Checks if the player is sending an invalid value for their MoveVector on client-side."
}

func (*BadPacketE) Punishable() bool {
	return true
}

func (d *BadPacketE) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketE) Detect(pk packet.Packet) {
	i, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return
	}

	for index := range 2 {
		if v := i.MoveVector[index]; v < -1.001 || v > 1.001 {
			d.mPlayer.FailDetection(d)
		}
	}
}
