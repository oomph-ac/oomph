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

	// inv move detections
	p.RegisterDetection(New_InvMoveA(p))

	// scaffold detections
	p.RegisterDetection(New_ScaffoldA(p))
	p.RegisterDetection(New_ScaffoldB(p))

	//p.RegisterDetection(New_NukerA(p))

	// killaura detections
	p.RegisterDetection(New_KillauraA(p))

	// reach detections
	p.RegisterDetection(New_ReachA(p))
	p.RegisterDetection(New_ReachB(p))

	// hitbox detections
	p.RegisterDetection(New_HitboxA(p))
}
