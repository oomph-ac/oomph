package acknowledgement

import (
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// PlayerEffects is an acknowledgment that is ran when the player receives an effects update from the server.
type PlayerEffects struct {
	mPlayer    *player.Player
	effectType int32
	amplifier  int32
	duration   int32
	operation  byte
}

func NewPlayerEffectsACK(p *player.Player, effectType, amplifier, duration int32, operation byte) *PlayerEffects {
	return &PlayerEffects{
		mPlayer:    p,
		effectType: effectType,
		amplifier:  amplifier,
		duration:   duration,
		operation:  operation,
	}
}

func (ack *PlayerEffects) Run() {
	if ack.operation == packet.MobEffectAdd || ack.operation == packet.MobEffectModify {
		t, ok := effect.ByID(int(ack.effectType))
		if !ok {
			return
		}

		if _, ok := t.(effect.LastingType); ok {
			ack.mPlayer.Effects().Add(ack.effectType, player.NewEffect(ack.amplifier+1, ack.duration))
		}
	} else {
		ack.mPlayer.Effects().Remove(ack.effectType)
	}
}

// TeleportPlayer is an acknowledgment that is ran when the player receives a teleport from the server.
type TeleportPlayer struct {
	mPlayer *player.Player

	pos                mgl32.Vec3
	onGround, smoothed bool
}

func NewTeleportPlayerACK(p *player.Player, pos mgl32.Vec3, onGround, smoothed bool) *TeleportPlayer {
	return &TeleportPlayer{
		mPlayer:  p,
		pos:      pos,
		onGround: onGround,
		smoothed: smoothed,
	}
}

func (ack *TeleportPlayer) Run() {
	ack.mPlayer.Movement().Teleport(ack.pos, ack.onGround, ack.smoothed)
	ack.mPlayer.Movement().RemovePendingTeleport()
}

type UpdateAbilities struct {
	mPlayer *player.Player

	mayFly bool
	flying bool
	noClip bool
}

func NewUpdateAbilitiesACK(p *player.Player, data protocol.AbilityData) *UpdateAbilities {
	// Determine if the player has the ability to fly and if they are currently flying.
	ack := &UpdateAbilities{mPlayer: p}
	for _, l := range data.Layers {
		layerSetAbilities := uint64(l.Abilities)
		layerValues := uint64(l.Values)
		if utils.HasFlag(layerSetAbilities, protocol.AbilityMayFly) {
			ack.mayFly = ack.mayFly || utils.HasFlag(layerValues, protocol.AbilityMayFly)
		}
		if utils.HasFlag(layerSetAbilities, protocol.AbilityFlying) {
			ack.flying = ack.flying || utils.HasFlag(layerValues, protocol.AbilityFlying)
		}
		if utils.HasFlag(layerSetAbilities, protocol.AbilityNoClip) {
			ack.noClip = ack.noClip || utils.HasFlag(layerValues, protocol.AbilityNoClip)
		}
	}
	return ack
}

func (ack *UpdateAbilities) Run() {
	ack.mPlayer.Movement().SetMayFly(ack.mayFly)
	ack.mPlayer.Movement().SetFlying(ack.flying)
	ack.mPlayer.Movement().SetNoClip(ack.noClip)

	if ack.mPlayer.Movement().Client().ToggledFly() {
		ack.mPlayer.Movement().SetTrustFlyStatus(ack.flying || ack.mayFly)
		ack.mPlayer.Movement().Client().SetToggledFly(false)
	}
}

// UpdateAttributes is an acknowledgment that is ran when the member player has it's attributes updated.
type UpdateAttributes struct {
	mPlayer    *player.Player
	attributes []protocol.Attribute
}

func NewUpdateAttributesACK(p *player.Player, attributes []protocol.Attribute) *UpdateAttributes {
	ack := &UpdateAttributes{mPlayer: p, attributes: make([]protocol.Attribute, len(attributes))}
	copy(ack.attributes, attributes)

	return ack
}

func (ack *UpdateAttributes) Run() {
	for _, attribute := range ack.attributes {
		switch attribute.Name {
		case "minecraft:movement":
			ack.mPlayer.Movement().SetMovementSpeed(attribute.Value)
			ack.mPlayer.Movement().SetDefaultMovementSpeed(attribute.Default)
		case "minecraft:health":
			ack.mPlayer.Alive = attribute.Value > 0
		case "minecraft:player.hunger":
			ack.mPlayer.IsHungry = attribute.Value < 20
		}
	}
}

// PlayerUpdateActorData is an acknowledgment that is ran when the member player receives an update for it's actor data
type PlayerUpdateActorData struct {
	mPlayer  *player.Player
	metadata map[uint32]interface{}
}

func NewUpdateActorData(p *player.Player, metadata map[uint32]interface{}) *PlayerUpdateActorData {
	return &PlayerUpdateActorData{
		mPlayer:  p,
		metadata: metadata,
	}
}

func (ack *PlayerUpdateActorData) Run() {
	newSize := ack.mPlayer.Movement().Size()
	if width, widthExists := ack.metadata[entity.DataKeyBoundingBoxWidth]; widthExists {
		newSize[0] = width.(float32)
	}
	if height, heightExists := ack.metadata[entity.DataKeyBoundingBoxHeight]; heightExists {
		newSize[1] = height.(float32)
	}
	if scale, scaleExists := ack.metadata[entity.DataKeyScale]; scaleExists {
		newSize[2] = scale.(float32)
	}

	// Set the new size of the player.
	ack.mPlayer.Movement().SetSize(newSize)

	if f, ok := ack.metadata[entity.DataKeyFlags]; ok {
		flags := f.(int64)
		ack.mPlayer.Movement().SetImmobile(utils.HasDataFlag(entity.DataFlagImmobile, flags))
		ack.mPlayer.Movement().SetServerSprint(utils.HasDataFlag(entity.DataFlagSprinting, flags))

		if !utils.HasDataFlag(entity.DataFlagAction, flags) && ack.mPlayer.StartUseConsumableTick != 0 {
			ack.mPlayer.StartUseConsumableTick = 0
		}
	}
}

// Knockback is an acknowledgment that is ran whenever the player receives a knockback update from the server.
type Knockback struct {
	mPlayer   *player.Player
	knockback mgl32.Vec3

	expiresIn int64
	ran       bool
}

func NewKnockbackACK(p *player.Player, knockback mgl32.Vec3, expiresIn int64) *Knockback {
	return &Knockback{mPlayer: p, knockback: knockback, expiresIn: expiresIn}
}

func (ack *Knockback) Run() {
	if !ack.ran {
		ack.mPlayer.Movement().SetKnockback(ack.knockback)
		ack.ran = true
	}
}

func (ack *Knockback) Tick() {
	if ack.ran {
		return
	}
	ack.expiresIn--
	if ack.expiresIn <= 0 {
		ack.mPlayer.Dbg.Notify(player.DebugModeLatency, true, "knockback ack for %T (%v) lag-compensation expired", ack.knockback, game.RoundVec32(ack.knockback, 4))
		ack.Run()
	}
}

// MovementCorrection is an acknowledgment that is ran whenever the player receives a movement correction from the server.
type MovementCorrection struct {
	mPlayer  *player.Player
	setField bool
}

func NewMovementCorrectionACK(p *player.Player) *MovementCorrection {
	return &MovementCorrection{
		mPlayer:  p,
		setField: !p.PendingCorrectionACK,
	}
}

func (ack *MovementCorrection) Run() {
	ack.mPlayer.Movement().RemovePendingCorrection()
	if ack.setField {
		ack.mPlayer.PendingCorrectionACK = false
	}
}
