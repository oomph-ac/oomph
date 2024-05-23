package detection

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const DetectionIDScaffoldA = "oomph:scaffold_a"

type ScaffoldA struct {
	BaseDetection
	startPlacementFace int32
	lastPlacementTick  int64
}

func NewScaffoldA() *ScaffoldA {
	d := &ScaffoldA{}
	d.Type = "Scaffold"
	d.SubType = "A"

	d.Description = "Checks for invalid zero-vector click positions whilst placing blocks."
	d.Punishable = true

	d.MaxViolations = 5
	d.trustDuration = player.TicksPerSecond * 10

	d.FailBuffer = 0
	d.MaxBuffer = 1

	d.startPlacementFace = -1
	return d
}

func (d *ScaffoldA) ID() string {
	return DetectionIDScaffoldA
}

func (d *ScaffoldA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	i, ok := pk.(*packet.InventoryTransaction)
	if !ok {
		return true
	}

	dat, ok := i.TransactionData.(*protocol.UseItemTransactionData)
	if !ok {
		return true
	}
	if dat.ActionType != protocol.UseItemActionClickBlock {
		return true
	}

	defer func() {
		d.lastPlacementTick = p.ClientFrame
	}()

	if dat.ClickedPosition != utils.EmptyVec32 {
		d.startPlacementFace = dat.BlockFace
		return true
	}

	delay := p.ClientFrame - d.lastPlacementTick
	if dat.BlockFace != d.startPlacementFace && delay <= 5 {
		p.Message("%v %v (delay=%v)", dat.BlockFace, d.startPlacementFace, delay)
	}

	return true
}

/* func (d *ScaffoldA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	_, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	placements := p.Handler(handler.HandlerIDChunks).(*handler.ChunksHandler).BlockPlacements
	if len(placements) == 0 {
		return true
	}

	defer func() {
		d.lastPlacementTick = p.ClientFrame
	}()

	// A zero-vector click position only occurs when the player is jump bridging. This means that the player would
	// have to be going in one direction whilst bridging.
	// A zero-vec click position will not happen if the player places a block in a different direction than
	// the player started from.

	for _, pl := range placements {
		// If the clicked position is not a zero vector, then it is not a result of jump bridging.
		if pl.RawData.ClickedPosition != utils.EmptyVec32 {
			d.startPlacementFace = pl.RawData.BlockFace
			continue
		}

		delay := p.ClientFrame - d.lastPlacementTick
		if pl.RawData.BlockFace != d.startPlacementFace && delay <= 5 {
			data := orderedmap.NewOrderedMap[string, any]()
			d.Fail(p, data)
		}
	}

	return true
} */
