package concurrency

import "sync"

// Manager provides concurrency control for tasks
type Manager struct {
	locks sync.Map // map[string]chan struct{}
}

// NewManager creates a new concurrency manager
func NewManager() *Manager {
	return &Manager{}
}

// TryAcquire attempts to acquire a lock for the given key.
// Returns true if lock was acquired, false if already locked.
// Key format: "owner/repo#number" (e.g., "facebook/react#123")
func (m *Manager) TryAcquire(key string) bool {
	// Create or load a buffered channel of size 1 (semaphore pattern)
	actual, _ := m.locks.LoadOrStore(key, make(chan struct{}, 1))
	ch := actual.(chan struct{})

	// Non-blocking send - succeeds only if channel is empty
	select {
	case ch <- struct{}{}:
		return true // Successfully acquired lock
	default:
		return false // Lock already held by another task
	}
}

// Release releases the lock for the given key.
// Safe to call even if lock was never acquired or already released.
func (m *Manager) Release(key string) {
	if actual, ok := m.locks.Load(key); ok {
		ch := actual.(chan struct{})
		// Non-blocking receive
		select {
		case <-ch:
			// Successfully released
		default:
			// Already released or not locked - safe to ignore
		}
	}
}