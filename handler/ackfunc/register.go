package ackfunc

import (
	"github.com/oomph-ac/oomph/handler/ack"
)

func init() {
	ack.FuncMap[ack.AckWorldSetBlock] = WorldSetBlock
	ack.FuncMap[ack.AckWorldUpdateChunks] = WorldUpdateChunks

	ack.FuncMap[ack.AckEntityUpdatePosition] = UpdateEntityPosition

	ack.FuncMap[ack.AckPlayerInitalized] = PlayerSetInitalized
	ack.FuncMap[ack.AckPlayerUpdateGamemode] = PlayerUpdateGamemode
	ack.FuncMap[ack.AckPlayerUpdateSimulationRate] = PlayerUpdateSimulationRate
	ack.FuncMap[ack.AckPlayerUpdateLatency] = PlayerUpdateLatency

	ack.FuncMap[ack.AckPlayerUpdateActorData] = PlayerUpdateActorData
	ack.FuncMap[ack.AckPlayerUpdateAbilities] = PlayerUpdateAbilities
	ack.FuncMap[ack.AckPlayerUpdateAttributes] = PlayerUpdateAttributes
	ack.FuncMap[ack.AckPlayerUpdateKnockback] = PlayerUpdateKnockback
	ack.FuncMap[ack.AckPlayerTeleport] = PlayerTeleport

	ack.FuncMap[ack.AckPlayerRecieveCorrection] = PlayerRecieveCorrection
}
