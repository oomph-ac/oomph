package utils

import (
	"sync"

	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
)

// BBoxCacheEntry represents a cached bounding box lookup result
type BBoxCacheEntry struct {
	Position  mgl32.Vec3
	Velocity  mgl32.Vec3
	BBoxes    []cube.BBox
	Timestamp int64
}

// BBoxCache caches bounding box lookups to avoid repeated expensive calculations
type BBoxCache struct {
	mu                  sync.RWMutex
	entry               *BBoxCacheEntry
	invalidateThreshold float32
	maxAge              int64
}

// NewBBoxCache creates a new bounding box cache
func NewBBoxCache() *BBoxCache {
	return &BBoxCache{
		invalidateThreshold: 0.1, // Invalidate if player moves > 0.1 blocks
		maxAge:              2,   // Cache valid for max 2 ticks
	}
}

// Get retrieves cached bounding boxes if valid, otherwise returns nil
func (c *BBoxCache) Get(pos, vel mgl32.Vec3, currentTick int64) []cube.BBox {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.entry == nil {
		return nil
	}

	// Check if cache is too old
	if currentTick-c.entry.Timestamp > c.maxAge {
		return nil
	}

	// Check if position has changed significantly
	posDelta := pos.Sub(c.entry.Position)
	if posDelta.Len() > c.invalidateThreshold {
		return nil
	}

	// Check if velocity has changed significantly
	velDelta := vel.Sub(c.entry.Velocity)
	if velDelta.Len() > c.invalidateThreshold {
		return nil
	}

	// Cache is valid, return copy of bboxes
	result := make([]cube.BBox, len(c.entry.BBoxes))
	copy(result, c.entry.BBoxes)
	return result
}

// Set stores bounding boxes in the cache
func (c *BBoxCache) Set(pos, vel mgl32.Vec3, bboxes []cube.BBox, currentTick int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Create copy of bboxes to avoid external modifications
	bboxesCopy := make([]cube.BBox, len(bboxes))
	copy(bboxesCopy, bboxes)

	c.entry = &BBoxCacheEntry{
		Position:  pos,
		Velocity:  vel,
		BBoxes:    bboxesCopy,
		Timestamp: currentTick,
	}
}

// Invalidate clears the cache
func (c *BBoxCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entry = nil
}

// Stats returns cache statistics for monitoring
type BBoxCacheStats struct {
	Hits   int64
	Misses int64
}

var (
	globalCacheStats   BBoxCacheStats
	globalCacheStatsMu sync.Mutex
)

// RecordHit increments the cache hit counter
func RecordCacheHit() {
	globalCacheStatsMu.Lock()
	globalCacheStats.Hits++
	globalCacheStatsMu.Unlock()
}

// RecordMiss increments the cache miss counter
func RecordCacheMiss() {
	globalCacheStatsMu.Lock()
	globalCacheStats.Misses++
	globalCacheStatsMu.Unlock()
}

// GetCacheStats returns current cache statistics
func GetCacheStats() BBoxCacheStats {
	globalCacheStatsMu.Lock()
	defer globalCacheStatsMu.Unlock()
	return globalCacheStats
}

// ResetCacheStats resets cache statistics
func ResetCacheStats() {
	globalCacheStatsMu.Lock()
	globalCacheStats = BBoxCacheStats{}
	globalCacheStatsMu.Unlock()
}
