package oconfig

type MovementOpts struct {
	// The correction threshold represents the amount of distance in blocks between the client and Oomph's
	// predicted position is required to trigger a correction.
	CorrectionThreshold float32 `json:"correction_threshold" comment:"The amount of blocks between the client and Oomph's prediction required to trigger a correction. It is recommended to keep this option at around 0.2-0.5 blocks to avoid large noticable corrections."`
	// PersuasionThreshold is the amount of blocks per tick Oomph should move towards the client position (given that there are no pending corrections and the player's movement has been valid for a long enough period of time).
	// Note that this persuasion is not applied on the Y-axis.
	PersuasionThreshold float32 `json:"persuasion_threshold" comment:"The amount of block per tick Oomph's position moves towards the client's position. Note that the persuasion is not applied on the Y-axis.\nIncreasing this value above the default option will result in slight movement bypasses."`
	// AcceptClientPosition is a boolean that represents if the Oomph proxy should accept the client's position if
	// their position is within the opts.PositionAcceptanceThreshold and the player has no pending corrections (
	// and some other factors, such as immobile, etc.)
	// By default, this is disabled as it may result in small movement bypasses, but enabling it can help
	// reduce the amount of false corrections sent to the client.
	AcceptClientPosition bool `json:"accept_client_position" comment:"Should Oomph accept the client's position completely if it's within the PositionAcceptanceThreshold?\nEnabling this option will result in small movement bypasses."`
	// PositionAcceptanceThreshold (which is only used if AcceptClientPosition is TRUE) is the maximum allowed distance in blocks required for Oomph to accept the client's position.
	PositionAcceptanceThreshold float32 `json:"position_acception_threshold" comment:"The distance between the client and Oomph's position (in blocks) required for Oomph to accept the client's position. This option is only applied if AcceptClientPosition is set to true."`
	// AcceptClientVelocity is a boolean that represents if the Oomph proxy should accept the client's velocity in
	// PlayerAuthInputPacket if the difference between the client's and server predicted end-frame velocity in blocks is
	// within the opts.VelocityAcceptanceThreshold and the player has no pending corrections (and some other factors, such
	// as immobile, etc.)
	// By default, this is disabled as it may result in small movement bypasses, but enabling it can help
	// reduce the amount of false corrections sent to the client.
	AcceptClientVelocity bool `json:"accept_client_velocity" comment:"Should Oomph accept the client's velocity completely if it's within the VelocityAcceptanceThreshold?\nMay result in small movement bypasses."`
	// VelocityAcceptanceThreshold (which is only used if AcceptClientVelocity is TRUE) is the maximum allowed difference
	// in blocks required for Oomph to accept the client's velocity. Note that this perusasion is not applied on the Y-axis.
	VelocityAcceptanceThreshold float32 `json:"velocity_acception_threshold" comment:"The difference between the client and Oomph's velocity (in blocks) required for Oomph to accept the client's velocity.\nNOTE: Increasing this above 0.07 may result in movement bypasses."`
	// LimitAllVelocity is a boolean that represents if the Oomph proxy should limit the prediction velocity to a certain limit. This prevents exploiters from setting a large
	// velocity value when exempted to cause crashes or lag.
	LimitAllVelocity bool `json:"limit_all_velocity" comment:"Should Oomph limit the prediction velocity to a certain limit? This prevents exploiters from setting a large velocity value when exempted to cause crashes or lag."`
	// LimitAllVelocityThreshold is the maximum allowed velocity in blocks per tick for all players. This is checked per-axis and not the overall velocity.
	LimitAllVelocityThreshold float32 `json:"limit_all_velocity_threshold" comment:"The maximum allowed velocity in blocks per tick for all players. This is checked per-axis and not the overall velocity."`
}

func Movement() MovementOpts {
	return Global.Movement
}
