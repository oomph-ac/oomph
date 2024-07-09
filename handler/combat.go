package handler

import (
	"bytes"

	"github.com/ethaniccc/float32-cube/cube/trace"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDCombat = "oomph:combat"

const (
	CombatPhaseNone byte = iota
	CombatPhaseTransaction
	CombatPhaseTicked
)

type CombatHandler struct {
	Phase          byte
	TargetedEntity *entity.Entity

	InterpolationStep float32
	AttackOffset      float32

	StartAttackPos mgl32.Vec3
	EndAttackPos   mgl32.Vec3

	StartEntityPos mgl32.Vec3
	EndEntityPos   mgl32.Vec3

	NonRaycastResults []float32
	RaycastResults    []float32

	LastSwingTick int64

	Clicking      bool
	Clicks        []int64
	ClickDelay    int64
	LastClickTick int64
	CPS           int
}

func NewCombatHandler() *CombatHandler {
	return &CombatHandler{
		InterpolationStep: 1 / 10.0,
	}
}

func DecodeCombatHandler(buf *bytes.Buffer) CombatHandler {
	h := CombatHandler{}
	h.Phase, _ = buf.ReadByte()
	h.InterpolationStep = utils.LFloat32(buf.Next(4))
	h.AttackOffset = utils.LFloat32(buf.Next(4))
	h.StartAttackPos = utils.ReadVec32(buf.Next(12))
	h.EndAttackPos = utils.ReadVec32(buf.Next(12))
	h.StartEntityPos = utils.ReadVec32(buf.Next(12))
	h.EndEntityPos = utils.ReadVec32(buf.Next(12))
	nrCount := utils.LInt32(buf.Next(4))
	h.NonRaycastResults = make([]float32, nrCount)
	for i := int32(0); i < nrCount; i++ {
		h.NonRaycastResults[i] = utils.LFloat32(buf.Next(4))
	}
	rCount := utils.LInt32(buf.Next(4))
	h.RaycastResults = make([]float32, rCount)
	for i := int32(0); i < rCount; i++ {
		h.RaycastResults[i] = utils.LFloat32(buf.Next(4))
	}
	h.LastSwingTick = utils.LInt64(buf.Next(8))
	h.Clicking = utils.Bool(buf.Next(1))
	cCount := utils.LInt32(buf.Next(4))
	h.Clicks = make([]int64, cCount)
	for i := int32(0); i < cCount; i++ {
		h.Clicks[i] = utils.LInt64(buf.Next(8))
	}
	h.ClickDelay = utils.LInt64(buf.Next(8))
	h.LastClickTick = utils.LInt64(buf.Next(8))
	h.CPS = int(utils.LInt32(buf.Next(4)))

	if buf.Len() != 0 {
		panic(oerror.New("unexpected %d bytes left in buffer", buf.Len()))
	}
	return h
}

func (h *CombatHandler) Encode(buf *bytes.Buffer) {
	buf.WriteByte(h.Phase)
	utils.WriteLFloat32(buf, h.InterpolationStep)
	utils.WriteLFloat32(buf, h.AttackOffset)
	utils.WriteVec32(buf, h.StartAttackPos)
	utils.WriteVec32(buf, h.EndAttackPos)
	utils.WriteVec32(buf, h.StartEntityPos)
	utils.WriteVec32(buf, h.EndEntityPos)
	utils.WriteLInt32(buf, int32(len(h.NonRaycastResults)))
	for _, result := range h.NonRaycastResults {
		utils.WriteLFloat32(buf, result)
	}
	utils.WriteLInt32(buf, int32(len(h.RaycastResults)))
	for _, result := range h.RaycastResults {
		utils.WriteLFloat32(buf, result)
	}
	utils.WriteLInt64(buf, h.LastSwingTick)
	utils.WriteBool(buf, h.Clicking)
	utils.WriteLInt32(buf, int32(len(h.Clicks)))
	for _, click := range h.Clicks {
		utils.WriteLInt64(buf, click)
	}
	utils.WriteLInt64(buf, h.ClickDelay)
	utils.WriteLInt64(buf, h.LastClickTick)
	utils.WriteLInt32(buf, int32(h.CPS))
}

func (h *CombatHandler) ID() string {
	return HandlerIDCombat
}

func (h *CombatHandler) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	switch pk := pk.(type) {
	case *packet.InventoryTransaction:
		if h.Phase != CombatPhaseNone {
			return true
		}

		dat, ok := pk.TransactionData.(*protocol.UseItemOnEntityTransactionData)
		if !ok {
			return true
		}

		if dat.ActionType != protocol.UseItemOnEntityActionAttack {
			return true
		}

		h.click(p)

		entity := p.Handler(HandlerIDEntities).(*EntitiesHandler).Find(dat.TargetEntityRuntimeID)
		if entity == nil {
			return true
		}

		if entity.TicksSinceTeleport <= 20 {
			return true
		}

		mDat := p.Handler(HandlerIDMovement).(*MovementHandler)
		if mDat.TicksSinceTeleport <= 20 {
			return true
		}

		h.AttackOffset = float32(1.62)
		if mDat.Sneaking {
			h.AttackOffset = 1.54
		}

		h.Phase = CombatPhaseTransaction
		h.TargetedEntity = entity

		h.StartAttackPos = mDat.PrevClientPosition.Add(mgl32.Vec3{0, h.AttackOffset})
		h.EndAttackPos = mDat.ClientPosition.Add(mgl32.Vec3{0, h.AttackOffset})

		h.StartEntityPos = entity.PrevPosition
		h.EndEntityPos = entity.Position
	case *packet.PlayerAuthInput:
		if p.Version >= player.GameVersion1_20_10 && utils.HasFlag(pk.InputData, packet.InputFlagMissedSwing) {
			h.click(p)
		}

		if h.Phase != CombatPhaseTransaction {
			return true
		}
		h.Phase = CombatPhaseTicked

		// The entity may have already been removed before we are able to do anything with it.
		if h.TargetedEntity == nil {
			h.Phase = CombatPhaseNone
			return true
		}

		// Ignore touch input, as they are able to interact with entities without actually looking at them.
		if pk.InputMode == packet.InputModeTouch {
			return true
		}

		h.calculateNonRaycastResults()
		h.calculateRaycastResults(p)
	case *packet.Animate:
		h.LastSwingTick = p.ClientFrame
	case *packet.LevelSoundEvent:
		if p.Version < player.GameVersion1_20_10 && pk.SoundType == packet.SoundEventAttackNoDamage {
			h.click(p)
		}
	}

	return true
}

