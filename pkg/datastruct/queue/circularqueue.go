package queue

type CircularQueue[T any] struct {
	elements []T
	front    int
	rear     int
	capacity int
}

func NewCircularQueue[T any](capacity int) *CircularQueue[T] {
	return &CircularQueue[T]{
		elements: make([]T, capacity+1),
		front:    0,
		rear:     0,
		capacity: capacity + 1,
	}
}

func (cq *CircularQueue[T]) Size() int {
	if cq.rear >= cq.front {
		return cq.rear - cq.front
	}
	return cq.capacity - (cq.front - cq.rear)
}

func (cq *CircularQueue[T]) Empty() bool {
	return cq.front == cq.rear
}

func (cq *CircularQueue[T]) Full() bool {
	return (cq.rear+1)%cq.capacity == cq.front
}

func (cq *CircularQueue[T]) Enqueue(value T) bool {
	nextIndex := (cq.rear + 1) % cq.capacity
	if cq.Full() {
		return false
	}
	cq.elements[nextIndex] = value
	cq.rear = nextIndex
	return true
}

func (cq *CircularQueue[T]) Dequeue() (T, bool) {
	if cq.Empty() {
		return *new(T), false
	}
	nextIndex := (cq.front + 1) % cq.capacity
	value := cq.elements[nextIndex]
	cq.front = nextIndex
	return value, true
}

func (cq *CircularQueue[T]) List() []T {
	list := make([]T, 0, cq.Size())
	for i := 0; i < cq.Size(); i++ {
		index := (cq.front + 1 + i) % cq.capacity
		list = append(list, cq.elements[index])
	}
	return list
}

func (cq *CircularQueue[T]) Reset() {
	cq.front = 0
	cq.rear = 0
}
