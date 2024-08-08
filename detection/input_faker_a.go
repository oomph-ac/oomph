package detection

import (
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDInputFakerA = "oomph:input_faker_a"

type InputFakerA struct {
	BaseDetection
}

func NewInputFakerA() *InputFakerA {
	d := &InputFakerA{}
	d.Type = "InputFaker"
	d.SubType = "A"

	d.Description = "Checks if the player is spoofing touch input mode. This is experimental."
	d.Punishable = false

	d.MaxViolations = 5
	d.trustDuration = -1

	d.FailBuffer = 0
	d.MaxBuffer = 1
	return d
}

func (d *InputFakerA) ID() string {
	return DetectionIDInputFakerA
}

func (d *InputFakerA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	playerAuthInput, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	if playerAuthInput.InputMode != packet.InputModeTouch {
		return true
	}

	if !utils.HasFlag(playerAuthInput.InputData, packet.InputFlagMissedSwing) {
		return true
	}

	c := p.Handler(handler.HandlerIDCombat).(*handler.CombatHandler)

	tickDiff := p.ClientFrame - c.LastSwingTick
	attackDiff := p.ClientFrame - c.LastAttackTick

	if tickDiff <= 1 && attackDiff > 4 {
		data := orderedmap.NewOrderedMap[string, any]()
		data.Set("tick_diff", tickDiff)
		data.Set("attack_diff", attackDiff)
		data.Set("current_tick", p.ClientFrame)
		data.Set("last_tick", c.LastSwingTick)
		d.Fail(p, data)
	}

	return true
}
