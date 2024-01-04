package detection

import "github.com/oomph-ac/oomph/player"

func RegisterDetections(p *player.Player) {
	p.RegisterDetection(&ReachA{})
}
