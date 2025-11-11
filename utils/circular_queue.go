package utils

import "iter"

type CircularQueue[T any] struct {
	items []T
	head  int
	tail  int
	size  int
}

func NewCircularQueue[T any](size int) *CircularQueue[T] {
	return &CircularQueue[T]{
		items: make([]T, size),
	}
}

func (q *CircularQueue[T]) Append(item T) {
	if len(q.items) == 0 {
		// Zero-capacity queue – nothing to do.
		return
	}

	// Write the new item at the current tail position.
	q.items[q.tail] = item

	// Advance tail first. If the queue is already full, we also need to
	// advance head to overwrite the oldest element.
	if q.size == len(q.items) {
		// Buffer is full, drop the oldest element located at head.
		q.head = (q.head + 1) % len(q.items)
	} else {
		q.size++
	}

	q.tail = (q.tail + 1) % len(q.items)
}

// Get returns the element at logical position index (0 = oldest).
// It panics if index is out of range.
func (q *CircularQueue[T]) Get(index int, defaultVal T) T {
	if index < 0 || index >= q.size {
		return defaultVal
	}
	return q.items[(q.head+index)%len(q.items)]
}

// Set sets the element at logical position index (0 = oldest).
// It panics if index is out of range.
func (q *CircularQueue[T]) Set(index int, item T) {
	if index < 0 || index >= q.size {
		panic("circular queue: index out of range")
	}
	q.items[(q.head+index)%len(q.items)] = item
}

func (q *CircularQueue[T]) Iter() iter.Seq[T] {
	return func(yield func(T) bool) {
		for index := range q.size {
			if !yield(q.items[(q.head+index)%len(q.items)]) {
				return
			}
		}
	}
}

// Len returns the current number of stored items.
func (q *CircularQueue[T]) Len() int {
	return q.size
}

// Cap returns the maximum number of items the queue can hold.
func (q *CircularQueue[T]) Cap() int {
	return len(q.items)
}

// Pop removes and returns the oldest element. The boolean ok is false if the
// queue is empty.
func (q *CircularQueue[T]) Pop() (item T, ok bool) {
	if q.size == 0 {
		return item, false
	}
	item = q.items[q.head]
	q.head = (q.head + 1) % len(q.items)
	q.size--
	return item, true
}
