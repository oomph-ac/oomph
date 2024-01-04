package detection

import (
	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/event"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/text"
)

type BaseDetection struct {
	Violations    float32
	MaxViolations float32
	Buffer        float32

	Punishable bool
	Settings   map[string]interface{}

	// trustDuration is the amount of ticks needed w/o flags before the detection trusts the player.
	trustDuration int64
	// lastFlagged is the last tick the detection was flagged.
	lastFlagged int64
}

func (d *BaseDetection) ID() string {
	panic("detection.ID() not implemented")
}

// Name returns the name of the detection, along with it's sub-type.
func (d *BaseDetection) Name() (string, string) {
	panic("detection.Name() not implemented")
}

// Description returns a description of the detection.
func (d *BaseDetection) Description() string {
	panic("detection.Description() not implemented")
}

// SetSettings sets the settings of the detection.
func (d *BaseDetection) SetSettings(settings map[string]interface{}) {
	d.Settings = settings
}

// Fail is called when the detection is triggered from adbnormal behavior.
func (d *BaseDetection) Fail(p *player.Player, maxBuffer float32, extraData *orderedmap.OrderedMap[string, any]) {
	d.Buffer = math32.Min(d.Buffer+1, maxBuffer*2)
	if d.Buffer < maxBuffer {
		return
	}

	ctx := event.C()
	p.EventHandler().OnFlagged(ctx, p, d, extraData)
	if ctx.Cancelled() {
		return
	}

	d.Violations += float32(p.ClientFrame-d.lastFlagged) / float32(d.trustDuration)
	d.lastFlagged = p.ClientFrame
	if d.Violations < d.MaxViolations {
		return
	}

	// If the detection is not punishable, we don't need to do anything.
	if !d.Punishable {
		return
	}

	ctx = event.C()
	message := text.Colourf("<red><bold>Oomph detected usage of third-party modifications.</bold></red>")
	p.EventHandler().OnPunishment(ctx, p, &message)
	if ctx.Cancelled() {
		return
	}

	p.Disconnect(message)
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
