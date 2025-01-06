package detection

import (
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type BadPacketB struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	last, tick int
}

func New_BadPacketB(p *player.Player) *BadPacketB {
	return &BadPacketB{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer: 1,
			MaxBuffer:  1,

			MaxViolations: 1,
		},
	}
}

func (*BadPacketB) Type() string {
	return TYPE_BAD_PACKET
}

func (*BadPacketB) SubType() string {
	return "B"
}

func (*BadPacketB) Description() string {
	return "Checks if a player is consistently sending MovePlayer packets rather than PlayerAuthInput."
}

func (*BadPacketB) Punishable() bool {
	return true
}

func (d *BadPacketB) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *BadPacketB) Detect(pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.MovePlayer:
		speed := d.tick - d.last
		if speed < 2 && !d.mPlayer.Movement().Immobile() && d.mPlayer.World.GetChunk(protocol.ChunkPos{
			int32(pk.Position.X()) >> 4,
			int32(pk.Position.Z()) >> 4,
		}) != nil {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("speed", speed)
			d.mPlayer.FailDetection(d, data)
		}

		d.last = d.tick
	case *packet.PlayerAuthInput:
		d.tick++
	}
}
