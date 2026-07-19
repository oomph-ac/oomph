package player

import "github.com/oomph-ac/oomph/anticheat/oconfig"

type Opts struct {
	Combat   oconfig.CombatOpts
	Movement oconfig.MovementOpts
	Network  oconfig.NetworkOpts
}

func (p *Player) Opts() *Opts {
	return p.opts
}
