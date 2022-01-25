package virtual

import (
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/sandertv/gophertunnel/minecraft"
	"go.uber.org/atomic"
	"math"
)

// Player contains information about a player, such as its virtual world.
type Player struct {
	w    *World
	conn *minecraft.Conn

	pos atomic.Value

	viewDist int32
}

// NewPlayer creates a new player from the given identity data, client data, position, and world.
func NewPlayer(w *World, viewDist int32, pos mgl64.Vec3, conn *minecraft.Conn) *Player {
	p := &Player{
		w:    w,
		conn: conn,

		viewDist: viewDist,
	}
	p.pos.Store(pos)
	return p
}

// Move moves the player to the given position.
func (p *Player) Move(pos mgl64.Vec3) {
	p.pos.Store(p.Position().Add(pos))
	p.cleanCache()
}

// Close closes the player.
func (p *Player) Close() {
	p.w.chunkMu.Lock()
	defer p.w.chunkMu.Unlock()
	p.w.chunks = nil
}

// Position returns the current position of the player.
func (p *Player) Position() mgl64.Vec3 {
	return p.pos.Load().(mgl64.Vec3)
}

// ChunkPos returns the chunk position of the player.
func (p *Player) ChunkPos() world.ChunkPos {
	pos := p.Position()
	return world.ChunkPos{int32(math.Floor(pos[0])) >> 4, int32(math.Floor(pos[2])) >> 4}
}

// cleanCache removes all cached chunks that are no longer in the player's view.
func (p *Player) cleanCache() {
	p.w.chunkMu.Lock()
	defer p.w.chunkMu.Unlock()

	activePos := p.ChunkPos()
	for pos := range p.w.chunks {
		if p.w.OutOfBounds(pos, activePos, p.viewDist) {
			delete(p.w.chunks, pos)
		}
	}
}
