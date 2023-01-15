package player

import (
	"fmt"

	"github.com/ethaniccc/float32-cube/cube/trace"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func (p *Player) updateCombatData(pk *packet.InventoryTransaction) {
	p.lastAttackData = pk
	p.needsCombatValidation = true
}

// validateCombat checks if the player's attack was valid for the tick. If combat is found to be legitimate, this function
// will return true. Note that if multiple attacks are recieved in a tick, this function will only validate the first
// processed in the tick, and any other hits will be ignored until next tick.
func (p *Player) validateCombat() {
	defer func() {
		p.needsCombatValidation = false
	}()

	// There is no combat that needs to be validated as of now.
	if !p.needsCombatValidation {
		return
	}

	// If the player is in a gamemode that has extended reach, there is no need to validate combat.
	if p.gameMode != packet.GameTypeSurvival && p.gameMode != packet.GameTypeAdventure {
		return
	}

	// This determines what tick we should rewind to get an entity position for lag compensation.
	// Lag compensation is limited to 300ms in this case, so we want two things:
	// 1) The tick we should rewind to should be no more than 6 ticks (300ms) in the past.
	// 2) The tick we should rewind to should not be higher than the current server tick
	tick, stick, cut := p.clientTick.Load(), p.serverTick.Load(), uint64(NetworkLatencyCutoff)
	if tick < stick-cut {
		tick = stick - cut
	}
	if tick > stick {
		tick = stick
	}

	attackPos := p.mInfo.ServerPosition.Add(mgl32.Vec3{0, 1.62})

	if p.lastAttackData == nil {
		if p.inputMode == packet.InputModeTouch {
			return
		}

		// We're going to be unable to create an inventory transaction for this hit if no equipment data is available.
		if p.lastEquipmentData == nil {
			return
		}

		p.entityMu.Lock()
		min, valid, eid := float32(69000.0), false, uint64(0)
		dV := game.DirectionVector(p.Entity().Rotation().Z(), p.Entity().Rotation().X())
		for id, e := range p.entities {
			if id == p.rid {
				continue
			}

			rew := e.RewindPosition(tick)
			if rew == nil {
				continue
			}

			if rew.Position.Sub(p.mInfo.ServerPosition).LenSqr() > 20.25 { // 20.25 ^ 0.5 = 4.5 - entities that are used for raycasting are always 4.5 blocks away
				continue
			}

			targetAABB := e.AABB().Grow(0.125).Translate(rew.Position)

			res, ok := trace.BBoxIntercept(targetAABB, attackPos, attackPos.Add(dV.Mul(3)))
			if !ok {
				continue
			}

			dist := res.Position().Sub(attackPos).LenSqr()
			if dist <= min {
				min = dist
				eid = id
				valid = true
			}
		}
		p.entityMu.Unlock()

		if valid {
			p.sendPacketToServer(&packet.InventoryTransaction{
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

	if t, ok := p.SearchEntity(hit.TargetEntityRuntimeID); ok {
		rew := t.RewindPosition(tick)
		if rew == nil {
			return
		}

		dist := game.AABBVectorDistance(t.AABB().Translate(rew.Position), attackPos)
		if dist > 3.1 {
			return
		}

		// If a player's input mode is touch, then a raycast will not be performed to validate combat.
		// This is because touchscreen players have the ability to use touch controls (instead of split controls),
		// which would allow the player to attack another entity without actually looking at them.
		if p.inputMode != packet.InputModeTouch {
			targetAABB := t.AABB().Grow(0.125).Translate(rew.Position)
			dV := game.DirectionVector(p.Entity().Rotation().Z(), p.Entity().Rotation().X())
			_, ok := trace.BBoxIntercept(targetAABB, attackPos, attackPos.Add(dV.Mul(3)))

			if ok {
				p.sendPacketToServer(p.lastAttackData)
			} else if p.debugger.ServerCombat {
				p.SendOomphDebug(fmt.Sprint("hit invalidated! raycast failed && {pos: ", game.RoundVec32(rew.Position, 4), " yaw: ", game.Round32(p.Rotation()[2], 2)), packet.TextTypeChat)
			}
		}
	}
}
