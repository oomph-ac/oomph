package packet

const (
	IDInitConnectionRequest uint32 = iota
	IDInitConnectionResponse
	IDAddPlayerRequest
	IDAddPlayerResponse
	IDPlayerDisconnect
	IDHeartbeat
	IDPlayerSnapshot
	IDBlockInteractionSnapshot
	IDAttackSnapshot
	IDUpdateEntityPosition
	IDUpdateEntityDimensions
	IDUpdateEntityStatus
	IDDetectionEvent
	IDPunishmentEvent
	IDCustomMessage
)
