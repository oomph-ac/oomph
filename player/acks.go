package player

// AcknowledgmentComponent is the component of the player that is responsible for sending and handling
// acknowledgments to the player. This is vital for ensuring lag-compensated player experiences.
type AcknowledgmentComponent interface {
	// Add adds an acknowledgment to the current acknowledgment list of the ack component.
	Add(ack Acknowledgment)
	// Execute runs a list of acknowledgments with the given timestamp. It returns true if the ack ID
	// given is on the list.
	Execute(ackID int64) bool
	// Timestamp returns the current timestamp of the acknowledgment component to be sent later to the member player.
	Timestamp() int64
	// SetTimestamp manually sets the timestamp of the acknowledgment component to be sent later to the member
	// player. This is likely to be used in Oomph recordings/replays.
	SetTimestamp(timestamp int64)
	// Responsive returns true if the member player is responding to acknowledgments.
	Responsive() bool

	// Legacy returns true if the acknowledgment component is using legacy mode.
	Legacy() bool
	// SetLegacy sets whether the acknowledgment component should use legacy mode.
	SetLegacy(legacy bool)

	// Tick ticks the acknowledgment component. The client parameter is true if the tick is triggered
	// by the member player sending a PlayerAuthInput packet (see packet.go)
	Tick(client bool)
	// Flush sends the acknowledgment packet to the client and stores all of the current acknowledgments
	// to be handled later.
	Flush()
	// Refresh resets the current timestamp of the acknowledgment component.
	Refresh()
	// Invalidate labels all pending UpdateBlock acknowledgments as invalid - this is primarily for transfers.
	Invalidate()
}

// Acknowledgment is an interface for client acknowledgments to be ran once the member player
// responds to the acknowledgment.
type Acknowledgment interface {
	// Run runs the acknowledgment once it has been acknowledged by the member player.
	Run()
}

func (p *Player) SetACKs(c AcknowledgmentComponent) {
	p.acks = c
}

func (p *Player) ACKs() AcknowledgmentComponent {
	return p.acks
}
