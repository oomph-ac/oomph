package packet

const (
	IDInitConnectionRequest uint32 = iota
	IDInitConnectionResponse
	IDHeartbeat
	IDPlayerSnapshot
	IDEntitySnapshot
	IDBlockInteractionSnapshot
	IDAttackSnapshot
)
