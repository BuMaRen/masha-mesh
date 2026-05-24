package ai_implementation

import "testing"

func TestNewCircularQueue_InvalidCapacity(t *testing.T) {
	_, err := NewCircularQueue[int](0)
	if err != ErrInvalidCapacity {
		t.Fatalf("expected %v, got %v", ErrInvalidCapacity, err)
	}
}

func TestCircularQueue_BasicFlow(t *testing.T) {
	q, err := NewCircularQueue[int](3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !q.IsEmpty() {
		t.Fatalf("expected queue empty")
	}

	if err := q.Enqueue(1); err != nil {
		t.Fatalf("unexpected enqueue error: %v", err)
	}
	if err := q.Enqueue(2); err != nil {
		t.Fatalf("unexpected enqueue error: %v", err)
	}
	if err := q.Enqueue(3); err != nil {
		t.Fatalf("unexpected enqueue error: %v", err)
	}

	if !q.IsFull() {
		t.Fatalf("expected queue full")
	}
	if q.Len() != 3 {
		t.Fatalf("expected len=3, got %d", q.Len())
	}

	if err := q.Enqueue(4); err != ErrQueueFull {
		t.Fatalf("expected full error, got %v", err)
	}

	v, err := q.Peek()
	if err != nil {
		t.Fatalf("unexpected peek error: %v", err)
	}
	if v != 1 {
		t.Fatalf("expected peek=1, got %d", v)
	}

	v, err = q.Dequeue()
	if err != nil {
		t.Fatalf("unexpected dequeue error: %v", err)
	}
	if v != 1 {
		t.Fatalf("expected dequeue=1, got %d", v)
	}

	if err := q.Enqueue(4); err != nil {
		t.Fatalf("unexpected enqueue error: %v", err)
	}

	// Ensure wrap-around order is preserved.
	expected := []int{2, 3, 4}
	for i, want := range expected {
		got, err := q.Dequeue()
		if err != nil {
			t.Fatalf("dequeue %d error: %v", i, err)
		}
		if got != want {
			t.Fatalf("dequeue %d expected %d, got %d", i, want, got)
		}
	}

	if !q.IsEmpty() {
		t.Fatalf("expected queue empty")
	}

	_, err = q.Dequeue()
	if err != ErrQueueEmpty {
		t.Fatalf("expected empty error, got %v", err)
	}
}

func TestCircularQueue_Clear(t *testing.T) {
	q, err := NewCircularQueue[string](2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := q.Enqueue("a"); err != nil {
		t.Fatalf("unexpected enqueue error: %v", err)
	}
	if err := q.Enqueue("b"); err != nil {
		t.Fatalf("unexpected enqueue error: %v", err)
	}

	q.Clear()

	if !q.IsEmpty() {
		t.Fatalf("expected queue empty after clear")
	}
	if q.Len() != 0 {
		t.Fatalf("expected len=0 after clear, got %d", q.Len())
	}
	if q.Cap() != 2 {
		t.Fatalf("expected cap=2, got %d", q.Cap())
	}

	_, err = q.Peek()
	if err != ErrQueueEmpty {
		t.Fatalf("expected empty error, got %v", err)
	}
}
