package ack

import (
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
)

type AckID int32
type AckFunc func(*player.Player, ...interface{})

const (
	ResendThreshold = 10

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

type BatchedAck struct {
	Acks        []Acknowledgement
	UntilResend int
}

func (b *BatchedAck) Add(a Acknowledgement) {
	b.Acks = append(b.Acks, a)
}

func (b *BatchedAck) Amt() int {
	return len(b.Acks)
}

func NewBatch() *BatchedAck {
	return &BatchedAck{
		Acks:        []Acknowledgement{},
		UntilResend: ResendThreshold,
	}
}

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
