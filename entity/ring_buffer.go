package entity

// RingBuffer is a fixed-size circular buffer for storing position history
type RingBuffer struct {
	buffer   []HistoricalPosition
	capacity int
	head     int // Points to the next write position
	size     int // Current number of elements
}

// NewRingBuffer creates a new ring buffer with the specified capacity
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		buffer:   make([]HistoricalPosition, capacity),
		capacity: capacity,
		head:     0,
		size:     0,
	}
}

// Add inserts a new position into the ring buffer
func (rb *RingBuffer) Add(pos HistoricalPosition) {
	rb.buffer[rb.head] = pos
	rb.head = (rb.head + 1) % rb.capacity
	if rb.size < rb.capacity {
		rb.size++
	}
}

// Get retrieves a position by tick, returns the position and true if found
func (rb *RingBuffer) Get(tick int64) (HistoricalPosition, bool) {
	if rb.size == 0 {
		return HistoricalPosition{}, false
	}

	// Search backwards from most recent
	for i := 0; i < rb.size; i++ {
		idx := (rb.head - 1 - i + rb.capacity) % rb.capacity
		if rb.buffer[idx].Tick == tick {
			return rb.buffer[idx], true
		}
		// If we've gone past the tick we're looking for, stop searching
		if rb.buffer[idx].Tick < tick {
			break
		}
	}

	return HistoricalPosition{}, false
}

// GetClosest retrieves the position closest to the given tick
func (rb *RingBuffer) GetClosest(tick int64) (HistoricalPosition, bool) {
	if rb.size == 0 {
		return HistoricalPosition{}, false
	}

	var closest HistoricalPosition
	var closestDist int64 = 1<<63 - 1
	found := false

	for i := 0; i < rb.size; i++ {
		idx := (rb.head - 1 - i + rb.capacity) % rb.capacity
		dist := abs64(rb.buffer[idx].Tick - tick)
		if dist < closestDist {
			closestDist = dist
			closest = rb.buffer[idx]
			found = true
		}
	}

	return closest, found
}

// GetRange retrieves positions within a tick range [startTick, endTick]
func (rb *RingBuffer) GetRange(startTick, endTick int64) []HistoricalPosition {
	if rb.size == 0 {
		return nil
	}

	result := make([]HistoricalPosition, 0, rb.size)
	for i := 0; i < rb.size; i++ {
		idx := (rb.head - 1 - i + rb.capacity) % rb.capacity
		if rb.buffer[idx].Tick >= startTick && rb.buffer[idx].Tick <= endTick {
			result = append(result, rb.buffer[idx])
		}
	}

	return result
}

// Size returns the current number of elements in the buffer
func (rb *RingBuffer) Size() int {
	return rb.size
}

// Capacity returns the maximum capacity of the buffer
func (rb *RingBuffer) Capacity() int {
	return rb.capacity
}

// Clear removes all elements from the buffer
func (rb *RingBuffer) Clear() {
	rb.head = 0
	rb.size = 0
}

// Latest returns the most recently added position
func (rb *RingBuffer) Latest() (HistoricalPosition, bool) {
	if rb.size == 0 {
		return HistoricalPosition{}, false
	}
	idx := (rb.head - 1 + rb.capacity) % rb.capacity
	return rb.buffer[idx], true
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
