package detection

import (
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var faceNotSet cube.Face = -1

type ScaffoldB struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	initialFace cube.Face
}

func New_ScaffoldB(p *player.Player) *ScaffoldB {
	return &ScaffoldB{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			MaxViolations: 10,
		},
		initialFace: faceNotSet,
	}
}

func (d *ScaffoldB) Type() string {
	return "Scaffold"
}

func (d *ScaffoldB) SubType() string {
	return "B"
}

func (d *ScaffoldB) Description() string {
	return "Checks if the block face the player is placing against is valid."
}

func (d *ScaffoldB) Punishable() bool {
	return true
}

func (d *ScaffoldB) Metadata() *player.DetectionMetadata {
	return d.metadata
}

func (d *ScaffoldB) Detect(pk packet.Packet) {
	tr, ok := pk.(*packet.InventoryTransaction)
	if !ok {
		return
	}

	dat, ok := tr.TransactionData.(*protocol.UseItemTransactionData)
	if !ok || dat.ActionType != protocol.UseItemActionClickBlock || dat.ClientPrediction == protocol.ClientPredictionFailure || !d.mPlayer.VersionInRange(player.GameVersion1_21_20, protocol.CurrentProtocol) {
		return
	}

	eyeOffset := game.DefaultPlayerHeightOffset
	if d.mPlayer.Movement().Sneaking() {
		eyeOffset = game.SneakingPlayerHeightOffset
	}
	prevEyePos := d.mPlayer.Movement().Client().LastPos()
	currEyePos := d.mPlayer.Movement().Client().Pos()
	prevEyePos[1] += eyeOffset
	currEyePos[1] += eyeOffset
	blockFace := cube.Face(dat.BlockFace)
	if dat.TriggerType == protocol.TriggerTypePlayerInput {
		d.initialFace = faceNotSet
	} else if d.initialFace == faceNotSet {
		d.initialFace = blockFace
	}
	blockPos := cube.Pos{int(dat.BlockPosition[0]), int(dat.BlockPosition[1]), int(dat.BlockPosition[2])}
	if !d.isFaceInteractable(prevEyePos, currEyePos, blockPos, blockFace, dat.TriggerType == protocol.TriggerTypePlayerInput) {
		d.mPlayer.FailDetection(d, nil)
	}
}

func (d *ScaffoldB) isFaceInteractable(
	startPos,
	endPos mgl32.Vec3,
	blockPos cube.Pos,
	targetFace cube.Face,
	isClientInput bool,
) bool {
	interactableFaces := make(map[cube.Face]struct{}, 6)
	blockX, blockY, blockZ := blockPos[0], blockPos[1], blockPos[2]

	if !isClientInput {
		interactableFaces[cube.FaceDown] = struct{}{}
		interactableFaces[cube.FaceUp] = struct{}{}
		interactableFaces[d.initialFace] = struct{}{}
		interactableFaces[d.initialFace.Opposite()] = struct{}{}
	} else {
		yFloorStart := int(startPos[1])
		yFloorEnd := int(endPos[1])
		xFloorStart := int(startPos[0])
		xFloorEnd := int(endPos[0])
		zFloorStart := int(startPos[2])
		zFloorEnd := int(endPos[2])

		// Check for the Y-axis faces first.
		// If floor(eyePos.Y) < blockPos.Y -> the bottom face is interactable.
		// If floor(eyePos.Y) > blockPos.Y -> the top face is interactable.
		isBelowBlock := yFloorStart < blockY || yFloorEnd < blockY
		isAboveBlock := yFloorStart > blockY || yFloorEnd > blockY
		if isBelowBlock {
			interactableFaces[cube.FaceDown] = struct{}{}
		}
		if isAboveBlock {
			interactableFaces[cube.FaceUp] = struct{}{}

			startXDelta := game.AbsNum(xFloorStart - blockX)
			endXDelta := game.AbsNum(xFloorEnd - blockX)
			if startXDelta <= 1 || endXDelta <= 1 {
				interactableFaces[cube.FaceWest] = struct{}{}
				interactableFaces[cube.FaceEast] = struct{}{}
			}

			startZDelta := game.AbsNum(zFloorStart - blockZ)
			endZDelta := game.AbsNum(zFloorEnd - blockZ)
			if startZDelta <= 1 || endZDelta <= 1 {
				interactableFaces[cube.FaceNorth] = struct{}{}
				interactableFaces[cube.FaceSouth] = struct{}{}
			}
		}

		// Check for the X-axis faces.
		// If floor(eyePos.X) < blockPos.X -> the west face is interactable.
		// If floor(eyePos.X) > blockPos.X -> the east face is interactable.
		/* if (xFloorStart == blockX || xFloorEnd == blockX) && isBelowBlock {
			interactableFaces[cube.FaceWest] = struct{}{}
			interactableFaces[cube.FaceEast] = struct{}{}
		} else {
			if xFloorStart < blockX || xFloorEnd < blockX {
				interactableFaces[cube.FaceWest] = struct{}{}
			}
			if xFloorStart > blockX || xFloorEnd > blockX {
				interactableFaces[cube.FaceEast] = struct{}{}
			}
		} */
		if xFloorStart <= blockX || xFloorEnd <= blockX {
			interactableFaces[cube.FaceWest] = struct{}{}
		}
		if xFloorStart >= blockX || xFloorEnd >= blockX {
			interactableFaces[cube.FaceEast] = struct{}{}
		}

		// Check for the Z-axis faces.
		// If floor(eyePos.Z) < blockPos.Z -> the north face is interactable.
		// If floor(eyePos.Z) > blockPos.Z -> the south face is interactable.
		if zFloorStart <= blockZ || zFloorEnd <= blockZ {
			interactableFaces[cube.FaceNorth] = struct{}{}
		}
		if zFloorStart >= blockZ || zFloorEnd >= blockZ {
			interactableFaces[cube.FaceSouth] = struct{}{}
		}
		/* if (zFloorStart == blockZ || zFloorEnd == blockZ) && isBelowBlock {
			interactableFaces[cube.FaceNorth] = struct{}{}
			interactableFaces[cube.FaceSouth] = struct{}{}
		} else {
			if zFloorStart < blockZ || zFloorEnd < blockZ {
				interactableFaces[cube.FaceNorth] = struct{}{}
			}
			if zFloorStart > blockZ || zFloorEnd > blockZ {
				interactableFaces[cube.FaceSouth] = struct{}{}
			}
		} */
	}

	_, interactable := interactableFaces[targetFace]
	//fmt.Println(blockPos, cube.PosFromVec3(startPos), cube.PosFromVec3(endPos), isClientInput, targetFace, interactableFaces, interactable)
	return interactable
}
