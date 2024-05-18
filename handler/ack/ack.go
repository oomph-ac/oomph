package ack

import (
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
)

type AckID int32
type AckFunc func(*player.Player, ...interface{})

const (
	AckWorldSetBlock     AckID = iota // OK
	AckWorldUpdateChunks              // OK

	AckEntityUpdatePosition // OK

	AckPlayerInitalized           // OK
	AckPlayerUpdateGamemode       // OK
	AckPlayerUpdateSimulationRate // OK
	AckPlayerUpdateLatency        // OK

	AckPlayerUpdateActorData  // OK
	AckPlayerUpdateAbilities  // OK
	AckPlayerUpdateAttributes // OK
	AckPlayerUpdateKnockback  // OK
	AckPlayerTeleport         // OK

	AckPlayerRecieveCorrection // OK
)

var FuncMap = map[AckID]AckFunc{}

type Acknowledgement struct {
	ID   AckID
	Data []interface{}

	f AckFunc
}

func New(id AckID, data ...interface{}) Acknowledgement {
	f, found := FuncMap[id]
	if !found {
		panic(oerror.New("acknowledgement id %d not found", id))
	}

	return Acknowledgement{
		ID:   id,
		Data: data,

		f: f,
	}
}

func (a Acknowledgement) Run(p *player.Player) {
	if a.f == nil {
		panic(oerror.New("acknowledgement id %d has no callback", a.ID))
	}

	a.f(p, a.Data...)
}
