package component

import (
	"math/rand/v2"
	"slices"
	"sync"
	"time"

	"github.com/oomph-ac/oomph/assert"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component/acknowledgement"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	ACK_DIVIDER              = 1_000
	MAX_ALLOWED_PENDING_ACKS = 200
)

var batchPool = sync.Pool{
	New: func() any {
		return &ackBatch{acks: make([]player.Acknowledgment, 0), timestamp: 0}
	},
}

// ACKComponent is the component of the player that is responsible for sending and handling
// acknowledgments to the player. This is vital for ensuring lag-compensated player experiences.
type ACKComponent struct {
	legacyMode bool
	mPlayer    *player.Player

	ticksSinceLastResponse int64
	currentBatch           *ackBatch
	pending                []*ackBatch
}

func NewACKComponent(p *player.Player) *ACKComponent {
	c := &ACKComponent{
		legacyMode: false, // <= 1.18 is legacy mode enabled
		mPlayer:    p,

		ticksSinceLastResponse: 0,
		pending:                make([]*ackBatch, 0),
	}
	c.Refresh()

	return c
}

// Add adds an acknowledgment to the current acknowledgment list of the ack component.
func (ackC *ACKComponent) Add(ack player.Acknowledgment) {
	assert.IsTrue(ackC.currentBatch != nil, "no batch initalized for %d", ackC.currentBatch.timestamp)
	ackC.currentBatch.acks = append(ackC.currentBatch.acks, ack)
}

// Execute runs a list of acknowledgments with the given timestamp. It returns true if the ack ID
// given is on the list.
func (ackC *ACKComponent) Execute(ackID int64) bool {
	if !ackC.legacyMode {
		ackID /= ACK_DIVIDER
		if ackC.mPlayer.ClientDat.DeviceOS != protocol.DeviceOrbis {
			ackID /= ACK_DIVIDER
		}
	}

	// Iterate throught the pending acks and find the one with the matching timestamp.
	var index int = -1
	for i, b := range ackC.pending {
		if b.timestamp == ackID {
			index = i
			break
		}
	}

	// If there is none found with the given ack ID.
	if index == -1 {
		return false
	}
	assert.IsTrue(index < len(ackC.pending) && index >= 0, "found index (%d) out of bounds of 0-%d", index, len(ackC.pending)-1)

	// Iterate through all the ACK batches up until the matching one and run them.
	for _, batch := range ackC.pending[:index+1] {
		assert.IsTrue(batch.timestamp != 0, "batch timestamp should never be zero")

		// Run all the acknowledgments in the batch
		for _, a := range batch.acks {
			a.Run()
		}

		// Put the batch back in the pool to eventually be reused.
		batch.acks = batch.acks[:0]
		batch.timestamp = 0
		batchPool.Put(batch)
	}

	// Delete all previous ACKs that were sent before the one we got back from the client.
	// Because we expect ACKs to be in order (due to RakNet ordering + game logic), if a client
	// didn't respond to a previous ACK, we don't expect them to send back the previous ones.
	ackC.pending = ackC.pending[index+1:]
	ackC.ticksSinceLastResponse = 0
	return true
}

// Timestamp returns the current timestamp of the acknowledgment component to be sent later to the member player.
func (ackC *ACKComponent) Timestamp() int64 {
	return ackC.currentBatch.timestamp
}

// SetTimestamp manually sets the timestamp of the acknowledgment component to be sent later to the member
// player. This is likely to be used in Oomph recodings/replays.
func (ackC *ACKComponent) SetTimestamp(timestamp int64) {
	assert.IsTrue(timestamp > 0, "timestamp is < 0 (%d)", timestamp)
	ackC.currentBatch.timestamp = timestamp
}

// Responsive returns true if the member player is responding to acknowledgments.
func (ackC *ACKComponent) Responsive() bool {
	if len(ackC.pending) == 0 {
		return true
	}
	return ackC.ticksSinceLastResponse <= MAX_ALLOWED_PENDING_ACKS
}

// Legacy returns true if the acknowledgment component is using legacy mode.
func (ackC *ACKComponent) Legacy() bool {
	return ackC.legacyMode
}

// SetLegacy sets wether or not the acknowledgment component should use legacy mode.
func (ackC *ACKComponent) SetLegacy(legacy bool) {
	ackC.legacyMode = legacy
}

// Tick ticks the acknowledgment component.
func (ackC *ACKComponent) Tick() {
	// Update the latency every second.
	if ackC.mPlayer.ServerTick%10 == 0 {
		ackC.Add(acknowledgement.NewLatencyACK(ackC.mPlayer, time.Now(), ackC.mPlayer.ServerTick))
	}

	// Validate that there are no duplicate timestamps.
	knownTimestamps := make([]int64, 0, len(ackC.pending))
	for _, batch := range ackC.pending {
		assert.IsTrue(!slices.Contains(knownTimestamps, batch.timestamp), "multiple acks found with timestamp %d", batch.timestamp)
		knownTimestamps = append(knownTimestamps, batch.timestamp)
	}

	if len(ackC.pending) > 0 {
		ackC.ticksSinceLastResponse++
	} else {
		ackC.ticksSinceLastResponse = 0
	}
}

// Flush sends the acknowledgment packet to the client and stores all of the current acknowledgments
// to be handled later.
func (ackC *ACKComponent) Flush() {
	assert.IsTrue(ackC.currentBatch.acks != nil, "no buffer for current timestamp to flush %d", ackC.currentBatch.timestamp)
	if len(ackC.currentBatch.acks) == 0 {
		return
	}

	timestamp := ackC.currentBatch.timestamp
	if ackC.legacyMode && ackC.mPlayer.ClientDat.DeviceOS == protocol.DeviceOrbis {
		timestamp /= ACK_DIVIDER
	}

	ackC.mPlayer.SendPacketToClient(&packet.NetworkStackLatency{
		Timestamp:     timestamp,
		NeedsResponse: true,
	})
	ackC.pending = append(ackC.pending, ackC.currentBatch)
	ackC.Refresh()
}

// Refresh resets the current timestamp of the acknowledgment component.
func (ackC *ACKComponent) Refresh() {
	newBatch := batchPool.Get().(*ackBatch)
	newBatch.acks = newBatch.acks[:0]
	newBatch.timestamp = 0
	ackC.currentBatch = newBatch

	// Create a random timestamp, and ensure that it is not already being used.
	for i := 0; i < 3; i++ {
		ackC.SetTimestamp(int64(rand.Uint32()))
		if ackC.legacyMode {
			// On older versions of the game, timestamps in NetworkStackLatency were sent to the nearest thousand.
			ackC.currentBatch.timestamp *= ACK_DIVIDER
		}

		unique := true
		for _, otherACK := range ackC.pending {
			if otherACK.timestamp == ackC.currentBatch.timestamp {
				unique = false
			}
		}

		// We found a timestamp that isn't matching.
		if unique {
			return
		}
	}

	panic(oerror.New("unable to find new ack ID after multiple attempts"))
}

type ackBatch struct {
	acks      []player.Acknowledgment
	timestamp int64
}
