package component

import (
	"math/rand/v2"
	"sync"

	"github.com/oomph-ac/oomph/assert"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	ACK_DIVIDER              = 1_000
	MAX_ALLOWED_PENDING_ACKS = 100
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

	ticksSinceResponse int64
	currentBatch       *ackBatch
	pending            []*ackBatch
}

// Add adds an acknowledgment to the current acknowledgment list of the ack component.
func (ac *ACKComponent) Add(ack player.Acknowledgment) {
	assert.IsTrue(ac.currentBatch != nil, "no batch initalized for %d", ac.currentBatch.timestamp)
	ac.currentBatch.acks = append(ac.currentBatch.acks, ack)
}

// Execute runs a list of acknowledgments with the given timestamp. It returns true if the ack ID
// given is on the list.
func (ac *ACKComponent) Execute(ackID int64) bool {
	if !ac.legacyMode {
		ackID /= ACK_DIVIDER
		if ac.mPlayer.ClientDat.DeviceOS != protocol.DeviceOrbis {
			ackID /= ACK_DIVIDER
		}
	}

	// Iterate throught the pending acks and find the one with the matching timestamp.
	var (
		batch *ackBatch
		index int
	)
	for i, b := range ac.pending {
		if b.timestamp == ackID {
			batch = b
			index = i
			break
		}
	}

	// If there is none found
	if batch == nil {
		return false
	}

	// Iterate through all the ACKs and run them.
	for _, ack := range batch.acks {
		ack.Run()
	}

	// Delete all previous ACKs that were sent before the one we got back from the client.
	// Because we expect ACKs to be in order (due to RakNet ordering + game logic), if a client
	// didn't respond to a previous ACK, we don't expect them to send back the previous ones.
	ac.pending = ac.pending[index+1:]

	return true
}

// Timestamp returns the current timestamp of the acknowledgment component to be sent later to the member player.
func (ac *ACKComponent) Timestamp() int64 {
	return ac.currentBatch.timestamp
}

// SetTimestamp manually sets the timestamp of the acknowledgment component to be sent later to the member
// player. This is likely to be used in Oomph recodings/replays.
func (ac *ACKComponent) SetTimestamp(timestamp int64) {
	ac.currentBatch.timestamp = timestamp
}

// Legacy returns true if the acknowledgment component is using legacy mode.
func (ac *ACKComponent) Legacy() bool {
	return ac.legacyMode
}

// SetLegacy sets wether or not the acknowledgment component should use legacy mode.
func (ac *ACKComponent) SetLegacy(legacy bool) {
	ac.legacyMode = legacy
}

// Flush sends the acknowledgment packet to the client and stores all of the current acknowledgments
// to be handled later.
func (ac *ACKComponent) Flush() {
	assert.IsTrue(ac.currentBatch.acks != nil, "no buffer for current timestamp to flush %d", ac.currentBatch.timestamp)
	if len(ac.currentBatch.acks) == 0 {
		return
	}

	timestamp := ac.currentBatch.timestamp
	if ac.legacyMode && ac.mPlayer.ClientDat.DeviceOS == protocol.DeviceOrbis {
		timestamp /= ACK_DIVIDER
	}

	ac.mPlayer.SendPacketToClient(&packet.NetworkStackLatency{
		Timestamp:     timestamp,
		NeedsResponse: true,
	})
	ac.Refresh()
}

// Refresh resets the current timestamp of the acknowledgment component.
func (ac *ACKComponent) Refresh() {
	newBatch := batchPool.Get().(*ackBatch)
	newBatch.acks = newBatch.acks[:0]
	ac.currentBatch = newBatch

	// Create a random timestamp, and ensure that it is not already being used.
timestampLoop:
	for i := 0; i < 5; i++ {
		ac.currentBatch.timestamp = int64(rand.Uint32())
		if ac.legacyMode {
			// On older versions of the game, timestamps in NetworkStackLatency were
			// sent to the nearest thousand.
			ac.currentBatch.timestamp *= ACK_DIVIDER
		}

		for _, ack := range ac.pending {
			if ack.timestamp == ac.currentBatch.timestamp {
				continue timestampLoop
			}
		}

		// We found a timestamp that isn't matching.
		return
	}

	panic(oerror.New("unable to find new ack ID after 5 random attempts"))
}

type ackBatch struct {
	acks      []player.Acknowledgment
	timestamp int64
}
