package handler

import (
	"math/rand"

	"github.com/oomph-ac/oomph/handler/ack"
	"github.com/oomph-ac/oomph/player"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const HandlerIDAcknowledgements = "oomph:acknowledgements"

const (
	AckDivider  = 1_000
	resendLimit = 3
)

// AcknowledgementHandler handles acknowledgements to the client, so that the anti-cheat knows the precise
// tick the client processed a certain action.
type AcknowledgementHandler struct {
	// LegacyMode is set to true if the client's protocol version is 1.20.0 or lower.
	LegacyMode bool
	// Playstation is set to true if the client is a Playstation client.
	Playstation bool

	// Ticked is true if the player was ticked.
	Ticked bool
	// NonResponsiveTicks is the amount of ticks the client has not responded to the server.
	NonResponsiveTicks int64

	// AckMap is a map of timestamps associated with a list of callbacks.
	// The callbacks are called when NetworkStackLatency is received from the client.
	AckMap map[int64]*ack.BatchedAck
	// CurrentTimestamp is the current timestamp for acks, which is refreshed every server tick
	// where the connections are flushed.
	CurrentTimestamp int64

	initalized bool
	canResend  bool
}

func NewAcknowledgementHandler() *AcknowledgementHandler {
	return &AcknowledgementHandler{
		AckMap: make(map[int64]*ack.BatchedAck),
	}
}

func (AcknowledgementHandler) ID() string {
	return HandlerIDAcknowledgements
}

func (a *AcknowledgementHandler) HandleClientPacket(pk packet.Packet, p *player.Player) bool {
	switch pk := pk.(type) {
	case *packet.NetworkStackLatency:
		return !a.Execute(p, pk.Timestamp)
	case *packet.PlayerAuthInput:
		a.Ticked = true
		a.canResend = true

		if !a.initalized {
			a.Playstation = (p.ClientDat.DeviceOS == protocol.DeviceOrbis)
			a.Refresh()
			a.initalized = true
		}
	}

	return true
}

func (*AcknowledgementHandler) HandleServerPacket(pk packet.Packet, p *player.Player) bool {
	return true
}

func (a *AcknowledgementHandler) OnTick(p *player.Player) {
	a.Validate(p)
	if !p.ReadBatchMode() {
		a.Flush(p)
	}
}

func (a *AcknowledgementHandler) Defer() {
}

func (a *AcknowledgementHandler) Flush(p *player.Player) {
	if p.MState.IsReplay {
		return
	}

	// Resend all the current acknowledgements to the client.
	if a.canResend {
		resends := 0
		a.canResend = false

		for timestamp, batch := range a.AckMap {
			// Check if the client still hasn't responded to the server within the resend threshold.
			if batch.UntilResend--; batch.UntilResend > 0 {
				continue
			}
			batch.UntilResend = ack.ResendThreshold

			// Resend the packet to the client.
			p.SendPacketToClient(&packet.NetworkStackLatency{
				Timestamp:     a.getModifiedTimestamp(timestamp),
				NeedsResponse: true,
			})

			// We do this to prevent Oomph from overloading the client with a bunch of packets.
			if resends++; resends >= resendLimit {
				break
			}
		}
	}

	if pk := a.CreatePacket(); pk != nil {
		p.SendPacketToClient(pk)
	}
	a.Refresh()
}

// Add adds an acknowledgement to AckMap.
func (a *AcknowledgementHandler) Add(newAck ack.Acknowledgement) {
	if a.AckMap[a.CurrentTimestamp] == nil {
		a.AckMap[a.CurrentTimestamp] = ack.NewBatch()
	}
	a.AckMap[a.CurrentTimestamp].Add(newAck)
}

// Execute takes a timestamp, and looks for callbacks associated with it.
func (a *AcknowledgementHandler) Execute(p *player.Player, timestamp int64) bool {
	if a.LegacyMode {
		return a.tryExecute(p, timestamp)
	}

	timestamp /= AckDivider
	if !a.Playstation {
		timestamp /= AckDivider
	}

	return a.tryExecute(p, timestamp)
}

func (a *AcknowledgementHandler) Validate(p *player.Player) {
	if !a.Ticked {
		return
	}
	a.Ticked = false

	if len(a.AckMap) == 0 {
		a.NonResponsiveTicks = 0
		return
	}

	a.NonResponsiveTicks++
	if a.NonResponsiveTicks >= 200 {
		p.Disconnect("Network timeout.")
	}
}

// Refresh updates the AcknowledgementHandler's current timestamp with a random value.
func (a *AcknowledgementHandler) Refresh() {
	// Create a random timestamp, and ensure that it is not already being used.
	for {
		a.CurrentTimestamp = int64(rand.Uint32())

		// On clients supposedly <1.20, the timestamp is rounded to the thousands.
		if a.LegacyMode {
			a.CurrentTimestamp *= 1000
		}

		// Check if the timestamp is already being used, if not, break out of the loop.
		if _, ok := a.AckMap[a.CurrentTimestamp]; !ok {
			break
		}
	}
}

// CreatePacket creates a NetworkStackLatency packet with the current timestamp.
func (a *AcknowledgementHandler) CreatePacket() *packet.NetworkStackLatency {
	batch, ok := a.AckMap[a.CurrentTimestamp]
	if !ok {
		return nil
	}

	if batch.Amt() == 0 {
		delete(a.AckMap, a.CurrentTimestamp)
		return nil
	}

	timestamp := a.getModifiedTimestamp(a.CurrentTimestamp)
	return &packet.NetworkStackLatency{
		Timestamp:     timestamp,
		NeedsResponse: true,
	}
}

func (a *AcknowledgementHandler) getModifiedTimestamp(original int64) int64 {
	timestamp := original
	if a.LegacyMode && a.Playstation {
		timestamp = original / AckDivider
	}

	return timestamp
}

// tryExecute takes a timestamp, and looks for callbacks associated with it.
func (a *AcknowledgementHandler) tryExecute(p *player.Player, timestamp int64) bool {
	p.Dbg.Notify(player.DebugModeACKs, "attempting to execute ack %d", timestamp)

	batch, ok := a.AckMap[timestamp]
	if !ok {
		p.Dbg.Notify(player.DebugModeACKs, "ack %d not found", timestamp)
		return false
	}

	p.Dbg.Notify(player.DebugModeACKs, "executing ack %d (total=%d)", timestamp, batch.Amt())
	a.NonResponsiveTicks = 0
	for _, acked := range batch.Acks {
		acked.Run(p)
	}

	delete(a.AckMap, timestamp)
	return true
}
