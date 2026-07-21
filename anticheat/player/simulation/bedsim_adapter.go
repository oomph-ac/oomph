package simulation

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/oomph-ac/bedsim"
	"github.com/oomph-ac/oomph/anticheat/game"
	"github.com/oomph-ac/oomph/anticheat/player"
	"github.com/oomph-ac/oomph/anticheat/utils"
	oworld "github.com/oomph-ac/oomph/anticheat/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func simulateWithBedsim(p *player.Player, movement player.MovementComponent) bedsim.SimulationResult {
	state := movementStateFromComponent(p, movement)
	sim := bedsim.Simulator{
		World:          bedsimWorldProvider{w: p.World()},
		BlockSemantics: bedsimBlockSemantics{},
		Effects:        bedsimEffectsProvider{p: p},
		Inventory:      bedsimInventoryProvider{p: p},
		Options: bedsim.SimulationOptions{
			UseSlideOffset:              p.VersionInRange(-1, player.GameVersion1_20_60),
			PositionCorrectionThreshold: p.Opts().Movement.CorrectionThreshold,
			LimitAllVelocity:            p.Opts().Movement.LimitAllVelocity,
			LimitAllVelocityThreshold:   p.Opts().Movement.LimitAllVelocityThreshold,
			Debugf: func(format string, args ...any) {
				p.Dbg.Notify(player.DebugModeMovementSim, true, format, args...)
			},
		},
	}

	result := sim.SimulateState(&state)
	applyBedsimState(movement, &state)
	return result
}

func movementStateFromComponent(p *player.Player, movement player.MovementComponent) bedsim.MovementState {
	client := movement.Client()
	state := bedsim.MovementState{
		// Client mirrors raw non-authoritative values used for correction deltas and tie-breaks.
		Client: bedsim.ClientState{
			Pos:                 client.Pos(),
			LastPos:             client.LastPos(),
			Vel:                 client.Vel(),
			LastVel:             client.LastVel(),
			Mov:                 client.Mov(),
			LastMov:             client.LastMov(),
			HorizontalCollision: client.HorizontalCollision(),
			VerticalCollision:   client.VerticalCollision(),
			ToggledFly:          client.ToggledFly(),
		},
		Pos:          movement.Pos(),
		LastPos:      movement.LastPos(),
		Vel:          movement.Vel(),
		LastVel:      movement.LastVel(),
		Mov:          movement.Mov(),
		LastMov:      movement.LastMov(),
		Rotation:     movement.Rotation(),
		LastRotation: movement.LastRotation(),
		SlideOffset:  movement.SlideOffset(),
		Impulse:      movement.Impulse(),
		Size:         movement.Size(),

		// Supporting block is tracked separately from collision booleans for edge-case jump logic.
		SupportingBlockPos: cloneBlockPos(movement.SupportingBlockPos()),

		Gravity:      movement.Gravity(),
		JumpHeight:   movement.JumpHeight(),
		FallDistance: movement.FallDistance(),

		MovementSpeed:        movement.MovementSpeed(),
		DefaultMovementSpeed: movement.DefaultMovementSpeed(),
		AirSpeed:             movement.AirSpeed(),

		Knockback: movement.Knockback(),

		PendingTeleportPos: movement.PendingTeleportPos(),
		PendingTeleports:   movement.PendingTeleports(),

		TeleportPos:        movement.TeleportPos(),
		TicksSinceTeleport: movement.TicksSinceTeleport(),
		TeleportIsSmoothed: movement.TeleportSmoothed(),

		Sprinting:      movement.Sprinting(),
		PressingSprint: movement.PressingSprint(),
		ServerSprint:   movement.ServerSprint(),

		Sneaking:      movement.Sneaking(),
		PressingSneak: movement.PressingSneak(),

		Jumping:      movement.Jumping(),
		PressingJump: movement.PressingJump(),
		// Climbing uses EffectiveJumping; Oomph currently only exposes the held jump key.
		EffectiveJumping: movement.PressingJump(),
		JumpDelay:        movement.JumpDelay(),

		CollideX: movement.XCollision(),
		CollideY: movement.YCollision(),
		CollideZ: movement.ZCollision(),
		OnGround: movement.OnGround(),

		PenetratedLastFrame: movement.PenetratedLastFrame(),
		StuckInCollider:     movement.StuckInCollider(),

		Immobile: movement.Immobile(),
		NoClip:   movement.NoClip(),

		Gliding:         movement.Gliding(),
		GlideBoostTicks: movement.GlideBoost(),

		HasGravity: movement.HasGravity(),

		Flying:               movement.Flying(),
		MayFly:               movement.MayFly(),
		TrustFlyStatus:       movement.TrustFlyStatus(),
		JustDisabledFlight:   movement.JustDisabledFlight(),
		InCorrectionCooldown: movement.InCorrectionCooldown(),

		PendingCorrections: movement.PendingCorrections(),

		Ready:    p.Ready,
		Alive:    p.Alive,
		GameMode: p.GameMode,
	}

	// Preserve current tick's knockback/teleport windows so bedsim keeps the same gates as Oomph.
	if movement.HasKnockback() {
		state.TicksSinceKnockback = 0
	} else {
		state.TicksSinceKnockback = 1
	}

	if movement.HasTeleport() {
		remaining := movement.RemainingTeleportTicks()
		if remaining < 0 {
			remaining = 0
		}
		state.TeleportCompletionTicks = state.TicksSinceTeleport + uint64(remaining)
	}

	return state
}

