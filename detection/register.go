package detection

import "github.com/oomph-ac/oomph/player"

// RegisterDetections registers all detections with the given player.
func RegisterDetections(p *player.Player) {
	p.RegisterDetection(NewReachA())
	p.RegisterDetection(NewReachB())
}
