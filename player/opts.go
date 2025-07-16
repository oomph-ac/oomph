package player

import "github.com/oomph-ac/oconfig"

type Opts struct {
	Combat           oconfig.CombatOpts
	Movement         oconfig.MovementOpts
	UseDebugCommands bool
}

func (p *Player) Opts() *Opts {
	return p.opts
}
