package acknowledgement

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

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

// UpdateAbilities is an acknowledgment that is ran when the member player has it's abilities updated.
type UpdateAbilities struct {
	mPlayer *player.Player
	data    protocol.AbilityData
}

func NewUpdateAbilitiesACK(p *player.Player, data protocol.AbilityData) *UpdateAbilities {
	return &UpdateAbilities{
		mPlayer: p,
		data:    data,
	}
}

func (ack *UpdateAbilities) Run() {
	for _, l := range ack.data.Layers {
		flying := utils.HasFlag(uint64(l.Values), protocol.AbilityMayFly) || utils.HasFlag(uint64(l.Values), protocol.AbilityFlying)
		ack.mPlayer.Movement().SetFlying(flying)
		ack.mPlayer.Movement().SetNoClip(utils.HasFlag(uint64(l.Values), protocol.AbilityNoClip))

		if ack.mPlayer.Movement().Client().ToggledFly() {
			ack.mPlayer.Movement().SetTrustFlyStatus(flying)
			ack.mPlayer.Movement().Client().SetToggledFly(false)
		}
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
		if attribute.Name == "minecraft:movement" {
			ack.mPlayer.Movement().SetMovementSpeed(attribute.Value)
			ack.mPlayer.Movement().SetDefaultMovementSpeed(attribute.Default)
		} else if attribute.Name == "minecraft:health" {
			ack.mPlayer.Alive = attribute.Value > 0
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
		newSize = newSize.Mul(scale.(float32))
	}

	// Set the new size of the player.
	ack.mPlayer.Movement().SetSize(newSize)

	if f, ok := ack.metadata[entity.DataKeyFlags]; ok {
		flags := f.(int64)
		ack.mPlayer.Movement().SetImmobile(utils.HasDataFlag(entity.DataFlagImmobile, flags))
		ack.mPlayer.Movement().SetSprinting(utils.HasDataFlag(entity.DataFlagSprinting, flags))
	}
}

// Knockback is an acknowledgment that is ran whenever the player receives a knockback update from the server.
type Knockback struct {
	mPlayer   *player.Player
	knockback mgl32.Vec3
}

func NewKnockbackACK(p *player.Player, knockback mgl32.Vec3) *Knockback {
	return &Knockback{mPlayer: p, knockback: knockback}
}

func (ack *Knockback) Run() {
	ack.mPlayer.Movement().SetKnockback(ack.knockback)
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
