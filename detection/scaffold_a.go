package detection

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDScaffoldA = "oomph:scaffold_a"

type ScaffoldA struct {
	BaseDetection

	// lane is the expected placement lane of the player oomph detects jump bridging
	lane placementLane

	expectedBlockFace int32
	lastPlacementTick int64
}

type placementLane struct {
	axis  cube.Axis
	value int
}

func NewScaffoldA() *ScaffoldA {
	d := &ScaffoldA{}
	d.Type = "Scaffold"
	d.SubType = "A"

	d.Description = "Checks for (two) invalid block placement scenarios."
	d.Punishable = true

	d.MaxViolations = 5
	d.trustDuration = -1

	d.FailBuffer = 0
	d.MaxBuffer = 1

	d.expectedBlockFace = -2
	d.lastPlacementTick = 0
	d.lane = placementLane{}

	return d
}

func (d *ScaffoldA) ID() string {
	return DetectionIDScaffoldA
}

func (d *ScaffoldA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	switch pk := pk.(type) {
	case *packet.InventoryTransaction:
		// It is possible that the player now is holding right click, which means that the next block placement
		// may have a zero-vector click position. We will set the expected block face to the next block face
		// that is clicked.
		if dat, ok := pk.TransactionData.(*protocol.UseItemTransactionData); ok && dat.ActionType == protocol.UseItemActionClickAir {
			d.expectedBlockFace = -1
		}
	case *packet.PlayerAuthInput:
		placements := p.Handler(handler.HandlerIDChunks).(*handler.ChunksHandler).BlockPlacements
		if len(placements) == 0 {
			return true
		}

		mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
		for _, pl := range placements {
			blockPlacedBelow := (pl.Position.Y() == int(mDat.ClientPosition.Y()-1))
			placedFace := cube.Face(pl.RawData.BlockFace)

			// Update the expected block face and the "building lane".
			if pl.RawData.ClickedPosition != utils.EmptyVec32 || d.expectedBlockFace == -1 {
				if d.expectedBlockFace != -1 {
					d.lane.axis = placedFace.Axis()
					d.lane.value = pl.Position.X()
					if placedFace.Axis() == cube.X {
						d.lane.value = pl.Position.Z()
					}
				}
				d.expectedBlockFace = pl.RawData.BlockFace

				continue
			}

			if pl.RawData.BlockFace != d.expectedBlockFace && blockPlacedBelow {
				// This behavior usually occurs with scaffolds due to the fact they are not done properly. This check
				// can be easily bypassed - but cheat developers must figure out what Oomph is doing here :^)
				data := orderedmap.NewOrderedMap[string, any]()
				data.Set("type", "WBF")
				d.Fail(p, data)

				p.Log().Infof("ScaffoldA WBF: expected=%v got=%v", d.expectedBlockFace, pl.RawData.BlockFace)
			} else if pl.RawData.BlockFace == d.expectedBlockFace && d.lane.axis != cube.Y {
				// Since the clicked position is still a zero-vector, we are assuming the player is jump bridging here.
				// Jump bridging cannot be done on two different axis at the same time, therefore, we validate that the
				// player is continuing a jump bridge on the same axis at the same value.
				var value int
				switch d.lane.axis {
				case cube.X:
					value = pl.Position.Z()
				case cube.Z:
					value = pl.Position.X()
				}

				if value != d.lane.value {
					data := orderedmap.NewOrderedMap[string, any]()
					data.Set("type", "IBL")
					d.Fail(p, data)
					p.Log().Infof("ScaffoldA IBL: expected=%v got=%v (axis=%v)", d.lane.value, value, d.lane.axis.String())
				}
			}
		}

		d.lastPlacementTick = p.ClientFrame
	}

	return true
}
