package handler

import (
	"github.com/chewxy/math32"
	"github.com/ethaniccc/float32-cube/cube/trace"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
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
	StartAttackPos    mgl32.Vec3
	EndAttackPos      mgl32.Vec3

	ClosestRawDistance        float32
	ClosestDirectionalResults []float32
	RaycastResults            []float32

	LastFrame     uint64
	LastSwingTick int64
}

func NewCombatHandler() *CombatHandler {
	return &CombatHandler{
		InterpolationStep: 1 / 20.0,
	}
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

		entity := p.Handler(HandlerIDEntities).(*EntitiesHandler).Find(dat.TargetEntityRuntimeID)
		if entity == nil {
			return true
		}

		movementHandler := p.Handler(HandlerIDMovement).(*MovementHandler)
		h.AttackOffset = float32(1.62)
		if movementHandler.Sneaking {
			h.AttackOffset = 1.54
		}

		h.Phase = CombatPhaseTransaction
		h.TargetedEntity = entity
		h.StartAttackPos = movementHandler.PrevClientPosition.Add(mgl32.Vec3{0, h.AttackOffset})
		h.EndAttackPos = movementHandler.ClientPosition.Add(mgl32.Vec3{0, h.AttackOffset}).Add(movementHandler.ClientMov)

		// Calculate the closest raw point from the attack positions to the entity's bounding box.
		entityBB := entity.Box(entity.Position).Grow(0.1)
		point1 := game.ClosestPointToBBox(h.StartAttackPos, entityBB)
		point2 := game.ClosestPointToBBox(h.EndAttackPos, entityBB)

		h.ClosestRawDistance = math32.Min(
			point1.Sub(h.StartAttackPos).Len(),
			point2.Sub(h.EndAttackPos).Len(),
		)
	case *packet.PlayerAuthInput:
		h.LastFrame = pk.Tick
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
		h.calculatePointingResults(p)
	case *packet.Animate:
		h.LastSwingTick = p.ClientFrame
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
}

func (h *CombatHandler) calculatePointingResults(p *player.Player) {
	movementHandler := p.Handler(HandlerIDMovement).(*MovementHandler)
	attackPosDelta := h.EndAttackPos.Sub(h.StartAttackPos)

	if movementHandler.TicksSinceTeleport == 0 {
		h.StartAttackPos = movementHandler.TeleportPos.Add(mgl32.Vec3{0, h.AttackOffset})
		h.EndAttackPos = movementHandler.TeleportPos.Add(mgl32.Vec3{0, h.AttackOffset})
	}

	startRotation := movementHandler.PrevRotation
	endRotation := movementHandler.Rotation
	rotationDelta := startRotation.Sub(endRotation)
	if startRotation.Sub(endRotation).Len() > 180 {
		return
	}

	startDirection := game.DirectionVector(startRotation.Z(), startRotation.X())
	endDirection := game.DirectionVector(endRotation.Z(), endRotation.X())

	startEntityPos := h.TargetedEntity.PrevPosition
	endEntityPos := h.TargetedEntity.Position
	entityPosDelta := endEntityPos.Sub(startEntityPos)

	h.ClosestDirectionalResults = []float32{}
	h.RaycastResults = []float32{}

	for partialTicks := float32(0); partialTicks <= 1; partialTicks += h.InterpolationStep {
		attackPos := h.StartAttackPos.Add(attackPosDelta.Mul(partialTicks))
		attackRotation := startRotation.Add(rotationDelta.Mul(partialTicks))
		entityPos := startEntityPos.Add(entityPosDelta.Mul(partialTicks))

		if partialTicks == 1.0 {
			attackPos = movementHandler.ClientPosition.Add(mgl32.Vec3{0, h.AttackOffset})
		}

		directionVector := game.DirectionVector(attackRotation.Z(), attackRotation.X())
		entityBB := h.TargetedEntity.Box(entityPos).Grow(0.1)

		h.ClosestDirectionalResults = append(h.ClosestDirectionalResults, game.ClosestPointToBBoxDirectional(
			attackPos,
			startDirection,
			endDirection,
			entityBB,
			14.0,
		).Sub(attackPos).Len())

		result, ok := trace.BBoxIntercept(entityBB, attackPos, attackPos.Add(directionVector.Mul(14.0)))
		if ok {
			h.RaycastResults = append(h.RaycastResults, attackPos.Sub(result.Position()).Len())
		}
	}
}
