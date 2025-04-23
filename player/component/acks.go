package component

import (
	"math/rand/v2"

	"github.com/oomph-ac/oomph/game"
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/player/component/acknowledgement"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	ACK_DIVIDER              = 1_000
	MAX_ALLOWED_PENDING_ACKS = 1200 // ~ 60 seconds worth of pending ACKs
)

// ACKComponent is the component of the player that is responsible for sending and handling
// acknowledgments to the player. This is vital for ensuring lag-compensated player experiences.
type ACKComponent struct {
	legacyMode   bool
	clientTicked bool
	mPlayer      *player.Player

	ticksSinceLastResponse int64
	currentBatch           *ackBatch
	pending                []*ackBatch
}

func NewACKComponent(p *player.Player) *ACKComponent {
	c := &ACKComponent{
		legacyMode: false, // <= 1.18 is legacy mode enabled
		mPlayer:    p,

		ticksSinceLastResponse: 0,
		pending:                make([]*ackBatch, 0, MAX_ALLOWED_PENDING_ACKS),
	}
	c.Refresh()

	return c
}

// Add adds an acknowledgment to the current acknowledgment list of the ack component.
func (ackC *ACKComponent) Add(ack player.Acknowledgment) {
	if ackC.currentBatch == nil {
		ackC.Refresh()
	}
	//assert.IsTrue(ackC.currentBatch != nil, "no batch initalized for %d", ackC.currentBatch.timestamp)
	ackC.currentBatch.acks = append(ackC.currentBatch.acks, ack)
}

// Execute runs a list of acknowledgments with the given timestamp. It returns true if the ack ID
// given is on the list.
func (ackC *ACKComponent) Execute(ackID int64) bool {
	ackC.mPlayer.Dbg.Notify(player.DebugModeACKs, true, "got raw ACK ID %d", ackID)
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
		ackC.mPlayer.Dbg.Notify(player.DebugModeACKs, true, "no ACK ID found for %d", ackID)
		return false
	}
	//assert.IsTrue(index < len(ackC.pending) && index >= 0, "found index (%d) out of bounds of 0-%d", index, len(ackC.pending)-1)

	// Iterate through all the ACK batches up until the matching one and run them.
	for _, batch := range ackC.pending[:index+1] {
		//assert.IsTrue(batch.timestamp != 0, "batch timestamp should never be zero")

		// Run all the acknowledgments in the batch
		for _, a := range batch.acks {
			a.Run()
		}
	}

	// Delete all previous ACKs that were sent before the one we got back from the client.
	// Because we expect ACKs to be in order (due to RakNet ordering + game logic), if a client
	// didn't respond to a previous ACK, we don't expect them to send back the previous ones.
	ackC.pending = ackC.pending[index+1:]
	ackC.ticksSinceLastResponse = 0
	ackC.mPlayer.Dbg.Notify(player.DebugModeACKs, true, "ACK ID %d executed", ackID)
	return true
}

// Timestamp returns the current timestamp of the acknowledgment component to be sent later to the member player.
func (ackC *ACKComponent) Timestamp() int64 {
	return ackC.currentBatch.timestamp
}

// SetTimestamp manually sets the timestamp of the acknowledgment component to be sent later to the member
// player. This is likely to be used in Oomph recodings/replays.
func (ackC *ACKComponent) SetTimestamp(timestamp int64) {
	//assert.IsTrue(timestamp > 0, "timestamp is < 0 (%d)", timestamp)
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

// SetLegacy sets whether the acknowledgment component should use legacy mode.
func (ackC *ACKComponent) SetLegacy(legacy bool) {
	ackC.legacyMode = legacy
}

// Tick ticks the acknowledgment component.
func (ackC *ACKComponent) Tick(client bool) {
	if client {
		ackC.clientTicked = true
		return
	}

	// Update the latency every half-second.
	if ackC.mPlayer.ServerTick%10 == 0 {
		ackC.Add(acknowledgement.NewLatencyACK(ackC.mPlayer, ackC.mPlayer.Time(), ackC.mPlayer.ServerTick))
	}

	// Validate that there are no duplicate timestamps.
	knownTimestamps := make(map[int64]struct{})
	for _, batch := range ackC.pending {
		_, exists := knownTimestamps[batch.timestamp]
		knownTimestamps[batch.timestamp] = struct{}{}
		if exists {
			ackC.mPlayer.Disconnect(game.ErrorInternalDuplicateACK)
			break
		}
	}

	// If the client hasn't sent us a PlayerAuthInput packet, we can assume that their client may be
	// frozen. In this case, we don't increase the ticksSinceLastResponse counter.
	if !ackC.clientTicked {
		return
	}

	ackC.clientTicked = false
	if len(ackC.pending) > 0 && ackC.mPlayer.ServerConn() != nil {
		ackC.ticksSinceLastResponse++
	} else {
		ackC.ticksSinceLastResponse = 0
	}
}

// Flush sends the acknowledgment packet to the client and stores all of the current acknowledgments
// to be handled later.
func (ackC *ACKComponent) Flush() {
	//assert.IsTrue(ackC.currentBatch.acks != nil, "no buffer for current timestamp to flush %d", ackC.currentBatch.timestamp)
	if ackC.currentBatch == nil {
		ackC.mPlayer.Disconnect(game.ErrorInternalACKIsNull)
		return
	} else if len(ackC.currentBatch.acks) == 0 {
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

// Invalidate drops and clears all current acknowledgments. This should only be called when the player is
// in the process of being transfered to another server.
func (ackC *ACKComponent) Invalidate() {
	ackC.pending = ackC.pending[:0]
	ackC.mPlayer.Log().Info("invalidated ACKs due to transfer")
}

// Refresh resets the current timestamp of the acknowledgment component.
func (ackC *ACKComponent) Refresh() {
	ackC.currentBatch = &ackBatch{timestamp: 0, acks: make([]player.Acknowledgment, 0)}
	// Create a random timestamp, and ensure that it is not already being used.
	for {
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

	//panic(oerror.New("unable to find new ack ID after multiple attempts"))
}

type ackBatch struct {
	acks      []player.Acknowledgment
	timestamp int64
}
