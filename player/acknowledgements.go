package player

import (
	"math/rand"
	"sync"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type Acknowledgements struct {
	AcknowledgeMap   map[int64][]func()
	HasTicked        bool
	CurrentTimestamp int64

	awaitResTicks uint64
	mu            sync.Mutex
}

// Add adds an acknowledgement to run in the future to the map of acknowledgements.
func (a *Acknowledgements) Add(f func()) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.AcknowledgeMap[a.CurrentTimestamp] = append(a.AcknowledgeMap[a.CurrentTimestamp], f)
}

// AddMap adds a list of functions in the AcknowledgeMap with a specified timestamp.
func (a *Acknowledgements) AddMap(m []func(), t int64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.AcknowledgeMap[t] = m
}

// GetMap returns the list of functions in the AcknowledgeMap with the specified timestamp.
func (a *Acknowledgements) GetMap(t int64) ([]func(), bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	m, ok := a.AcknowledgeMap[t]
	if !ok {
		return nil, false
	}

	return m, true
}

// Handle gets the acknowledgement in the map with the timestamp given in the function. If there is no acknowledgement
// found, then false is returned. If there is an acknowledgement, then it is removed from the map and the function is ran.
// "awaitResTicks" will also bet set to 0, as the client has responded to an acknowledgement.
func (a *Acknowledgements) Handle(i int64, tryOther bool) bool {
	ok := a.tryHandle(i)
	if !ok && tryOther {
		ok = a.tryHandle(i / 1000)
	}

	return ok
}

func (a *Acknowledgements) tryHandle(i int64) bool {
	a.mu.Lock()
	calls, ok := a.AcknowledgeMap[i]
	a.mu.Unlock()

	if ok {
		a.awaitResTicks = 0
		a.Remove(i)
		for _, f := range calls {
			f()
		}
	}

	return ok
}

// Remove removes an acknowledgement from the map of acknowledgements.
func (a *Acknowledgements) Remove(i int64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.AcknowledgeMap, i)
}

// Refresh sets a new timestamp for the acknowledgements.
func (a *Acknowledgements) Refresh() {
	a.mu.Lock()
	defer a.mu.Unlock()

	for {
		a.CurrentTimestamp = int64(rand.Uint32()) * 1000
		if _, ok := a.AcknowledgeMap[a.CurrentTimestamp]; !ok {
			break
		}
	}
}

// Create creats a new acknowledgement packet and returns it.
func (a *Acknowledgements) Create() *packet.NetworkStackLatency {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.AcknowledgeMap[a.CurrentTimestamp]) == 0 {
		return nil
	}

	return &packet.NetworkStackLatency{
		Timestamp:     a.CurrentTimestamp,
		NeedsResponse: true,
	}
}

// Validate checks if the client is still responding to acknowledgements sent to it. If it's determined that
// the client is not responding despite ticking, this function will return false. This is to prevent modified
// clients from breaking certain systems by simply ignoring acknowledgements we send.
func (a *Acknowledgements) Validate() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.HasTicked {
		return true
	}

	a.HasTicked = false

	if len(a.AcknowledgeMap) == 0 {
		a.awaitResTicks = 0
		return true
	}

	a.awaitResTicks++
	return a.awaitResTicks < 200
}

func (p *Player) sendAck() {
	acks := p.Acknowledgements()
	if pk := acks.Create(); pk != nil {
		m, ok := acks.GetMap(acks.CurrentTimestamp)
		if !ok {
			return
		}
		p.conn.WritePacket(pk)

		// NetworkStackLatency behavior on Playstation devices sends the original timestamp
		// back to the server for a certain period of time (?) but then starts dividing the timestamp later on.
		// TODO: Figure out wtf is going on and get rid of this hack (aka never!)
		if p.ClientData().DeviceOS == protocol.DeviceOrbis {
			acks.AddMap(m, acks.CurrentTimestamp/1000)
		}

		acks.Refresh()
	}
}
