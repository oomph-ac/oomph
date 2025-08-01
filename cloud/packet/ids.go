package packet

const (
	IDInitConnectionRequest uint32 = iota
	IDInitConnectionResponse
	IDHeartbeat
	IDPlayerSnapshot
	IDBlockInteractionSnapshot
	IDAttackSnapshot
	IDPlayerDisconnect
	IDUpdateEntityPosition
	IDUpdateEntityDimensions
	IDUpdateEntityStatus
)
