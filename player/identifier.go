package player

import (
	"math/big"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	IdentifierStageInit byte = iota
	IdentifierStageCreatingNewID
	IdentifierStageComplete
)

// IdentifierComponent is a component that handles the unique identification of players.
type IdentifierComponent interface {
	// Stage returns the current status of the identifier component.
	Stage() byte
	// SetStage sets the stage of the identifier component.
	SetStage(stage byte)

	// StartTimeout starts a countdown for the identifier component before it marks itself as timed out
	// because the player did not respond to the blobs.
	StartTimeout(timeout uint16)
	// Tick ticks the identifier component and returns false if an exising timeout has not been resolved.
	Tick() bool

	// Identity returns the identity calculated by this component.
	Identity() *big.Int

	// Request sends chunk blobs to the player to obtain a unique identifier. This identifier, in theory, should remain
	// persistent throughout each device.
	Request()
	// HandleResponse handles the misses and hits from the blob status the client sends back to Oomph. If the client has
	// no ID (all of the blobs are misses), we need to create a new ID for the player. Ideally, this should be rate-limited
	// per IP or some other metric (e.g - DeviceID)
	HandleResponse(pk *packet.ClientCacheBlobStatus)
}

func (p *Player) SetIdentifier(i IdentifierComponent) {
	p.identifier = i
}

func (p *Player) Identifier() IdentifierComponent {
	return p.identifier
}
