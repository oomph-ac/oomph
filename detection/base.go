package detection

import (
	"fmt"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/event"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/text"
)

type BaseDetection struct {
	Type        string
	SubType     string
	Description string

	Violations    float32
	MaxViolations float32

	Buffer     float32
	FailBuffer float32
	MaxBuffer  float32

	Punishable bool
	Settings   map[string]interface{}

	// trustDuration is the amount of ticks needed w/o flags before the detection trusts the player.
	trustDuration int64
	// lastFlagged is the last tick the detection was flagged.
	lastFlagged int64
}

// ID returns the ID of the detection.
func (d *BaseDetection) ID() string {
	panic(oerror.New("detection.ID() not implemented"))
}

// SetSettings sets the settings of the detection.
func (d *BaseDetection) SetSettings(settings map[string]interface{}) {
	d.Settings = settings
}

// Fail is called when the detection is triggered from adbnormal behavior.
func (d *BaseDetection) Fail(p *player.Player, extraData *orderedmap.OrderedMap[string, any]) {
	if extraData == nil {
		extraData = orderedmap.NewOrderedMap[string, any]()
	}
	extraData.Set("latency", fmt.Sprintf("%vms", p.Handler(handler.HandlerIDLatency).(*handler.LatencyHandler).StackLatency))

	d.Buffer = math32.Min(d.Buffer+1, d.MaxBuffer)
	if d.Buffer < d.FailBuffer {
		return
	}

	ctx := event.C()
	p.EventHandler().HandleFlag(ctx, p, d, extraData)
	if ctx.Cancelled() {
		return
	}

	if d.trustDuration != -1 {
		d.Violations += math32.Max(0, float32(d.trustDuration)-float32(p.ClientFrame-d.lastFlagged)) / float32(d.trustDuration)
	} else {
		d.Violations++
	}

	d.lastFlagged = p.ClientFrame
	if d.Violations >= 0.5 {
		p.SendRemoteEvent(player.NewFlaggedEvent(p, d.Type, d.SubType, d.Violations, OrderedMapToString(*extraData)))
		p.Log().Warnf("%s flagged %s (%s) <x%f> %s", p.Conn().IdentityData().DisplayName, d.Type, d.SubType, game.Round32(d.Violations, 2), OrderedMapToString(*extraData))
	}

	if d.Violations < d.MaxViolations {
		return
	}

	// If the detection is not punishable, we don't need to do anything.
	if !d.Punishable {
		return
	}

	ctx = event.C()
	message := text.Colourf("<red><bold>Oomph detected usage of third-party modifications.</bold></red>")
	p.EventHandler().HandlePunishment(ctx, p, d, &message)
	if ctx.Cancelled() {
		return
	}

	p.Log().Warnf("%s was removed from the server for usage of third-party modifications (%s%s).", p.Conn().IdentityData().DisplayName, d.Type, d.SubType)
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

func (d *BaseDetection) Defer() {
}