func applyBedsimState(movement player.MovementComponent, state *bedsim.MovementState) {
	if state == nil {
		return
	}

	movement.SetSlideOffset(state.SlideOffset)
	movement.SetSupportingBlockPos(cloneBlockPos(state.SupportingBlockPos))

	// Preserve Oomph's last/current snapshots in the same order as the legacy movement path.
	movement.SetPos(state.LastPos)
	movement.SetPos(state.Pos)
	movement.SetVel(state.LastVel)
	movement.SetVel(state.Vel)
	movement.SetMov(state.LastMov)
	movement.SetMov(state.Mov)

	movement.SetCollisions(state.CollideX, state.CollideY, state.CollideZ)
	movement.SetOnGround(state.OnGround)
	movement.SetPenetratedLastFrame(state.PenetratedLastFrame)
	movement.SetStuckInCollider(state.StuckInCollider)
	movement.SetJumpDelay(state.JumpDelay)
	movement.SetGliding(state.Gliding)
}

// intersectingLiquid reports whether the movement bounding box currently intersects a liquid block.
// Oomph still exempts liquid scenarios until swimming/liquid-layer state is wired through to bedsim.
func intersectingLiquid(p *player.Player, movement player.MovementComponent) bool {
	stateBB := movement.BoundingBox()
	for result := range utils.NearbyBlocks(stateBB.Grow(1), false, true, p.World()) {
		if _, isLiquid := result.Block.(world.Liquid); !isLiquid {
			continue
		}
		blockBB := cube.Box32(0, 0, 0, 1, 1, 1).Translate(game.BlockPosVec3(result.Position))
		if stateBB.IntersectsWith(blockBB) {
			return true
		}
	}
	return false
}

type bedsimWorldProvider struct {
	w *oworld.World
}

func (wp bedsimWorldProvider) Block(pos cube.Pos) world.Block {
	if wp.w == nil {
		return block.Air{}
	}
	return wp.w.Block(pos)
}

func (wp bedsimWorldProvider) BlockCollisions(pos cube.Pos) []cube.BBox32 {
	if wp.w == nil {
		return nil
	}
	b := wp.w.Block(pos)
	return utils.BlockCollisions(b, pos, wp.w)
}

func (wp bedsimWorldProvider) GetNearbyBBoxes(aabb cube.BBox32) []cube.BBox32 {
	if wp.w == nil {
		return nil
	}
	return utils.NearbyBBoxes(aabb, wp.w)
}

func (wp bedsimWorldProvider) HasNearbyBBoxes(aabb cube.BBox32) bool {
	if wp.w == nil {
		return false
	}
	return utils.HasNearbyBBoxes(aabb, wp.w)
}

func (wp bedsimWorldProvider) IsChunkLoaded(chunkX, chunkZ int32) bool {
	if wp.w == nil {
		return false
	}
	return wp.w.Chunk(protocol.ChunkPos{chunkX, chunkZ}) != nil
}

type bedsimBlockSemantics struct{}

func (bedsimBlockSemantics) BlockName(b world.Block) string {
	return utils.BlockName(b)
}

func (bedsimBlockSemantics) BlockFriction(b world.Block) float32 {
	return utils.BlockFriction(b)
}

func (bedsimBlockSemantics) BlockClimbable(b world.Block) bool {
	return utils.BlockClimbable(b)
}

type bedsimEffectsProvider struct {
	p *player.Player
}

func (ep bedsimEffectsProvider) GetEffect(effectID int32) (int32, bool) {
	if ep.p == nil {
		return 0, false
	}
	effects := ep.p.Effects()
	if effects == nil {
		return 0, false
	}
	eff, ok := effects.Get(effectID)
	if !ok {
		return 0, false
	}
	return eff.Amplifier, true
}

type bedsimInventoryProvider struct {
	p *player.Player
}

func (ip bedsimInventoryProvider) HasElytra() bool {
	if ip.p == nil {
		return false
	}
	inventory := ip.p.Inventory()
	if inventory == nil {
		return false
	}
	_, ok := inventory.Chestplate().Item().(item.Elytra)
	return ok
}

func cloneBlockPos(pos *cube.Pos) *cube.Pos {
	if pos == nil {
		return nil
	}
	cloned := *pos
	return &cloned
}
