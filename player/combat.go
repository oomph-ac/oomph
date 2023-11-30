package player

import (
	"fmt"
	"math"

	"github.com/ethaniccc/float32-cube/cube/trace"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/entity"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const maxCrosshairAttackDist float32 = 3.0
const maxTouchAttackDist float32 = 3.125

// updateCombatData updates the player's current combat data, and sets the needsCombatValidation flag to true. The combat data will be
// nil if the player swung in the air (check for possible client misprediction).
func (p *Player) updateCombatData(pk *packet.InventoryTransaction) {
	p.lastAttackData = pk
	p.needsCombatValidation = true
}

// validateCombat checks if the player's attack was valid for the tick. If combat is found to be legitimate, this function
// will return true. Note that if multiple attacks are recieved in a tick, this function will only validate the first
// processed in the tick, and any other hits will be ignored until next tick.
func (p *Player) validateCombat(attackPos mgl32.Vec3) {
	defer func() {
		p.needsCombatValidation = false
		p.lastAttackData = nil
	}()

	// There is no combat that needs to be validated as of now.
	if !p.needsCombatValidation || !p.ready {
		return
	}

	// If the player is in a gamemode that has extended reach, there is no need to validate combat.
	if p.gamemode != packet.GameTypeSurvival && p.gamemode != packet.GameTypeAdventure {
		if p.lastAttackData != nil {
			p.SendPacketToServer(p.lastAttackData)
		}

		return
	}

	// If the player hasn't recieved a teleport yet, they are at a disavantage due to latency. This is because most server
	// softwares will force the player to be at the teleport position before they have even recieved the teleport packet.
	if p.mInfo.AwaitingTeleport {
		attackPos = p.mInfo.TeleportPos.Add(mgl32.Vec3{0, p.eyeOffset, 0})
	}

	// This determines what tick we should rewind to get an entity position for lag compensation.
	// Lag compensation is limited to 300ms by default in this case, so we want two things:
	// 1) The tick we should rewind to should be no more than the latency cutoff in the past.
	// 2) The tick we should rewind to should not be higher than the current server rewTick
	rewTick, sTick, cut := p.clientTick.Load()-1, p.serverTick.Load(), uint64(p.combatNetworkCutoff)

	// If the current tick we want to rewind to is lower than the latency cutoff, we need to cut it off.
	if rewTick+cut < sTick {
		rewTick = sTick - cut + 1
		p.TryDebug(fmt.Sprint("cutoff reached - least available tick is ", rewTick, " (max rewind is ", p.combatNetworkCutoff, ")"), DebugTypeChat, p.debugger.LogCombat)
	}

	// The rewind cannot exceed the current (server tick - 1). If it does, set the rewind tick to the server tick.
	if rewTick >= sTick {
		rewTick = sTick - 1
	}

	if p.lastAttackData == nil {
		// We can't assume the touch client has mispredicted a hit because they can pretty
		// much touch anywhere on their screen and hit an entity.
		if p.inputMode == packet.InputModeTouch {
			return
		}

		// We're going to be unable to create an inventory transaction for this hit if no equipment data is available.
		if p.lastEquipmentData == nil {
			return
		}

		min, valid, eid := float32(69000.0), false, uint64(0)
		dV := game.DirectionVector(p.Entity().Rotation().Z(), p.Entity().Rotation().X())

		// Check if there is a block in the way of our raycast. If this is the case, then we cannot continue.
		b, blockDist := p.GetTargetBlock(dV, attackPos, maxCrosshairAttackDist)
		p.entities.Range(func(k, v any) bool {
			e := v.(*entity.Entity)
			id := k.(uint64)

			rew := e.RewindPosition(rewTick)
			if rew == nil {
				return true
			}

			if rew.Position.Sub(p.mInfo.ServerPosition).LenSqr() > 20.25 { // 20.25 ^ 0.5 = 4.5 - entities that are used for raycasting are 4.5 blocks away
				return true
			}

			targetAABB := e.AABB().Grow(0.1).Translate(rew.Position)

			res, ok := trace.BBoxIntercept(targetAABB, attackPos, attackPos.Add(dV.Mul(maxCrosshairAttackDist)))
			if !ok {
				return true
			}

			dist := res.Position().Sub(attackPos).LenSqr()

			// The player's ray intersects with the block first which means they shouldn't be able to attack the entity.
			if b != nil && blockDist < dist {
				return true
			}

			if dist <= min {
				min = dist
				eid = id
				valid = true
			}

			return true
		})

		if valid {
			p.TryDebug("(client misprediction) valid w/ dist="+fmt.Sprint(math.Sqrt(float64(min))), DebugTypeChat, p.debugger.LogCombat)
			p.SendPacketToServer(&packet.InventoryTransaction{
				TransactionData: &protocol.UseItemOnEntityTransactionData{
					TargetEntityRuntimeID: eid,
					ActionType:            protocol.UseItemOnEntityActionAttack,
					HotBarSlot:            int32(p.lastEquipmentData.HotBarSlot),
					HeldItem:              p.lastEquipmentData.NewItem,
					Position:              p.mInfo.ServerPosition,
					ClickedPosition:       mgl32.Vec3{},
				},
			})
		}

		return
	}

	hit, ok := p.lastAttackData.TransactionData.(*protocol.UseItemOnEntityTransactionData)
	if !ok { // This should never happen, as lastAttackData will only be sent when an attack is detected
		return
	}

	if hit.ActionType != protocol.UseItemOnEntityActionAttack {
		return
	}

	t, ok := p.SearchEntity(hit.TargetEntityRuntimeID)
	if !ok {
		return
	}

	// The rewind should never be null here because we have validated the rewind tick.
	rew := t.RewindPosition(rewTick)
	if rew == nil {
		if len(t.PositionBuffer()) >= int(p.combatNetworkCutoff) {
			p.SendOomphDebug("§cERROR: §7Combat system failed to rewind, please report this to an admin. (buffSize="+fmt.Sprint(len(t.PositionBuffer()))+" currTick="+
				fmt.Sprint(p.ServerTick())+" rewTick="+fmt.Sprint(rewTick)+")", packet.TextTypeChat)
		}

		return
	}

	targetAABB := t.AABB().Grow(0.1).Translate(rew.Position)

	if targetAABB.IntersectsWith(p.AABB()) {
		p.SendPacketToServer(p.lastAttackData)
		p.TryDebug("hit valid: intersected with player", DebugTypeChat, p.debugger.LogCombat)
		return
	}

	// AABB distance check, to make sure the player is within search range of the entity.
	touchDist := game.AABBVectorDistance(targetAABB, attackPos)
	if touchDist > maxTouchAttackDist {
		p.TryDebug("hit invalid: aabb dist check failed w/ dist="+fmt.Sprint(touchDist), DebugTypeChat, p.debugger.LogCombat)
		return
	}

	// If a player's input mode is touch, then a raycast will not be performed to validate combat.
	// This is because touchscreen players have the ability to use touch controls (instead of split controls),
	// which would allow the player to attack another entity without actually looking at them.
	if p.inputMode == packet.InputModeTouch {
		p.SendPacketToServer(p.lastAttackData)
		return
	}

	dV := game.DirectionVector(p.Entity().Rotation().Z(), p.Entity().Rotation().X())
	res, ok := trace.BBoxIntercept(targetAABB, attackPos, attackPos.Add(dV.Mul(14)))

	b, d := p.GetTargetBlock(dV, attackPos, maxCrosshairAttackDist)
	if ok {
		dist := res.Position().Sub(attackPos).Len()
		valid := dist <= maxCrosshairAttackDist

		if b != nil && d < dist {
			p.TryDebug("(misprediction) block "+utils.BlockName(b)+" in the way at dist "+fmt.Sprint(d), DebugTypeChat, p.debugger.LogCombat)
			return
		}

		color := "§c"
		if valid {
			color = "§a"
		}
		p.TryDebug("dist="+fmt.Sprint(dist)+" && valid="+color+fmt.Sprint(valid), DebugTypeChat, p.debugger.LogCombat)

		if !valid {
			return
		}

		p.SendPacketToServer(p.lastAttackData)
	} else {
		p.TryDebug("hit invalid: casted ray did not land. rewTick:"+fmt.Sprint(rewTick), DebugTypeChat, p.debugger.LogCombat)
	}
}
