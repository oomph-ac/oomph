package utils

import (
	"errors"
	"iter"

	"github.com/oomph-ac/oomph/oerror"
)

type CircularQueue[T any] struct {
	items []T
	head  int
	tail  int
	size  int
}

func NewCircularQueue[T any](size int, propagate func() T) *CircularQueue[T] {
	queue := &CircularQueue[T]{
		items: make([]T, size),
		size:  size,
	}
	if propagate != nil {
		for index := range queue.items {
			queue.items[index] = propagate()
		}
	}
	return queue
}

// Get returns the element at logical position index (0 = oldest), or an error if out of range.
func (q *CircularQueue[T]) Get(index int) (T, error) {
	var zero T
	if index < 0 || index >= q.size {
		return zero, errors.New("circularqueue: get out of range")
	}
	return q.items[(q.head+index)%len(q.items)], nil
}

// Set sets the element at logical position index (0 = oldest), or returns an error if out of range.
func (q *CircularQueue[T]) Set(index int, item T) error {
	if index < 0 || index >= q.size {
		return errors.New("circularqueue: set out of range")
	}
	q.items[(q.head+index)%len(q.items)] = item
	return nil
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

// Size returns the maximum number of items the queue can hold.
func (q *CircularQueue[T]) Size() int {
	return q.size
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

// Append appends an item or returns an error if the queue has zero capacity.
func (q *CircularQueue[T]) Append(item T) error {
	if len(q.items) == 0 {
		return oerror.New("circularQueue: append on zero-capacity queue")
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
	return nil
}
