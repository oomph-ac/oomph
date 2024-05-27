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
	if dat.BlockFace != d.startPlacementFace && delay == 0 {
		d.Fail(p, nil)
	}

	return true
}
