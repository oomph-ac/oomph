package oconfig

type CombatOpts struct {
	// LeftCPSLimit is the maximum amount of clicks per second that Oomph will allow for the left click.
	LeftCPSLimit int64 `json:"left_cps_limit" comment:"The maximum amount of CPS that Oomph will allow for the left click."`
	// RightCPSLimit is the maximum amount of clicks per second that Oomph will allow for the right click (for placing blocks).
	RightCPSLimit int64 `json:"right_cps_limit" comment:"The maximum amount of CPS that Oomph will allow for the right click."`

	// LeftCPSLimitMobile is the maximum amount of clicks per second that Oomph will allow for the "left click" on mobile devices.
	LeftCPSLimitMobile int64 `json:"left_cps_limit_mobile" comment:"The maximum amount of CPS that Oomph will allow for the left click on mobile devices."`
	// RightCPSLimitMobile is the maximum amount of clicks per second that Oomph will allow for the "right click" on mobile devices.
	RightCPSLimitMobile int64 `json:"right_cps_limit_mobile" comment:"The maximum amount of CPS that Oomph will allow for the right click on mobile devices."`

	// MaximumAttackAngle is the maximum angle in degrees that Oomph will allow for an attack to be considered valid.
	MaximumAttackAngle float32 `json:"maximum_attack_angle" comment:"The maximum angle in degrees that Oomph will allow for an attack to be considered valid."`
	// EnableClientEntityTracking is a boolean that indicates if the proxy should also enable it's client-sided entity tracking to perfectly lag compensate for the client view of entities. This is primarily used for
	// detecting and taking action against reach/killaura. This option is not neccessary for combat rewind to work properly, but should be enabled if you need precise information for herustics or whatnot.
	EnableClientEntityTracking bool `json:"enable_client_entity_tracking" comment:"This option is used to enable Oomph's client-sided entity tracking to perfectly lag compensate for the client view of entities. If you want to enable reach detections, this should be enabled."`
	// AllowNonMobileTouch is a boolean indicating if the proxy should allow non-mobile players to use the touch input mode. Although some devices on Windows may allow for the touch input mode,
	// it can allow for tiny combat gains when specifically being abused.
	AllowNonMobileTouch bool `json:"allow_non_mobile_touch" comment:"Should Oomph allow non-mobile players to use the touch input mode? It is recommended to keep this option disabled as it may allow for\ntiny combat gains from players spoofing their input mode to touch."`
	// AllowSwitchInputMode is a boolean indicating if the proxy should allow players to switch their input mode, or if it should enforce one input mode to be used unless the player re-joins the server. This can primarily
	// be used to prevent players from using exploits that switch the input mode to touch only when enabled to obtain a combat advantage. This option is recommended to be set to false.
	AllowSwitchInputMode bool `json:"allow_switch_input_mode" comment:"Should Oomph allow players to switch their input mode?\nIt is recommended to keep this option disabled as it may allow for tiny combat gains from players switching their input mode to touch."`

	// DisableFullAuthoritative removes server-side gating of attack packets:
	// attacks always forward, while detections still flag rejected hits.
	DisableFullAuthoritative bool `json:"disable_full_authoritative" comment:"If true, attacks always forward to the remote server even if Oomph's validator rejects them.\nDetections still flag — they just no longer block hit registration. Default: false."`
	// DisableBlockOcclusionCheck disables the wall-occlusion check while
	// retaining reach and angle validation.
	DisableBlockOcclusionCheck bool `json:"disable_block_occlusion_check" comment:"If true, hits are not invalidated when their ray passes through a solid block.\nUseful when client/server block-state desync (e.g. a recently-broken block) eats legit hits. Default: false."`
	// RawDistanceFallback accepts in-cone, in-reach hits with no raycast
	// intersection for non-touch input modes. Touch always uses this fallback.
	RawDistanceFallback bool `json:"raw_distance_fallback" comment:"If true, accepts the closest-point-on-bbox distance for non-touch input modes when no raycast lands.\nMEANINGFUL WEAKENING of reach detection — accepts any in-cone, in-reach attack. Default: false."`

	// BBoxExpansion grows the targeted entity's bounding box during raycast.
	// Negative values fall back to 0.1; zero means no growth.
	BBoxExpansion float64 `json:"bbox_expansion" comment:"Amount the targeted entity's bbox is grown during raycast. Larger = more lenient on edge-of-hitbox hits.\nNegative falls back to 0.1; 0 is honoured as 'exact bbox, no growth'."`
	// MaximumReach is the maximum allowed distance for a survival hit.
	// Non-positive values fall back to 2.9.
	MaximumReach float64 `json:"maximum_reach" comment:"Maximum allowed distance for a valid survival hit. <= 0 falls back to 2.9 (zero reach would land no hits)."`
	// ReachLeniency is added to MaximumReach for the raycast pass only.
	ReachLeniency float64 `json:"reach_leniency" comment:"Added to MaximumReach for the raycast pass only. Absorbs ~1 frame of jitter without weakening raw-distance reach. Default: 0."`
	// LerpSteps is the partial-tick sample count between the previous and
	// current attack positions. Non-positive values fall back to 10.
	LerpSteps int `json:"lerp_steps" comment:"Partial-tick sample count between previous and current attack position when validating hits.\nHigher = more accurate, more CPU. <= 0 falls back to 10."`
	// EntitySearchRadius controls misprediction resolution for client air
	// swings. Negative values fall back to 6; zero disables the search.
	EntitySearchRadius float64 `json:"entity_search_radius" comment:"Radius the misprediction-search uses when the client swung in air but may have hit something.\nNegative falls back to 6.0; 0 is honoured as 'do not search' (disables misprediction resolution — anti-cheat hole)."`
}

func Combat() CombatOpts {
	return Global.Combat
}
