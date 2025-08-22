package utils

import (
	"container/list"
	"sync"
)

// Queue provides a thread-safe FIFO queue implementation
// Safe for concurrent access from multiple goroutines
type Queue struct {
	l  list.List
	mu sync.Mutex // Protects the list from concurrent access
}

// Push adds an item to the front of the queue (FIFO behavior)
// Thread-safe for concurrent use
func (q *Queue) Push(obj any) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.l.PushFront(obj)
}

// Pop removes and returns the oldest item from the queue
// Returns nil if the queue is empty
// Thread-safe for concurrent use
func (q *Queue) Pop() any {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	if v := q.l.Back(); v != nil {
		q.l.Remove(v)
		return v.Value
	}
	
	return nil
}

// Size returns the current number of items in the queue
// Thread-safe for concurrent use
func (q *Queue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.l.Len()
}
