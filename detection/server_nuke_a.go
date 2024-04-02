package detection

import (
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDServerNukeA = "oomph:server_nuke_a"

type ServerNukeA struct {
	BaseDetection
}

func NewServerNukeA() *ServerNukeA {
	d := &ServerNukeA{}
	d.Type = "ServerNuke"
	d.SubType = "A"

	d.Description = "Checks if a player is sending giant modal form response packet (used to lag servers)."
	d.Punishable = true // should ip ban when this flags

	d.MaxViolations = 1
	d.trustDuration = -1

	d.FailBuffer = 0
	d.MaxBuffer = 1
	return d
}

func (d *ServerNukeA) ID() string {
	return DetectionIDServerNukeA
}

func (d *ServerNukeA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	i, ok := pk.(*packet.ModalFormResponse)
	if !ok {
		return true
	}

	responseData, ok := i.ResponseData.Value()
	if !ok {
		return true
	}

	if len(responseData) > 16384 {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("len", len(responseData))
		d.Fail(p, data)
	}

	return true
}
