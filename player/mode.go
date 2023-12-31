package player

type AuthorityMode byte

const (
	AuthorityModeNone AuthorityMode = iota
	AuthorityModeSemiServer
	AuthorityModeCompleteServer
)
