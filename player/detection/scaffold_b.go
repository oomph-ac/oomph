package detection

import (
	"github.com/df-mc/dragonfly/server/world"
	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

var (
	faceNotSet  cube.Face = -1
	faceNotInit cube.Face = -2
)

type ScaffoldB struct {
	mPlayer  *player.Player
	metadata *player.DetectionMetadata

	initialFace cube.Face
}

func New_ScaffoldB(p *player.Player) *ScaffoldB {
	return &ScaffoldB{
		mPlayer: p,
		metadata: &player.DetectionMetadata{
			FailBuffer: 1.01,
			MaxBuffer:  1.5,

			MaxViolations: 10,
		},
		initialFace: faceNotInit,
	}
}

func (d *ScaffoldB) Type() string {
	return TypeScaffold
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
	if !ok || dat.ActionType != protocol.UseItemActionClickBlock || !d.mPlayer.VersionInRange(player.GameVersion1_21_20, protocol.CurrentProtocol) {
		return
	}
	inHand := d.mPlayer.Inventory().Holding()
	if _, isBlock := inHand.Item().(world.Block); !isBlock {
		return
	}

	// We have to check this regardless of whether the client predicted the interaction failed or not - otherwise we get false positives when
	// checking during when the trigger type is of TriggerTypeSimulationTick (player frame).
	blockFace := cube.Face(dat.BlockFace)
	if dat.TriggerType == protocol.TriggerTypePlayerInput {
		d.initialFace = faceNotSet
	} else if d.initialFace == faceNotSet && blockFace != cube.FaceUp && blockFace != cube.FaceDown {
		d.initialFace = blockFace
	} else if d.initialFace == faceNotInit {
		d.mPlayer.Log().Debug("scaffold_b", "initFace", "faceNotInit", "face", blockFace)
		d.mPlayer.FailDetection(d, nil)
	}
	if dat.ClientPrediction == protocol.ClientPredictionFailure {
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
	blockPos := cube.Pos{int(dat.BlockPosition[0]), int(dat.BlockPosition[1]), int(dat.BlockPosition[2])}
	if !d.isFaceInteractable(prevEyePos, currEyePos, blockPos, blockFace, dat.TriggerType == protocol.TriggerTypePlayerInput) {
		d.mPlayer.FailDetection(d, nil)
	} else {
		d.mPlayer.PassDetection(d, 0.5)
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
	floorPosStart := cube.PosFromVec3(startPos)
	floorPosEnd := cube.PosFromVec3(endPos)

	if !isClientInput {
		interactableFaces[cube.FaceDown] = struct{}{}
		interactableFaces[cube.FaceUp] = struct{}{}
		if d.initialFace != faceNotSet {
			interactableFaces[d.initialFace] = struct{}{}
			interactableFaces[d.initialFace.Opposite()] = struct{}{}
		}
	} else {
		// Check for the Y-axis faces first.
		// If floor(eyePos.Y) < blockPos.Y -> the bottom face is interactable.
		// If floor(eyePos.Y) > blockPos.Y -> the top face is interactable.
		isBelowBlock := floorPosStart[1] < blockY || floorPosEnd[1] < blockY
		isAboveBlock := floorPosStart[1] > blockY || floorPosEnd[1] > blockY
		isOnBlock := floorPosStart[1] == blockY+2 || floorPosEnd[1] == blockY+2
		if isBelowBlock {
			interactableFaces[cube.FaceDown] = struct{}{}
		}
		if isAboveBlock {
			interactableFaces[cube.FaceUp] = struct{}{}
			if isOnBlock {
				startXDelta := game.AbsNum(floorPosStart[0] - blockX)
				endXDelta := game.AbsNum(floorPosEnd[0] - blockX)
				if startXDelta <= 1 || endXDelta <= 1 {
					interactableFaces[cube.FaceWest] = struct{}{}
					interactableFaces[cube.FaceEast] = struct{}{}
				}

				startZDelta := game.AbsNum(floorPosStart[2] - blockZ)
				endZDelta := game.AbsNum(floorPosEnd[2] - blockZ)
				if startZDelta <= 1 || endZDelta <= 1 {
					interactableFaces[cube.FaceNorth] = struct{}{}
					interactableFaces[cube.FaceSouth] = struct{}{}
				}
			}
			//fmt.Println(isOnBlock, floorPosStart[1], floorPosEnd[1], blockY)
		}

		// Check for the X-axis faces.
		// If floor(eyePos.X) < blockPos.X -> the west face is interactable.
		// If floor(eyePos.X) > blockPos.X -> the east face is interactable.
		if floorPosStart[0] < blockX || floorPosEnd[0] < blockX {
			interactableFaces[cube.FaceWest] = struct{}{}
		}
		if floorPosStart[0] > blockX || floorPosEnd[0] > blockX {
			interactableFaces[cube.FaceEast] = struct{}{}
		}

		// Check for the Z-axis faces.
		// If floor(eyePos.Z) < blockPos.Z -> the north face is interactable.
		// If floor(eyePos.Z) > blockPos.Z -> the south face is interactable.
		if floorPosStart[2] < blockZ || floorPosEnd[2] < blockZ {
			interactableFaces[cube.FaceNorth] = struct{}{}
		}
		if floorPosStart[2] > blockZ || floorPosEnd[2] > blockZ {
			interactableFaces[cube.FaceSouth] = struct{}{}
		}
	}

	_, interactable := interactableFaces[targetFace]
	if !interactable {
		d.mPlayer.Log().Debug("scaffold_b", "blockPos", blockPos, "startPos", startPos, "endPos", endPos, "isClientInput", isClientInput, "targetFace", targetFace, "interactableFaces", interactableFaces)
	}
	return interactable
}
