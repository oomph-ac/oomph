package detection

import "github.com/oomph-ac/oomph/player"

// RegisterDetections registers all detections with the given player.
func RegisterDetections(p *player.Player) {
	// aim detections
	p.RegisterDetection(NewAimA())
	p.RegisterDetection(NewAimA())

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

	// timer detections
	p.RegisterDetection(NewTimerA())

	// badpacket detections
	p.RegisterDetection(NewBadPacketA())
	p.RegisterDetection(NewBadPacketB())
	p.RegisterDetection(NewBadPacketC())

	// edition faker detections
	p.RegisterDetection(NewEditionFakerA())
	p.RegisterDetection(NewEditionFakerB())
	p.RegisterDetection(NewEditionFakerC())

	// input faker detections
	p.RegisterDetection(NewInputFakerA())

	// server nuke detections
	p.RegisterDetection(NewServerNukeA())
	p.RegisterDetection(NewServerNukeB())
}
