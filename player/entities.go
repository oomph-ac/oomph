package player

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/oomph-ac/oomph/entity"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// EntityTrackerComponent is a component that handles entities that the member player is
// viewing on their screen.
type EntityTrackerComponent interface {
	// AddEntity adds an entity to the entity tracker component.
	AddEntity(rid uint64, ent *entity.Entity)
	// RemoveEntity removes an entity from the entity tracker component.
	RemoveEntity(rid uint64)
	// FindEntity searches for an entity in the entity tracker component from the given runtime ID.
	FindEntity(rid uint64) *entity.Entity
	// All returns all the entities the entity tracker component is tracking.
	All() map[uint64]*entity.Entity
	// MoveEntity moves an entity to the given position.
	MoveEntity(rid uint64, tick int64, pos mgl32.Vec3, teleport bool)

	// HandleMovePlayer is a function that handles entity position updates sent with MovePlayerPacket.
	HandleMovePlayer(pk *packet.MovePlayer)
	// HandleMoveActorAbsolute is a function that handles entity position updates sent with MoveActorAbsolutePacket.
	HandleMoveActorAbsolute(pk *packet.MoveActorAbsolute)

	// SetMaxRewind sets the maximum amount of ticks that entities are allowed to be
	// rewinded by.
	SetMaxRewind(rTicks int)
	// MaxRewind returns the maximum amount of ticks that entities are allowed to be
	// rewound by.
	MaxRewind() int

	// Tick makes the entity tracker component tick all of the entities. If the player has
	// full authoritative combat enabled, this is called on the "server" goroutine. On all other
	// modes it is called when PlayerAuthInput is recieved.
	Tick(tick int64)
}

func (p *Player) SetEntityTracker(et EntityTrackerComponent) {
	p.entTracker = et
}

func (p *Player) EntityTracker() EntityTrackerComponent {
	return p.entTracker
}

func (p *Player) SetClientEntityTracker(et EntityTrackerComponent) {
	p.clientEntTracker = et
}

func (p *Player) ClientEntityTracker() EntityTrackerComponent {
	return p.clientEntTracker
}
