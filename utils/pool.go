package utils

import (
	"sync"

	"github.com/ethaniccc/float32-cube/cube"
	"github.com/go-gl/mathgl/mgl32"
)

// Vec3Pool is a pool of reusable Vec3 objects to reduce allocations
var Vec3Pool = sync.Pool{
	New: func() interface{} {
		return &mgl32.Vec3{}
	},
}

// GetVec3 retrieves a Vec3 from the pool
func GetVec3() *mgl32.Vec3 {
	return Vec3Pool.Get().(*mgl32.Vec3)
}

// PutVec3 returns a Vec3 to the pool
func PutVec3(v *mgl32.Vec3) {
	*v = mgl32.Vec3{} // Reset to zero
	Vec3Pool.Put(v)
}

// BBoxListPool is a pool of reusable BBox slices
var BBoxListPool = sync.Pool{
	New: func() interface{} {
		s := make([]cube.BBox, 0, 32) // Pre-allocate capacity for common case
		return &s
	},
}

// GetBBoxList retrieves a BBox slice from the pool
func GetBBoxList() *[]cube.BBox {
	list := BBoxListPool.Get().(*[]cube.BBox)
	*list = (*list)[:0] // Reset length to 0
	return list
}

// PutBBoxList returns a BBox slice to the pool
func PutBBoxList(list *[]cube.BBox) {
	if list != nil {
		*list = (*list)[:0] // Clear the slice
		BBoxListPool.Put(list)
	}
}

// CollisionBuffer holds pre-allocated vectors for collision calculations
type CollisionBuffer struct {
	YVel        mgl32.Vec3
	XVel        mgl32.Vec3
	ZVel        mgl32.Vec3
	Penetration mgl32.Vec3
}

// CollisionBufferPool is a pool of reusable collision buffers
var CollisionBufferPool = sync.Pool{
	New: func() interface{} {
		return &CollisionBuffer{}
	},
}

// GetCollisionBuffer retrieves a collision buffer from the pool
func GetCollisionBuffer() *CollisionBuffer {
	buf := CollisionBufferPool.Get().(*CollisionBuffer)
	// Reset all vectors
	buf.YVel = mgl32.Vec3{}
	buf.XVel = mgl32.Vec3{}
	buf.ZVel = mgl32.Vec3{}
	buf.Penetration = mgl32.Vec3{}
	return buf
}

// PutCollisionBuffer returns a collision buffer to the pool
func PutCollisionBuffer(buf *CollisionBuffer) {
	if buf != nil {
		CollisionBufferPool.Put(buf)
	}
}

// PoolStats tracks pool usage statistics
type PoolStats struct {
	Vec3Gets         int64
	Vec3Puts         int64
	BBoxListGets     int64
	BBoxListPuts     int64
	CollisionBufGets int64
	CollisionBufPuts int64
}

var (
	poolStats   PoolStats
	poolStatsMu sync.Mutex
)

// RecordVec3Get increments Vec3 get counter
func RecordVec3Get() {
	poolStatsMu.Lock()
	poolStats.Vec3Gets++
	poolStatsMu.Unlock()
}

// RecordVec3Put increments Vec3 put counter
func RecordVec3Put() {
	poolStatsMu.Lock()
	poolStats.Vec3Puts++
	poolStatsMu.Unlock()
}

// GetPoolStats returns current pool statistics
func GetPoolStats() PoolStats {
	poolStatsMu.Lock()
	defer poolStatsMu.Unlock()
	return poolStats
}

// ResetPoolStats resets pool statistics
func ResetPoolStats() {
	poolStatsMu.Lock()
	poolStats = PoolStats{}
	poolStatsMu.Unlock()
}
