package detection

import "github.com/oomph-ac/oomph/player"

// RegisterDetections registers all detections with the given player.
func RegisterDetections(p *player.Player) {
	p.RegisterDetection(NewReachA())
	p.RegisterDetection(NewReachB())

	p.RegisterDetection(NewKillAuraA())

	p.RegisterDetection(NewMovementA())
	p.RegisterDetection(NewMovementB())
	p.RegisterDetection(NewMovementC())

	p.RegisterDetection(NewVelocityA())
	p.RegisterDetection(NewVelocityB())

	p.RegisterDetection(NewTimerA())

	p.RegisterDetection(NewBadPacketA())
	p.RegisterDetection(NewBadPacketB())
	p.RegisterDetection(NewBadPacketC())

	p.RegisterDetection(NewAutoClickerA())
	p.RegisterDetection(NewAutoClickerB())
	p.RegisterDetection(NewAutoClickerC())

	p.RegisterDetection(NewEditionFakerA())
	p.RegisterDetection(NewEditionFakerB())
}
