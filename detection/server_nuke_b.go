package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDServerNukeB = "oomph:server_nuke_b"

type ServerNukeB struct {
	BaseDetection
}

func NewServerNukeB() *ServerNukeB {
	d := &ServerNukeB{}
	d.Type = "ServerNuke"
	d.SubType = "B"

	d.Description = "Checks if a player is sending text packet jukebox popup (usually combined with large parameters array to lag servers)."
	d.Punishable = true // should ip ban when this flags
	d.BlockIp = true

	d.MaxViolations = 1
	d.trustDuration = -1

	d.FailBuffer = 0
	d.MaxBuffer = 1
	return d
}

func (d *ServerNukeB) ID() string {
	return DetectionIDServerNukeB
}

func (d *ServerNukeB) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	i, ok := pk.(*packet.Text)
	if !ok {
		return true
	}

	if i.TextType == packet.TextTypeJukeboxPopup {
		d.Fail(p, nil)
	}

	return true
}