func (h *CombatHandler) HandleServerPacket(pk packet.Packet, p *player.Player) bool {
	return true
}

func (*CombatHandler) OnTick(p *player.Player) {
}

func (h *CombatHandler) Defer() {
	if h.Phase == CombatPhaseTicked {
		h.Phase = CombatPhaseNone
	}

	h.Clicking = false
}

func (h *CombatHandler) calculateNonRaycastResults() {
	attackPosDelta := h.EndAttackPos.Sub(h.StartAttackPos)
	entityPosDelta := h.EndEntityPos.Sub(h.StartEntityPos)
	h.NonRaycastResults = make([]float32, 0, 20)

	for partialTicks := float32(0); partialTicks <= 1; partialTicks += h.InterpolationStep {
		attackPos := h.StartAttackPos.Add(attackPosDelta.Mul(partialTicks))
		entityPos := h.StartEntityPos.Add(entityPosDelta.Mul(partialTicks))
		h.NonRaycastResults = append(h.NonRaycastResults, game.ClosestPointToBBox(attackPos, h.TargetedEntity.Box(entityPos).Grow(0.1)).Sub(attackPos).Len())
	}
}

func (h *CombatHandler) calculateRaycastResults(p *player.Player) {
	mDat := p.Handler(HandlerIDMovement).(*MovementHandler)
	attackPosDelta := h.EndAttackPos.Sub(h.StartAttackPos)
	entityPosDelta := h.EndEntityPos.Sub(h.StartEntityPos)

	startRotation := mDat.PrevRotation
	endRotation := mDat.Rotation
	rotationDelta := endRotation.Sub(startRotation)
	if rotationDelta.Len() >= 180 {
		return
	}

	altEntityStartPos := h.TargetedEntity.PrevPosition
	altEntityEndPos := h.TargetedEntity.Position
	altEntityPosDelta := altEntityEndPos.Sub(altEntityStartPos)

	/* altStartAttackPos := mDat.PrevClientPosition.Add(mgl32.Vec3{0, h.AttackOffset})
	altEndAttackPos := mDat.ClientPosition.Add(mgl32.Vec3{0, h.AttackOffset})
	altAttackPosDelta := altEndAttackPos.Sub(altStartAttackPos) */

	h.RaycastResults = make([]float32, 0, 20)
	for partialTicks := float32(0); partialTicks <= 1; partialTicks += h.InterpolationStep {
		attackPos := h.StartAttackPos.Add(attackPosDelta.Mul(partialTicks))
		entityPos := h.StartEntityPos.Add(entityPosDelta.Mul(partialTicks))
		bb := h.TargetedEntity.Box(entityPos).Grow(0.1)

		rotation := startRotation.Add(rotationDelta.Mul(partialTicks))
		directionVec := game.DirectionVector(rotation.Z(), rotation.X()).Mul(14)

		result, ok := trace.BBoxIntercept(bb, attackPos, attackPos.Add(directionVec))
		if ok {
			h.RaycastResults = append(h.RaycastResults, attackPos.Sub(result.Position()).Len())
		}

		// An extra raycast is ran here with the current entity position, as the client may have ticked
		// the entity to a new position while the frame logic was running (where attacks are done).
		entityPos = altEntityStartPos.Add(altEntityPosDelta.Mul(partialTicks))
		bb = h.TargetedEntity.Box(entityPos).Grow(0.1)
		result, ok = trace.BBoxIntercept(bb, attackPos, attackPos.Add(directionVec))
		if ok {
			h.RaycastResults = append(h.RaycastResults, attackPos.Sub(result.Position()).Len())
		}
	}
}

// Click adds a click to the player's click history.
func (h *CombatHandler) click(p *player.Player) {
	currentTick := p.ClientFrame

	h.Clicking = true
	if len(h.Clicks) > 0 {
		h.ClickDelay = (currentTick - h.LastClickTick) * 50
	} else {
		h.ClickDelay = 0
	}
	h.Clicks = append(h.Clicks, currentTick)
	var clicks []int64
	for _, clickTick := range h.Clicks {
		if currentTick-clickTick <= 20 {
			clicks = append(clicks, clickTick)
		}
	}
	h.LastClickTick = currentTick
	h.Clicks = clicks
	h.CPS = len(h.Clicks)
}
