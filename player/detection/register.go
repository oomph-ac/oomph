package detection

import "github.com/oomph-ac/oomph/player"

func Register(p *player.Player) {
	// autoclicker detections
	p.RegisterDetection(New_AutoclickerA(p))

	// aim detections
	p.RegisterDetection(New_AimA(p))

	// bad packet detections
	p.RegisterDetection(New_BadPacketA(p))
	p.RegisterDetection(New_BadPacketB(p))
	p.RegisterDetection(New_BadPacketC(p))
	p.RegisterDetection(New_BadPacketD(p))
	p.RegisterDetection(New_BadPacketE(p))
	p.RegisterDetection(New_BadPacketF(p))
	p.RegisterDetection(New_BadPacketG(p))

	// edition faker detections
	p.RegisterDetection(New_EditionFakerA(p))
	p.RegisterDetection(New_EditionFakerB(p))
	p.RegisterDetection(New_EditionFakerC(p))

	p.RegisterDetection(New_InvMoveA(p))

	//p.RegisterDetection(New_NukerA(p))

	// killaura detections
	p.RegisterDetection(New_KillauraA(p))

	// reach detections
	p.RegisterDetection(New_ReachA(p))
	p.RegisterDetection(New_ReachB(p))

	// hitbox detections
	p.RegisterDetection(New_HitboxA(p))
}
