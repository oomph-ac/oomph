package simulation

import (
	"github.com/df-mc/dragonfly/server/block"
	df_cube "github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/world"
	float_cube "github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/oomph-ac/bedsim"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	oworld "github.com/oomph-ac/oomph/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

func simulateWithBedsim(p *player.Player, movement player.MovementComponent) bedsim.SimulationResult {
	state := movementStateFromComponent(p, movement)
	sim := bedsim.Simulator{
		World:     bedsimWorldProvider{w: p.World()},
		Effects:   bedsimEffectsProvider{p: p},
		Inventory: bedsimInventoryProvider{p: p},
		Options: bedsim.SimulationOptions{
			UseSlideOffset:              p.VersionInRange(-1, player.GameVersion1_20_60),
			PositionCorrectionThreshold: float64(p.Opts().Movement.CorrectionThreshold),
			LimitAllVelocity:            p.Opts().Movement.LimitAllVelocity,
			LimitAllVelocityThreshold:   float64(p.Opts().Movement.LimitAllVelocityThreshold),
		},
	}

	result := sim.SimulateState(&state)
	applyBedsimState(movement, &state)
	return result
}

func movementStateFromComponent(p *player.Player, movement player.MovementComponent) bedsim.MovementState {
	client := movement.Client()
	state := bedsim.MovementState{
		Client: bedsim.ClientState{
			Pos:                 vec32To64(client.Pos()),
			LastPos:             vec32To64(client.LastPos()),
			Vel:                 vec32To64(client.Vel()),
			LastVel:             vec32To64(client.LastVel()),
			Mov:                 vec32To64(client.Mov()),
			LastMov:             vec32To64(client.LastMov()),
			HorizontalCollision: client.HorizontalCollision(),
			VerticalCollision:   client.VerticalCollision(),
			ToggledFly:          client.ToggledFly(),
		},
		Pos:          vec32To64(movement.Pos()),
		LastPos:      vec32To64(movement.LastPos()),
		Vel:          vec32To64(movement.Vel()),
		LastVel:      vec32To64(movement.LastVel()),
		Mov:          vec32To64(movement.Mov()),
		LastMov:      vec32To64(movement.LastMov()),
		Rotation:     vec32To64(movement.Rotation()),
		LastRotation: vec32To64(movement.LastRotation()),
		SlideOffset:  vec2_32To64(movement.SlideOffset()),
		Impulse:      vec2_32To64(movement.Impulse()),
		Size:         vec32To64(movement.Size()),

		SupportingBlockPos: toDFPosPtr(movement.SupportingBlockPos()),

		Gravity:      float64(movement.Gravity()),
		JumpHeight:   float64(movement.JumpHeight()),
		FallDistance: float64(movement.FallDistance()),

		MovementSpeed:        float64(movement.MovementSpeed()),
		DefaultMovementSpeed: float64(movement.DefaultMovementSpeed()),
		AirSpeed:             float64(movement.AirSpeed()),

		Knockback: vec32To64(movement.Knockback()),

		PendingTeleportPos: vec32To64(movement.PendingTeleportPos()),
		PendingTeleports:   movement.PendingTeleports(),

		TeleportPos:        vec32To64(movement.TeleportPos()),
		TicksSinceTeleport: movement.TicksSinceTeleport(),
		TeleportIsSmoothed: movement.TeleportSmoothed(),

		Sprinting:      movement.Sprinting(),
		PressingSprint: movement.PressingSprint(),
		ServerSprint:   movement.ServerSprint(),

		Sneaking:      movement.Sneaking(),
		PressingSneak: movement.PressingSneak(),

		Jumping:      movement.Jumping(),
		PressingJump: movement.PressingJump(),
		JumpDelay:    movement.JumpDelay(),

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

	movement.SetSlideOffset(vec2_64To32(state.SlideOffset))
	movement.SetSupportingBlockPos(toFloatPosPtr(state.SupportingBlockPos))

	movement.SetPos(vec64To32(state.LastPos))
	movement.SetPos(vec64To32(state.Pos))
	movement.SetVel(vec64To32(state.LastVel))
	movement.SetVel(vec64To32(state.Vel))
	movement.SetMov(vec64To32(state.LastMov))
	movement.SetMov(vec64To32(state.Mov))

	movement.SetCollisions(state.CollideX, state.CollideY, state.CollideZ)
	movement.SetOnGround(state.OnGround)
	movement.SetPenetratedLastFrame(state.PenetratedLastFrame)
	movement.SetStuckInCollider(state.StuckInCollider)
	movement.SetJumpDelay(state.JumpDelay)
	movement.SetGliding(state.Gliding)
}

type bedsimWorldProvider struct {
	w *oworld.World
}

func (wp bedsimWorldProvider) Block(pos df_cube.Pos) world.Block {
	if wp.w == nil {
		return block.Air{}
	}
	return wp.w.Block(pos)
}

func (wp bedsimWorldProvider) BlockCollisions(pos df_cube.Pos) []df_cube.BBox {
	if wp.w == nil {
		return nil
	}
	b := wp.w.Block(pos)
	pos32 := float_cube.Pos{pos[0], pos[1], pos[2]}
	boxes32 := utils.BlockCollisions(b, pos32, wp.w)

	boxes := make([]df_cube.BBox, len(boxes32))
	for i, bb := range boxes32 {
		boxes[i] = game.CubeBoxToDFBox(bb)
	}
	return boxes
}

func (wp bedsimWorldProvider) GetNearbyBBoxes(aabb df_cube.BBox) []df_cube.BBox {
	if wp.w == nil {
		return nil
	}
	boxes32 := utils.GetNearbyBBoxes(game.DFBoxToCubeBox(aabb), wp.w)
	boxes := make([]df_cube.BBox, len(boxes32))
	for i, bb := range boxes32 {
		boxes[i] = game.CubeBoxToDFBox(bb)
	}
	return boxes
}

func (wp bedsimWorldProvider) IsChunkLoaded(chunkX, chunkZ int32) bool {
	if wp.w == nil {
		return false
	}
	return wp.w.GetChunk(protocol.ChunkPos{chunkX, chunkZ}) != nil
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

func toDFPosPtr(pos *float_cube.Pos) *df_cube.Pos {
	if pos == nil {
		return nil
	}
	dfPos := df_cube.Pos{pos[0], pos[1], pos[2]}
	return &dfPos
}

func toFloatPosPtr(pos *df_cube.Pos) *float_cube.Pos {
	if pos == nil {
		return nil
	}
	floatPos := float_cube.Pos{pos[0], pos[1], pos[2]}
	return &floatPos
}

func vec32To64(v mgl32.Vec3) mgl64.Vec3 {
	return mgl64.Vec3{float64(v[0]), float64(v[1]), float64(v[2])}
}

func vec64To32(v mgl64.Vec3) mgl32.Vec3 {
	return mgl32.Vec3{float32(v[0]), float32(v[1]), float32(v[2])}
}

func vec2_32To64(v mgl32.Vec2) mgl64.Vec2 {
	return mgl64.Vec2{float64(v[0]), float64(v[1])}
}

func vec2_64To32(v mgl64.Vec2) mgl32.Vec2 {
	return mgl32.Vec2{float32(v[0]), float32(v[1])}
}
