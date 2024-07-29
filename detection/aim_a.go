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

	d.FailBuffer = 2
	d.MaxBuffer = 4

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
	if mDat.HorizontallyCollided || math32.Abs(mDat.Rotation.X()) >= 89 { // why does this always false ROTATION checks??!!!
		return true
	}

	if mDat.TicksSinceTeleport <= 1 {
		d.rotationCount = 0
		return true
	}

	yawDelta := game.Round32(math32.Abs(mDat.DeltaRotation.Z()), 5)
	if yawDelta < 0.001 || yawDelta >= 180 {
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

		if matchAmt <= 5 {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("bSl", game.Round32(bSlope, 5))
			data.Set("amt", matchAmt)
			d.Fail(p, data)
		} else {
			d.Buffer = 0
		}

		d.rotationCount = 0
	}

	return true
}

func (d *AimA) determineBestSlope(rotations []float32) (float32, int) {
	slopes := make([]float32, len(rotations)-2)
	for i := 0; i < len(rotations)-2; i++ {
		slopes[i] = rotations[i+1] - rotations[i]
	}
	slices.Sort(slopes)

	var (
		bestSlope    float32
		currentSlope float32 = math32.MaxFloat32 - 1

		bestCount, currentCount int
	)

	for _, slope := range slopes {
		if math32.Abs(slope-currentSlope) <= 1e-4 {
			currentCount++
		} else {
			currentSlope = slope
			currentCount = 1
		}

		if currentCount > bestCount {
			bestCount = currentCount
			bestSlope = currentSlope
		}
	}

	return bestSlope, bestCount
}
