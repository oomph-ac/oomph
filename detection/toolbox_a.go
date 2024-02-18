package detection

import (
	"strings"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDToolboxA = "oomph:toolbox_a"

type ToolboxA struct {
	BaseDetection
	run bool
}

func (d *ToolboxA) ID() string {
	return DetectionIDToolboxA
}

func NewToolboxA() *ToolboxA {
	d := &ToolboxA{}
	d.Type = "Toolbox"
	d.SubType = "A"

	d.Description = "Checks if the first word if the players device model is not uppercase on android."
	d.Punishable = true

	d.MaxViolations = 1
	d.trustDuration = -1

	d.FailBuffer = 0
	d.MaxBuffer = 1

	d.run = true

	return d
}

func (d *ToolboxA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	if !d.run {
		return true
	}

	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}
	d.run = false

	deviceOS := p.Conn().ClientData().DeviceOS
	deviceModel := p.Conn().ClientData().DeviceModel
	if deviceOS != 1 { // only run on android
		return true
	}
	if deviceModel == "" { // empty string if linux
		return true
	}
	if strings.Split(deviceModel, " ")[0] != strings.ToUpper(strings.Split(deviceModel, " ")[0]) { // i checked vanilla android client jni code and false positive is impossible without 3rd party software
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("DeviceModel", p.Conn().ClientData().DeviceModel)
		d.Fail(p, data)
		return false
	}
	return true
}
