package player

type AuthorityMode byte

const (
	// AuthorityModeNone is the authority mode in where Oomph accepts any input given without validation.
	AuthorityModeNone AuthorityMode = iota
	// AuthorityModeSemi is the authority mode in which Oomph validates input, but still allows for any
	// input determined as invalid to be sent to the server. This mode usually consists of detections notifying
	// any available staff members.
	AuthorityModeSemi
	// AuthorityModeComplete is the authority mode in which Oomph validates input, corrects any invalid input,
	// and sends corrected input to the server. This mode does not have any detections involved.
	AuthorityModeComplete
)
