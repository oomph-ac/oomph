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
	rotationSamples = 150
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

	d.FailBuffer = 1
	d.MaxBuffer = 1

	d.rotations = make([]float32, rotationSamples)
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
	if mDat.HorizontallyCollided || math32.Abs(mDat.Rotation.X()) >= 89 || mDat.TicksSinceTeleport <= 1 {
		return true
	}

	yawDelta := mDat.DeltaRotation.Z()
	if yawDelta == 0 {
		return true
	}

	for i := 0; i < d.rotationCount; i++ {
		if math32.Abs(d.rotations[i]-yawDelta) <= 1e-4 {
			return true
		}
	}
	d.rotations[d.rotationCount] = yawDelta
	d.rotationCount++

	if d.rotationCount == rotationSamples {
		rots := make([]float32, len(d.rotations))
		copy(rots, d.rotations)
		slices.Sort(rots)

		slopes := make([]float32, len(rots)-2)
		for i := 0; i < len(rots)-2; i++ {
			slopes[i] = rots[i+1] - rots[i]
		}
		slices.Sort(slopes)
		bSlope, matchAmt := d.mostFrequentSlope(slopes)

		p.Dbg.Notify(player.DebugModeAimA, true, "bestSlope=%f matchAmt=%d", bSlope, matchAmt)

		if bSlope < 0.007 && matchAmt <= 4 {
			data := orderedmap.NewOrderedMap[string, any]()
			data.Set("bSl", game.Round32(bSlope, 5))
			data.Set("amt", matchAmt)
			d.Fail(p, data)
			d.rotationCount = 0
			return true
		} else {
			d.Buffer = 0
		}

		d.rotationCount--
		for i := 1; i < rotationSamples; i++ {
			d.rotations[i-1] = d.rotations[i]
		}
	}

	return true
}

func (d *AimA) mostFrequentSlope(slopes []float32) (float32, int) {
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
