package utils

import (
	"github.com/df-mc/dragonfly/server/block"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/block/model"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/enchantment"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	oworld "github.com/oomph-ac/oomph/world"
)

type blockPlacer interface {
	Position() mgl64.Vec3
	Rotation() cube.Rotation
	HeldItems() (mainHand, offHand item.Stack)
	PlaceBlock(clickedBlockPos, replaceBlockPos cube.Pos, face cube.Face, b world.Block)
}

func ActivateBlock(placer blockPlacer, b block.Activatable, replaceBlockPos cube.Pos, src *oworld.World) {
	if placer == nil {
		return
	}

	held, _ := placer.HeldItems()
	switch b := b.(type) {
	case block.Anvil, block.Barrel, block.Beacon, block.BlastFurnace, block.BrewingStand,
		block.Chest, block.CraftingTable, block.EnchantingTable, block.EnderChest,
		block.Furnace, block.Grindstone, block.Hopper, block.Loom, block.SmithingTable,
		block.Smoker, block.Stonecutter:
		// We don't need to do anything here since the server will send the appropriate windows.
	case block.Cake:
		b.Bites++
		if b.Bites > 6 {
			src.SetBlock(replaceBlockPos, block.Air{}, nil)
			return
		}
		src.SetBlock(replaceBlockPos, b, nil)
	case block.Campfire, block.DecoratedPot, block.DragonEgg, block.ItemFrame, block.Jukebox, block.Lectern,
		block.Note, block.Sign:
		// Remote server should implement this logic - since there is no change to meaningful change
		// to the bounding box - we don't need to do anything here.
	case block.Composter:
		if b.Level >= 7 {
			if b.Level == 8 {
				b.Level = 0
				src.SetBlock(replaceBlockPos, block.Air{}, nil)
				return
			}
			return
		}
		// TODO: Impl random chance of composting??
		if _, ok := held.Item().(item.Compostable); ok {
			b.Level++
		}
	case block.CopperDoor:
		b.Open = !b.Open
		src.SetBlock(replaceBlockPos, b, nil)
	case block.CopperTrapdoor:
		b.Open = !b.Open
		src.SetBlock(replaceBlockPos, b, nil)
	case block.TNT:
		if _, ok := held.Enchantment(enchantment.FireAspect); ok || ItemName(held.Item()) == "minecraft:flint_and_steel" {
			src.SetBlock(replaceBlockPos, block.Air{}, nil)
		}
	case block.WoodDoor:
		b.Open = !b.Open
		src.SetBlock(replaceBlockPos, b, nil)
	case block.WoodFenceGate:
		b.Open = !b.Open
		src.SetBlock(replaceBlockPos, b, nil)
	case block.WoodTrapdoor:
		b.Open = !b.Open
		src.SetBlock(replaceBlockPos, b, nil)
	}
}

type UseOnBlockOpts struct {
	Placer         blockPlacer
	UseableOnBlock item.UsableOnBlock

	ClickedBlockPos cube.Pos
	ReplaceBlockPos cube.Pos

	ClickPos mgl64.Vec3
	Face     cube.Face
	Src      *oworld.World
}

