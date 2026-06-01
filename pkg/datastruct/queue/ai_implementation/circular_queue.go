package ai_implementation

import (
	"errors"
	"sync"
)

var (
	ErrInvalidCapacity = errors.New("queue capacity must be greater than 0")
	ErrQueueFull       = errors.New("queue is full")
	ErrQueueEmpty      = errors.New("queue is empty")
)

// CircularQueue is a fixed-size ring buffer queue.
type CircularQueue[T any] struct {
	mu       sync.RWMutex
	buffer   []T
	head     int
	tail     int
	size     int
	capacity int
}

// NewCircularQueue creates a queue with a fixed capacity.
func NewCircularQueue[T any](capacity int) (*CircularQueue[T], error) {
	if capacity <= 0 {
		return nil, ErrInvalidCapacity
	}

	return &CircularQueue[T]{
		buffer:   make([]T, capacity),
		capacity: capacity,
	}, nil
}

// Enqueue pushes a value into the queue.
func (q *CircularQueue[T]) Enqueue(value T) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.size == q.capacity {
		return ErrQueueFull
	}

	q.buffer[q.tail] = value
	q.tail = (q.tail + 1) % q.capacity
	q.size++

	return nil
}

// Dequeue pops a value from the queue.
func (q *CircularQueue[T]) Dequeue() (T, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	var zero T
	if q.size == 0 {
		return zero, ErrQueueEmpty
	}

	value := q.buffer[q.head]
	q.head = (q.head + 1) % q.capacity
	q.size--

	if q.size == 0 {
		q.head = 0
		q.tail = 0
	}

	return value, nil
}

// Peek returns the next value without removing it.
func (q *CircularQueue[T]) Peek() (T, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var zero T
	if q.size == 0 {
		return zero, ErrQueueEmpty
	}

	return q.buffer[q.head], nil
}

func (q *CircularQueue[T]) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return q.size
}

func (q *CircularQueue[T]) Cap() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return q.capacity
}

func (q *CircularQueue[T]) IsEmpty() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return q.size == 0
}

func (q *CircularQueue[T]) IsFull() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return q.size == q.capacity
}

// Clear resets the queue to empty state.
func (q *CircularQueue[T]) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.head = 0
	q.tail = 0
	q.size = 0

	var zero T
	for i := range q.buffer {
		q.buffer[i] = zero
	}
}
