package detection

import (
	"fmt"

	"github.com/chewxy/math32"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type Data struct {
	Name  string
	Value any
}

type BaseDetection struct {
	Buffer float32

	Violations    float32
	MaxViolations float32

	Settings map[string]interface{}
}

func (d *BaseDetection) ID() string {
	panic("implement me")
}

// Name returns the name of the detection, along with it's sub-type.
func (d *BaseDetection) Name() (string, string) {
	panic("implement me")
}

// Description returns a description of the detection.
func (d *BaseDetection) Description() string {
	panic("implement me")
}

// Fail is called when the detection is triggered from adbnormal behavior.
func (d *BaseDetection) Fail(p *player.Player, maxBuffer float32, extraData ...Data) {
	d.Buffer = math32.Min(d.Buffer+1, maxBuffer*2)
	if d.Buffer < maxBuffer {
		return
	}

	d.Violations++
	extraDataMsg := "["
	count := len(extraData)
	for i, data := range extraData {
		extraDataMsg += fmt.Sprintf("%s: %v", data.Name, data.Value)
		if i != count-1 {
			extraDataMsg += " "
		}
	}
	extraDataMsg += "]"

	p.Message("lol kid ur bad " + extraDataMsg)
}

// Debuff...
func (d *BaseDetection) Debuff(amount float32) {
	d.Buffer = math32.Max(d.Buffer-amount, 0)
}

func (d *BaseDetection) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	return true
}

func (d *BaseDetection) HandleServerPacket(pk packet.Packet, p *player.Player) bool {
	return true
}

func (d *BaseDetection) OnTick(p *player.Player) {
}