func UseOnBlock(opts UseOnBlockOpts) {
	placer := opts.Placer
	b := opts.UseableOnBlock
	clickedBlockPos := opts.ClickedBlockPos
	replaceBlockPos := opts.ReplaceBlockPos
	clickPos := opts.ClickPos
	face := opts.Face
	src := opts.Src

	if placer == nil {
		return
	}

	switch b := b.(type) {
	case block.Anvil:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Facing = placer.Rotation().Direction().RotateLeft()
		place(placer, clickedBlockPos, pos, face, b)
	case block.Banner:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used || face == cube.FaceDown {
			return
		}
		if face == cube.FaceUp {
			b.Attach = block.StandingAttachment(placer.Rotation().Orientation().Opposite())
			place(placer, clickedBlockPos, pos, face, b)
			return
		}
		b.Attach = block.WallAttachment(face.Direction())
		place(placer, clickedBlockPos, pos, face, b)
	case block.Barrel:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b = block.NewBarrel()
		b.Facing = calculateFace(placer, pos)
		place(placer, clickedBlockPos, pos, face, b)
	case block.Basalt:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Axis = face.Axis()
		place(placer, clickedBlockPos, pos, face, b)
	case block.BeetrootSeeds:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if _, ok := src.Block(pos.Side(cube.FaceDown)).(block.Farmland); !ok {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.BlastFurnace:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		place(placer, clickedBlockPos, pos, face, block.NewBlastFurnace(placer.Rotation().Direction().Opposite()))
	case block.Bone:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Axis = face.Axis()
		place(placer, clickedBlockPos, pos, face, b)
	case block.BrewingStand:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b = block.NewBrewingStand()
		place(placer, clickedBlockPos, pos, face, b)
	case block.Cactus:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used || !canCactusGrowOn(pos, src, true) {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.Cake:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if _, air := src.Block(pos.Side(cube.FaceDown)).(block.Air); air {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.Campfire:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if _, ok := src.Block(pos.Side(cube.FaceDown)).(block.Campfire); ok {
			return
		}
		b.Facing = placer.Rotation().Direction().Opposite()
		place(placer, clickedBlockPos, pos, face, b)
	case block.Carpet:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if _, ok := src.Block(pos.Side(cube.FaceDown)).(block.Air); ok {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.Carrot:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if _, ok := src.Block(pos.Side(cube.FaceDown)).(block.Farmland); !ok {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.IronChain:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Axis = face.Axis()
		place(placer, clickedBlockPos, pos, face, b)
	case block.Chest:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b = block.NewChest()
		b.Facing = placer.Rotation().Direction().Opposite()

		// Hopefully paired chests don't screw up everything... right?
		place(placer, clickedBlockPos, pos, face, b)
	case block.CocoaBean:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if face == cube.FaceUp || face == cube.FaceDown {
			return
		}

		var woodType block.WoodType
		oppositePos := pos.Side(face.Opposite())
		if log, ok := src.Block(oppositePos).(block.Log); ok {
			woodType = log.Wood
		} else if wood, ok := src.Block(oppositePos).(block.Wood); ok {
			woodType = wood.Wood
		}
		if woodType == block.JungleWood() {
			b.Facing = face.Opposite().Direction()
			place(placer, clickedBlockPos, pos, face, b)
		}
	case block.CopperDoor:
		if face != cube.FaceUp {
			return
		}
		below := replaceBlockPos
		replaceBlockPos = replaceBlockPos.Side(cube.FaceUp)
		if !replaceableWith(src, replaceBlockPos, b) || !replaceableWith(src, replaceBlockPos.Side(cube.FaceUp), b) {
			return
		}
		if !src.Block(below).Model().FaceSolid(below, cube.FaceUp, src) {
			return
		}
		b.Facing = placer.Rotation().Direction()
		left := src.Block(replaceBlockPos.Side(b.Facing.RotateLeft().Face()))
		right := src.Block(replaceBlockPos.Side(b.Facing.RotateRight().Face()))
		if _, ok := left.Model().(model.Door); ok {
			b.Right = true
		}
		// The side the door hinge is on can be affected by the blocks to the left and right of the door. In particular,
		// opaque blocks on the right side of the door with transparent blocks on the left side result in a right sided
		// door hinge.
		if diffuser, ok := right.(block.LightDiffuser); !ok || diffuser.LightDiffusionLevel() != 0 {
			if diffuser, ok := left.(block.LightDiffuser); ok && diffuser.LightDiffusionLevel() == 0 {
				b.Right = true
			}
		}
		// TODO: figure out what is actually happening here...?
		place(placer, clickedBlockPos, replaceBlockPos, face, b)
		place(placer, clickedBlockPos, replaceBlockPos.Side(cube.FaceUp), cube.FaceUp, block.CopperDoor{Oxidation: b.Oxidation, Waxed: b.Waxed, Facing: b.Facing, Top: true, Right: b.Right})
	case block.CopperTrapdoor:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Facing = placer.Rotation().Direction().Opposite()
		b.Top = (clickPos[1] > 0.5 && face != cube.FaceUp) || face == cube.FaceDown
		place(placer, clickedBlockPos, pos, face, b)
	case block.Coral:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if !src.Block(pos.Side(cube.FaceDown)).Model().FaceSolid(pos.Side(cube.FaceDown), cube.FaceUp, src) {
			return
		}
		// TODO: Account for multi-layer liquids.
		if liquid, ok := src.Block(pos).(world.Liquid); ok {
			if water, ok := liquid.(block.Water); ok {
				if water.Depth != 8 {
					return
				}
			}
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.DeadBush:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.DecoratedPot:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Facing = placer.Rotation().Direction().Opposite()
		place(placer, clickedBlockPos, pos, face, b)
	case block.Deepslate:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if b.Type == block.NormalDeepslate() {
			b.Axis = face.Axis()
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.DoubleFlower:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used || !replaceableWith(src, pos.Side(cube.FaceUp), b) || !supportsVegetation(b, src.Block(pos.Side(cube.FaceDown))) {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
		place(placer, clickedBlockPos, pos.Side(cube.FaceUp), cube.FaceUp, block.DoubleFlower{Type: b.Type, UpperPart: true})
	case block.DoubleTallGrass:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used || !replaceableWith(src, pos.Side(cube.FaceUp), b) || !supportsVegetation(b, src.Block(pos.Side(cube.FaceDown))) {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
		place(placer, clickedBlockPos, pos.Side(cube.FaceUp), cube.FaceUp, block.DoubleTallGrass{Type: b.Type, UpperPart: true})
	case block.EndRod:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Facing = face
		if other, ok := src.Block(pos.Side(face.Opposite())).(block.EndRod); ok {
			if face == other.Facing {
				b.Facing = face.Opposite()
			}
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.EnderChest:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b = block.NewEnderChest()
		b.Facing = placer.Rotation().Direction().Opposite()
		place(placer, clickedBlockPos, pos, face, b)
	case block.Fern:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used || !supportsVegetation(b, src.Block(pos.Side(cube.FaceDown))) {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.Flower:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used || !supportsVegetation(b, src.Block(pos.Side(cube.FaceDown))) {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.Froglight:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Axis = face.Axis()
		place(placer, clickedBlockPos, pos, face, b)
	case block.Furnace:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		place(placer, clickedBlockPos, pos, face, block.NewFurnace(placer.Rotation().Direction().Opposite()))
	case block.GlazedTerracotta:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Facing = placer.Rotation().Direction().Opposite()
		place(placer, clickedBlockPos, pos, face, b)
	case block.Grindstone:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Facing = placer.Rotation().Direction().Opposite()
		if face == cube.FaceDown {
			b.Attach = block.HangingGrindstoneAttachment()
		} else if face != cube.FaceUp {
			b.Attach = block.WallGrindstoneAttachment()
			b.Facing = face.Direction()
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.HayBale:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Axis = face.Axis()
		place(placer, clickedBlockPos, pos, face, b)
	case block.Hopper:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b = block.NewHopper()
		b.Facing = cube.FaceDown
		if b.Facing != face {
			b.Facing = face.Opposite()
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.ItemFrame:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		// TODO: also account for Oomph's custom empty model???
		if _, ok := src.Block(pos.Side(face.Opposite())).Model().(model.Empty); ok {
			return
		}
		b.Facing = face.Opposite()
		b.DropChance = 1.0
		place(placer, clickedBlockPos, pos, face, b)
	case block.Kelp:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		below := pos.Side(cube.FaceDown)
		belowBlock := src.Block(below)
		if _, kelp := belowBlock.(block.Kelp); !kelp {
			if !belowBlock.Model().FaceSolid(below, cube.FaceUp, src) {
				return
			}
		}

		liquid, ok := src.Block(pos).(world.Liquid)
		if !ok {
			return
		} else if _, ok := liquid.(block.Water); !ok || liquid.LiquidDepth() < 8 {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.Ladder:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if face == cube.FaceUp || face == cube.FaceDown {
			return
		}
		if _, ok := src.Block(pos.Side(face.Opposite())).(block.LightDiffuser); ok {
			found := false
			for _, i := range []cube.Face{cube.FaceNorth, cube.FaceSouth, cube.FaceEast, cube.FaceWest} {
				if diffuser, ok := src.Block(pos.Side(i)).(block.LightDiffuser); !ok || diffuser.LightDiffusionLevel() == 15 {
					found = true
					face = i.Opposite()
					break
				}
			}
			if !found {
				return
			}
		}
		b.Facing = face.Direction()
		place(placer, clickedBlockPos, pos, face, b)
	case block.Lantern:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if face == cube.FaceDown {
			upPos := pos.Side(cube.FaceUp)
			if _, ok := src.Block(upPos).(block.IronChain); !ok && !src.Block(upPos).Model().FaceSolid(upPos, cube.FaceDown, src) {
				face = cube.FaceUp
			}
		}
		if face != cube.FaceDown {
			downPos := pos.Side(cube.FaceDown)
			if !src.Block(downPos).Model().FaceSolid(downPos, cube.FaceUp, src) {
				return
			}
		}
		b.Hanging = face == cube.FaceDown
		place(placer, clickedBlockPos, pos, face, b)
	case block.Leaves:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Persistent = true
		place(placer, clickedBlockPos, pos, face, b)
	case block.Lectern:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Facing = placer.Rotation().Direction().Opposite()
		place(placer, clickedBlockPos, pos, face, b)
	case block.LilyPad:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if liq, ok := src.Block(pos.Side(cube.FaceDown)).(world.Liquid); !ok || liq.LiquidType() != "water" || liq.LiquidDepth() < 8 {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.LitPumpkin:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Facing = placer.Rotation().Direction().Opposite()
		place(placer, clickedBlockPos, pos, face, b)
	case block.Log:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Axis = face.Axis()
		place(placer, clickedBlockPos, pos, face, b)
	case block.Loom:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Facing = placer.Rotation().Direction().Opposite()
		place(placer, clickedBlockPos, pos, face, b)
	case block.MelonSeeds:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if _, ok := src.Block(pos.Side(cube.FaceDown)).(block.Farmland); !ok {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.MossCarpet:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if _, ok := src.Block(pos.Side(cube.FaceDown)).(block.Air); ok {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.MuddyMangroveRoots:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Axis = face.Axis()
		place(placer, clickedBlockPos, pos, face, b)
	case block.NetherSprouts:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if !supportsVegetation(b, src.Block(pos.Side(cube.FaceDown))) {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.NetherWart:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if _, ok := src.Block(pos.Side(cube.FaceDown)).(block.SoulSand); !ok {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.PinkPetals:
		if existing, ok := src.Block(replaceBlockPos).(block.PinkPetals); ok {
			if existing.AdditionalCount >= 3 {
				return
			}
			existing.AdditionalCount++
			place(placer, clickedBlockPos, replaceBlockPos, face, existing)
			return
		}

		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if !supportsVegetation(b, src.Block(pos.Side(cube.FaceDown))) {
			return
		}
		b.Facing = placer.Rotation().Direction().Opposite()
		place(placer, clickedBlockPos, pos, face, b)
	case block.Potato:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if _, ok := src.Block(pos.Side(cube.FaceDown)).(block.Farmland); !ok {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.PumpkinSeeds:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if _, ok := src.Block(pos.Side(cube.FaceDown)).(block.Farmland); !ok {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.Pumpkin:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Facing = placer.Rotation().Direction().Opposite()
		place(placer, clickedBlockPos, pos, face, b)
	case block.PurpurPillar:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Axis = face.Axis()
		place(placer, clickedBlockPos, pos, face, b)
	case block.QuartzPillar:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Axis = face.Axis()
		place(placer, clickedBlockPos, pos, face, b)
	case block.SeaPickle:
		if existing, ok := src.Block(replaceBlockPos).(block.SeaPickle); ok {
			if existing.AdditionalCount >= 3 {
				return
			}
			existing.AdditionalCount++
			place(placer, clickedBlockPos, replaceBlockPos, face, existing)
			return
		}
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if !seaPickleCanSurvive(pos, src) {
			return
		}
		b.Dead = true
		if liquid, ok := src.Block(pos).(world.Liquid); ok {
			_, isWater := liquid.(block.Water)
			b.Dead = !isWater
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.ShortGrass:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used || !supportsVegetation(b, src.Block(pos.Side(cube.FaceDown))) {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.Sign:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used || face == cube.FaceDown {
			return
		}
		if face == cube.FaceUp {
			b.Attach = block.StandingAttachment(placer.Rotation().Orientation().Opposite())
		} else {
			b.Attach = block.WallAttachment(face.Direction())
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.Skull:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used || face == cube.FaceDown {
			return
		}
		if face == cube.FaceUp {
			b.Attach = block.StandingAttachment(placer.Rotation().Orientation().Opposite())
		} else {
			b.Attach = block.WallAttachment(face.Direction())
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.Slab:
		id, meta := b.EncodeItem()
		clickedBlock := src.Block(replaceBlockPos)
		if clickedSlab, ok := clickedBlock.(block.Slab); ok && !b.Double {
			clickedId, clickedMeta := clickedSlab.EncodeItem()
			if !clickedSlab.Double && id == clickedId && meta == clickedMeta &&
				((face == cube.FaceUp && !clickedSlab.Top) || (face == cube.FaceDown && clickedSlab.Top)) {
				clickedSlab.Double = true
				place(placer, clickedBlockPos, replaceBlockPos, face, clickedSlab)
				return
			}
		}
		if sideSlab, ok := src.Block(replaceBlockPos.Side(face)).(block.Slab); ok && !replaceableWith(src, replaceBlockPos, b) && !b.Double {
			sideId, sideMeta := sideSlab.EncodeItem()
			if !sideSlab.Double && id == sideId && meta == sideMeta {
				sideSlab.Double = true
				place(placer, clickedBlockPos, replaceBlockPos.Side(face), face, sideSlab)
				return
			}
		}
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if face == cube.FaceDown || (clickPos[1] > 0.5 && face != cube.FaceUp) {
			b.Top = true
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.Smoker:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		place(placer, clickedBlockPos, pos, face, block.NewSmoker(placer.Rotation().Direction().Opposite()))
	case block.Sponge:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.SporeBlossom:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if !src.Block(pos.Side(cube.FaceUp)).Model().FaceSolid(pos.Side(cube.FaceUp), cube.FaceDown, src) {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.Stairs:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Facing = placer.Rotation().Direction()
		if face == cube.FaceDown || (clickPos[1] > 0.5 && face != cube.FaceUp) {
			b.UpsideDown = true
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.Stonecutter:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Facing = placer.Rotation().Direction().Opposite()
		place(placer, clickedBlockPos, pos, face, b)
	case block.SugarCane:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used || !canSugarCaneGrowOn(pos, src, true) {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.Torch:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if face == cube.FaceDown {
			return
		}
		if !src.Block(pos.Side(face.Opposite())).Model().FaceSolid(pos.Side(face.Opposite()), face, src) {
			found := false
			for _, i := range []cube.Face{cube.FaceNorth, cube.FaceSouth, cube.FaceEast, cube.FaceWest} {
				if src.Block(pos.Side(i)).Model().FaceSolid(pos.Side(i), face, src) {
					found = true
					face = i.Opposite()
					break
				}
			}
			if !found {
				return
			}
		}
		b.Facing = face.Opposite()
		place(placer, clickedBlockPos, pos, face, b)
	case block.Vines:
		if _, ok := src.Block(replaceBlockPos).Model().(model.Solid); !ok || face.Axis() == cube.Y {
			return
		}
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if _, ok := src.Block(pos).(block.Vines); ok {
			return
		}
		b = b.WithAttachment(face.Direction().Opposite(), true)
		place(placer, clickedBlockPos, pos, face, b)
	case block.Wall:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		// TODO: calculate connections and posts and blah blah blah FML i will do it in ANOTHER COMMIT ON ANOTHER DAY
		// because if I am doubling down doing this WITHOUT CURSOR and I AM GOING TO CRASH OUT FOR FUCKS SAKE.
		place(placer, clickedBlockPos, pos, face, b)
	case block.WheatSeeds:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		if _, ok := src.Block(pos.Side(cube.FaceDown)).(block.Farmland); !ok {
			return
		}
		place(placer, clickedBlockPos, pos, face, b)
	case block.WoodDoor:
		if face != cube.FaceUp {
			return
		}
		below := replaceBlockPos
		replaceBlockPos = replaceBlockPos.Side(cube.FaceUp)
		if !replaceableWith(src, replaceBlockPos, b) || !replaceableWith(src, replaceBlockPos.Side(cube.FaceUp), b) {
			return
		}
		if !src.Block(below).Model().FaceSolid(below, cube.FaceUp, src) {
			return
		}
		b.Facing = placer.Rotation().Direction()
		left := src.Block(replaceBlockPos.Side(b.Facing.RotateLeft().Face()))
		right := src.Block(replaceBlockPos.Side(b.Facing.RotateRight().Face()))
		if _, ok := left.Model().(model.Door); ok {
			b.Right = true
		}
		if diffuser, ok := right.(block.LightDiffuser); !ok || diffuser.LightDiffusionLevel() != 0 {
			if diffuser, ok := left.(block.LightDiffuser); ok && diffuser.LightDiffusionLevel() == 0 {
				b.Right = true
			}
		}
		place(placer, clickedBlockPos, replaceBlockPos, face, b)
		place(placer, clickedBlockPos, replaceBlockPos.Side(cube.FaceUp), cube.FaceUp, block.WoodDoor{Facing: b.Facing, Top: true, Right: b.Right})
	case block.WoodFenceGate:
		pos, _, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Facing = placer.Rotation().Direction()

		leftSide := b.Facing.RotateLeft().Face()
		_, left := src.Block(pos.Side(leftSide)).(block.Wall)
		_, right := src.Block(pos.Side(leftSide.Opposite())).(block.Wall)
		b.Lowered = left || right
		place(placer, clickedBlockPos, pos, face, b)
	case block.WoodTrapdoor:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Facing = placer.Rotation().Direction().Opposite()
		b.Top = (clickPos[1] > 0.5 && face != cube.FaceUp) || face == cube.FaceDown
		place(placer, clickedBlockPos, pos, face, b)
	case block.Wood:
		pos, face, used := firstReplaceable(src, replaceBlockPos, face, b)
		if !used {
			return
		}
		b.Axis = face.Axis()
		place(placer, clickedBlockPos, pos, face, b)
		// TODO: add neccessary item interactions that do affect the world state (e.g. - water/lava buckets)
	}
}

func place(p blockPlacer, clickedBlockPos, replaceBlockPos cube.Pos, face cube.Face, b world.Block) {
	p.PlaceBlock(clickedBlockPos, replaceBlockPos, face, b)
}

func canSugarCaneGrowOn(pos cube.Pos, src *oworld.World, recursive bool) bool {
	if _, ok := src.Block(pos.Side(cube.FaceDown)).(block.SugarCane); ok && recursive {
		return canSugarCaneGrowOn(pos.Side(cube.FaceDown), src, recursive)
	}

	if supportsVegetation(block.SugarCane{}, src.Block(pos.Sub(cube.Pos{0, 1}))) {
		for _, face := range cube.HorizontalFaces() {
			if liquid, ok := src.Block(pos.Side(face).Side(cube.FaceDown)).(world.Liquid); ok {
				if _, ok := liquid.(block.Water); ok {
					return true
				}
			}
		}
	}
	return false
}

func seaPickleCanSurvive(pos cube.Pos, src *oworld.World) bool {
	below := src.Block(pos.Side(cube.FaceDown))
	if !below.Model().FaceSolid(pos.Side(cube.FaceDown), cube.FaceUp, src) {
		return false
	}
	if liquid, ok := src.Block(pos).(world.Liquid); ok {
		if _, ok := liquid.(block.Water); !ok || liquid.LiquidDepth() < 8 {
			return false
		}
	}
	if emitter, ok := below.(block.LightDiffuser); ok && emitter.LightDiffusionLevel() != 15 {
		return false
	}
	return true
}

func canCactusGrowOn(pos cube.Pos, src *oworld.World, recursive bool) bool {
	for _, face := range cube.HorizontalFaces() {
		if _, ok := src.Block(pos.Side(face)).(block.Air); !ok {
			return false
		}
	}
	if _, ok := src.Block(pos.Side(cube.FaceDown)).(block.Cactus); ok && recursive {
		return canCactusGrowOn(pos.Side(cube.FaceDown), src, recursive)
	}
	return supportsVegetation(block.Cactus{}, src.Block(pos.Side(cube.FaceDown)))
}

func supportsVegetation(vegetation, b world.Block) bool {
	soil, ok := b.(block.Soil)
	return ok && soil.SoilFor(vegetation)
}

func calculateFace(user blockPlacer, placePos cube.Pos) cube.Face {
	userPos := user.Position()
	pos := cube.PosFromVec3(userPos)
	if abs(pos[0]-placePos[0]) < 2 && abs(pos[2]-placePos[2]) < 2 {
		y := userPos[1]
		if eyed, ok := user.(interface{ EyeHeight() float64 }); ok {
			y += eyed.EyeHeight()
		}

		if y-float64(placePos[1]) > 2.0 {
			return cube.FaceUp
		} else if float64(placePos[1])-y > 0.0 {
			return cube.FaceDown
		}
	}
	return user.Rotation().Direction().Opposite().Face()
}

func abs(x int) int {
	if x > 0 {
		return x
	}
	return -x
}

func replaceableWith(src *oworld.World, pos cube.Pos, with world.Block) bool {
	// TODO: Account for other dimensions.
	if pos.OutOfBounds(world.Overworld.Range()) {
		return false
	}
	b := src.Block(pos)
	if replaceable, ok := b.(block.Replaceable); ok {
		if !replaceable.ReplaceableBy(with) || b == with {
			return false
		}
		// TODO: Check liquids on other layers.
		if liquid, ok := b.(world.Liquid); ok {
			replaceable, ok := liquid.(block.Replaceable)
			return ok && replaceable.ReplaceableBy(with)
		}
		return true
	}
	return false
}

func firstReplaceable(src *oworld.World, pos cube.Pos, face cube.Face, with world.Block) (cube.Pos, cube.Face, bool) {
	if replaceableWith(src, pos, with) {
		// A replaceableWith block was clicked, so we can replace it. This will then be assumed to be placed on
		// the top face. (Torches, for example, will get attached to the floor when clicking tall grass.)
		return pos, cube.FaceUp, true
	}
	side := pos.Side(face)
	if replaceableWith(src, side, with) {
		return side, face, true
	}
	return pos, face, false
}
