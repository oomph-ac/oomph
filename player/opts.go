package player

import "github.com/oomph-ac/oconfig"

type Opts struct {
	Combat   oconfig.CombatOpts
	Movement oconfig.MovementOpts
	Network  oconfig.NetworkOpts

	// LocalCombat tunes the server-authoritative combat validator: knobs to
	// trade detection strictness for hit-registration leniency.
	//
	// The zero value is unsafe — BBoxExpansion=0 is honoured as "exact bbox"
	// and EntitySearchRadius=0 disables misprediction resolution (a real
	// killaura hole). Callers constructing Opts{} directly MUST seed this
	// from DefaultLocalCombatOpts(); player.New() already does so.
	LocalCombat LocalCombatOpts
}

// LocalCombatOpts controls the server-authoritative combat component's
// hit-validation behaviour. The zero value is unsafe for production — see
// Opts.LocalCombat. Construct via DefaultLocalCombatOpts() and tweak.
type LocalCombatOpts struct {
	// DisableFullAuthoritative removes server-side gating of attack packets:
	// attacks always forward, detections still flag but no longer block the
	// hit. Default: false (gating on).
	DisableFullAuthoritative bool
	// DisableBlockOcclusionCheck disables the wall-occlusion check (reach/
	// angle gating still applies). Useful when client/server block-state
	// desync eats legit hits. Default: false (check on).
	DisableBlockOcclusionCheck bool
	// RawDistanceFallback also accepts in-cone, in-reach hits with no raycast
	// intersection for non-touch input modes (touch always gets this).
	// Meaningful weakening of reach detection. Default: false.
	RawDistanceFallback bool

	// BBoxExpansion grows the targeted entity's bbox during raycast.
	// Negative => fallback to default 0.1; 0 is honoured as "exact bbox".
	BBoxExpansion float32
	// MaximumReach is the max allowed distance for a valid survival hit.
	// <= 0 falls back to 2.9 (zero reach would land no hits).
	MaximumReach float32
	// ReachLeniency is added to MaximumReach for the raycast pass only,
	// absorbing ~1 frame of jitter without weakening raw-distance reach.
	// Default: 0.
	ReachLeniency float32
	// LerpSteps is the partial-tick sample count between previous and current
	// attack position. <= 0 falls back to 10. Captured once at construction
	// (sizes pre-allocated slices); runtime changes apply next session.
	LerpSteps int
	// EntitySearchRadius is the misprediction-search radius for client
	// air-swings. Negative => fallback to default 6.0; 0 is honoured as
	// "do not search" (disables misprediction resolution entirely).
	EntitySearchRadius float32
}

// DefaultLocalCombatOpts returns oomph's original strict combat tuning.
// Use this when constructing Opts{} directly — the zero LocalCombatOpts{}
// is not numerically equivalent (see Opts.LocalCombat).
func DefaultLocalCombatOpts() LocalCombatOpts {
	return LocalCombatOpts{
		BBoxExpansion:      0.1,
		MaximumReach:       2.9,
		ReachLeniency:      0.0,
		LerpSteps:          10,
		EntitySearchRadius: 6.0,
	}
}

func (p *Player) Opts() *Opts {
	return p.opts
}
