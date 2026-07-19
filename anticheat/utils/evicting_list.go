package utils

type EvictingList[T any] struct {
	items []T
	size  int
	count int
	head  int
}

func NewEvictingList[T any](size int) *EvictingList[T] {
	return &EvictingList[T]{
		size:  size,
		items: make([]T, size),
	}
}

func (e *EvictingList[T]) Add(v T) {
	if e.count < e.size {
		e.items[e.count] = v
		e.count++
	} else {
		e.items[e.head] = v
		e.head = (e.head + 1) % e.size
	}
}

func (e *EvictingList[T]) Items() []T {
	if e.count == 0 {
		return nil
	}

	out := make([]T, e.count)

	if e.count < e.size {
		copy(out, e.items[:e.count])
		return out
	}

	copy(out, e.items[e.head:])
	copy(out[e.size-e.head:], e.items[:e.head])
	return out
}

func (e *EvictingList[T]) Full() bool {
	return e.count >= e.size
}

func (e *EvictingList[T]) Clear() {
	e.count = 0
	e.head = 0
}

func (e *EvictingList[T]) Len() int {
	return e.count
}

func (e *EvictingList[T]) Cap() int {
	return e.size
}
