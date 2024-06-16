package detection

import "github.com/oomph-ac/oomph/player"

// RegisterDetections registers all detections with the given player.
func RegisterDetections(p *player.Player) {
	// aim detections
	p.RegisterDetection(NewAimA())
	p.RegisterDetection(NewAimB())

	// hitbox detections
	p.RegisterDetection(NewHitboxA())

	// reach detections
	p.RegisterDetection(NewReachA())
	p.RegisterDetection(NewReachB())

	// auto-clicker detections
	p.RegisterDetection(NewAutoClickerA())
	p.RegisterDetection(NewAutoClickerB())
	//p.RegisterDetection(NewAutoClickerC())

	// kill aura detections
	p.RegisterDetection(NewKillAuraA())

	// fly detections
	p.RegisterDetection(NewFlyA())

	// speed detections
	p.RegisterDetection(NewSpeedA())

	// (invalid) motion detections
	p.RegisterDetection(NewMotionA())
	p.RegisterDetection(NewMotionB())
	p.RegisterDetection(NewMotionC())

	// velocity (aka. anti-kb) detections
	p.RegisterDetection(NewVelocityA())
	p.RegisterDetection(NewVelocityB())

	// timer detections
	p.RegisterDetection(NewTimerA())

	// badpacket detections
	p.RegisterDetection(NewBadPacketA())
	p.RegisterDetection(NewBadPacketB())
	p.RegisterDetection(NewBadPacketC())

	// edition faker detections
	p.RegisterDetection(NewEditionFakerA())
	p.RegisterDetection(NewEditionFakerB())

	// server nuke detections
	p.RegisterDetection(NewServerNukeA())
	p.RegisterDetection(NewServerNukeB())

	// scaffold detections
	p.RegisterDetection(NewScaffoldA())
}
