package detection

import (
	"slices"

	"github.com/chewxy/math32"
	"github.com/elliotchance/orderedmap/v2"
	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	DetectionIDAimA = "oomph:aim_a"
)

type AimA struct {
	rotations     []float32
	rotationCount int

	BaseDetection
}

func NewAimA() *AimA {
	d := &AimA{}
	d.Type = "Aim"
	d.SubType = "A"

	d.Description = "Checks for an inconsistent difference between player rotations."
	d.Punishable = true

	d.MaxViolations = 10
	d.trustDuration = -1

	d.FailBuffer = 5
	d.MaxBuffer = 5

	d.rotations = make([]float32, 100)
	d.rotationCount = 0
	return d
}

func (AimA) ID() string {
	return DetectionIDAimA
}

func (d *AimA) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	input, ok := pk.(*packet.PlayerAuthInput)
	if !ok {
		return true
	}

	// This check will only apply to players rotating their camera with a mouse.
	if input.InputMode != packet.InputModeMouse {
		return true
	}

	mDat := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	yawDelta := game.Round32(math32.Abs(mDat.DeltaRotation.Z()), 5)
	if yawDelta < 1e-4 || yawDelta >= 180 {
		return true
	}

	for _, r := range d.rotations {
		if math32.Abs(r-yawDelta) <= 1e-4 {
			return true
		}
	}
	d.rotations[d.rotationCount] = yawDelta
	d.rotationCount++

	if d.rotationCount == 100 {
		var rotations = make([]float32, len(d.rotations))
		copy(rotations, d.rotations)
		slices.Sort(rotations)

		bSlope, matchAmt := d.determineBestSlope(rotations)
		p.Dbg.Notify(player.DebugModeRotations, true, "bestSlope=%f matchAmt=%d", bSlope, matchAmt)

		if bSlope < 0.2 && matchAmt <= 5 {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("bSl", game.Round32(bSlope, 5))
			data.Set("amt", matchAmt)
			d.Fail(p, data)
		} else {
			d.Buffer = 0
		}

		for i := 1; i < len(d.rotations); i++ {
			d.rotations[i-1] = d.rotations[i]
		}
		d.rotationCount--
	}

	return true
}

func (d *AimA) determineBestSlope(rotations []float32) (float32, int) {
	// slope:similarity count
	var slopesMatchCount = map[float32]int{}

	for i := 0; i < len(rotations)-1; i++ {
		slope := rotations[i+1] - rotations[i]
		if _, ok := slopesMatchCount[slope]; ok {
			continue
		}

		slopesMatchCount[slope] = 0
		for j := 0; j < len(rotations)-1; j++ {
			if j == i {
				continue
			}

			currentSlope := rotations[j+1] - rotations[j]
			if currentSlope > slope && math32.Mod(currentSlope, slope) < slope*0.01 ||
				currentSlope < slope && math32.Mod(slope, currentSlope) < currentSlope*0.01 {
				slopesMatchCount[slope]++
			}
		}
	}

	var bestSlope float32
	for slope, count := range slopesMatchCount {
		if bestSlope == 0 || count > slopesMatchCount[bestSlope] || (count == slopesMatchCount[bestSlope] && slope > bestSlope) {
			bestSlope = slope
		}
	}

	return bestSlope, slopesMatchCount[bestSlope]
}
