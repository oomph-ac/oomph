package detection

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/event"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
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
	Settings   *orderedmap.OrderedMap[string, any]

	// trustDuration is the amount of ticks needed w/o flags before the detection trusts the player.
	trustDuration int64
	// lastFlagged is the last tick the detection was flagged.
	lastFlagged int64
}

func Encode(buf *bytes.Buffer, d player.Handler) {
	utils.WriteLInt32(buf, int32(len(d.ID())))
	buf.WriteString(d.ID())

	enc, err := json.Marshal(d)
	if err != nil {
		panic(err)
	}

	utils.WriteLInt32(buf, int32(len(enc)))
	buf.Write(enc)
}

func Decode(buf *bytes.Buffer) player.Handler {
	len := utils.LInt32(buf.Next(4))
	id := string(buf.Next(int(len)))

	var t player.Handler
	switch id {
	case DetectionIDAutoClickerA:
		t = &AutoClickerA{}
	case DetectionIDAutoClickerB:
		t = &AutoClickerB{}
	case DetectionIDAutoClickerC:
		t = &AutoClickerC{}
	case DetectionIDBadPacketA:
		t = &BadPacketA{}
	case DetectionIDBadPacketB:
		t = &BadPacketB{}
	case DetectionIDBadPacketC:
		t = &BadPacketC{}
	case DetectionIDEditionFakerA:
		t = &EditionFakerA{}
	case DetectionIDEditionFakerB:
		t = &EditionFakerB{}
	case DetectionIDHitboxA:
		t = &HitboxA{}
	case DetectionIDKillAuraA:
		t = &KillAuraA{}
	case DetectionIDFlyA:
		t = &FlyA{}
	case DetectionIDMotionA:
		t = &MotionA{}
	case DetectionIDMotionB:
		t = &MotionB{}
	case DetectionIDMotionC:
		t = &MotionC{}
	case DetectionIDSpeedA:
		t = &SpeedA{}
	case DetectionIDReachA:
		t = &ReachA{}
	case DetectionIDReachB:
		t = &ReachB{}
	case DetectionIDServerNukeA:
		t = &ServerNukeA{}
	case DetectionIDServerNukeB:
		t = &ServerNukeB{}
	case DetectionIDTimerA:
		t = &TimerA{}
	case DetectionIDToolboxA:
		t = &ToolboxA{}
	case DetectionIDVelocityA:
		t = &VelocityA{}
	case DetectionIDVelocityB:
		t = &VelocityB{}
	case DetectionIDScaffoldA:
		t = &ScaffoldA{}
	default:
		panic(oerror.New("unknown detection ID: %s", id))
	}

	len = utils.LInt32(buf.Next(4))
	err := json.Unmarshal(buf.Next(int(len)), t)
	if err != nil {
		panic(err)
	}

	return t
}

// ID returns the ID of the detection.
func (d *BaseDetection) ID() string {
	panic(oerror.New("detection.ID() not implemented"))
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

	oldVl := d.Violations
	if d.trustDuration > 0 {
		d.Violations += math32.Max(0, float32(d.trustDuration)-float32(p.ClientFrame-d.lastFlagged)) / float32(d.trustDuration)
	} else {
		d.Violations++
	}

	ctx := event.C()
	p.EventHandler().HandleFlag(ctx, p, d, extraData)
	if ctx.Cancelled() {
		d.Violations = oldVl
		return
	}

	d.lastFlagged = p.ClientFrame
	if d.Violations >= 0.5 {
		p.SendRemoteEvent(player.NewFlaggedEvent(p, d.Type, d.SubType, d.Violations, OrderedMapToString(*extraData)))
		p.Log().Warnf("%s flagged %s (%s) <x%f> %s", p.IdentityDat.DisplayName, d.Type, d.SubType, game.Round32(d.Violations, 2), OrderedMapToString(*extraData))
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

	p.Log().Warnf("%s was removed from the server for usage of third-party modifications (%s%s).", p.IdentityDat.DisplayName, d.Type, d.SubType)
	p.Disconnect(message)
	p.Close()
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
