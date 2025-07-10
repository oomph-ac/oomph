package detection

import "github.com/oomph-ac/oomph/player"

func Register(p *player.Player) {
	// aim detections
	p.RegisterDetection(New_AimA(p))

	// bad packet detections
	p.RegisterDetection(New_BadPacketA(p))
	p.RegisterDetection(New_BadPacketB(p))

	// edition faker detections
	p.RegisterDetection(New_EditionFakerA(p))
	p.RegisterDetection(New_EditionFakerB(p))
	p.RegisterDetection(New_EditionFakerC(p))

	p.RegisterDetection(New_InvMoveA(p))

	//p.RegisterDetection(New_NukerA(p))

	// killaura detections
	p.RegisterDetection(New_KillauraA(p))
}
