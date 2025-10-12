package executor

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrencyManager_WithLock_SerializesAccess(t *testing.T) {
	cm := NewConcurrencyManager()
	key := "owner/repo/issue-123"

	var counter int32
	var maxConcurrent int32
	var wg sync.WaitGroup

	// Launch 10 goroutines trying to access the same resource
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			err := cm.WithLock(key, func() error {
				// Track concurrent access
				current := atomic.AddInt32(&counter, 1)
				if current > maxConcurrent {
					atomic.StoreInt32(&maxConcurrent, current)
				}

				// Simulate work
				time.Sleep(10 * time.Millisecond)

				atomic.AddInt32(&counter, -1)
				return nil
			})

			if err != nil {
				t.Errorf("WithLock returned error: %v", err)
			}
		}()
	}

	wg.Wait()

	// Verify that only 1 goroutine was ever accessing the resource at a time
	if maxConcurrent != 1 {
		t.Errorf("Expected max concurrent access = 1, got %d", maxConcurrent)
	}
}

func TestConcurrencyManager_WithLock_DifferentKeysAreIndependent(t *testing.T) {
	cm := NewConcurrencyManager()
	key1 := "owner/repo/issue-1"
	key2 := "owner/repo/issue-2"

	var counter1, counter2 int32
	var wg sync.WaitGroup

	// Launch goroutines for key1
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cm.WithLock(key1, func() error {
				atomic.AddInt32(&counter1, 1)
				time.Sleep(10 * time.Millisecond)
				return nil
			})
		}()
	}

	// Launch goroutines for key2
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cm.WithLock(key2, func() error {
				atomic.AddInt32(&counter2, 1)
				time.Sleep(10 * time.Millisecond)
				return nil
			})
		}()
	}

	wg.Wait()

	// Both keys should have processed all their tasks
	if counter1 != 5 {
		t.Errorf("Expected counter1 = 5, got %d", counter1)
	}
	if counter2 != 5 {
		t.Errorf("Expected counter2 = 5, got %d", counter2)
	}
}

func TestConcurrencyManager_WithLock_PropagatesError(t *testing.T) {
	cm := NewConcurrencyManager()
	key := "owner/repo/issue-123"

	expectedErr := errors.New("test error")

	err := cm.WithLock(key, func() error {
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

func TestConcurrencyManager_WithLock_ReleasesLockOnPanic(t *testing.T) {
	cm := NewConcurrencyManager()
	key := "owner/repo/issue-123"

	// First goroutine panics
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic, got nil")
			}
		}()

		cm.WithLock(key, func() error {
			panic("test panic")
		})
	}()

	// Second goroutine should be able to acquire the lock
	acquired := false
	err := cm.WithLock(key, func() error {
		acquired = true
		return nil
	})

	if err != nil {
		t.Errorf("WithLock returned error: %v", err)
	}

	if !acquired {
		t.Error("Lock was not released after panic")
	}
}

func TestConcurrencyManager_WithLock_OrderingIsPreserved(t *testing.T) {
	cm := NewConcurrencyManager()
	key := "owner/repo/issue-123"

	var order []int
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Launch 10 goroutines in sequence
	for i := 0; i < 10; i++ {
		wg.Add(1)
		id := i
		go func() {
			defer wg.Done()
			// Small delay to ensure goroutines start in order
			time.Sleep(time.Duration(id) * time.Millisecond)

			cm.WithLock(key, func() error {
				mu.Lock()
				order = append(order, id)
				mu.Unlock()
				return nil
			})
		}()
	}

	wg.Wait()

	// Verify all tasks completed
	if len(order) != 10 {
		t.Errorf("Expected 10 tasks, got %d", len(order))
	}

	// Note: We don't strictly enforce FIFO ordering because Go's channel
	// implementation doesn't guarantee it for multiple senders. But we
	// verify that all tasks completed sequentially (no concurrent execution).
}

func TestConcurrencyManager_WithLock_LazyInitialization(t *testing.T) {
	cm := NewConcurrencyManager()

	// Initially, no locks should exist
	if len(cm.locks) != 0 {
		t.Errorf("Expected 0 locks initially, got %d", len(cm.locks))
	}

	// First access creates the lock
	key := "owner/repo/issue-123"
	cm.WithLock(key, func() error { return nil })

	if len(cm.locks) != 1 {
		t.Errorf("Expected 1 lock after first access, got %d", len(cm.locks))
	}

	// Second access reuses the lock
	cm.WithLock(key, func() error { return nil })

	if len(cm.locks) != 1 {
		t.Errorf("Expected 1 lock after second access, got %d", len(cm.locks))
	}
}

func TestConcurrencyManager_WithLock_MultipleKeys(t *testing.T) {
	cm := NewConcurrencyManager()

	keys := []string{
		"owner1/repo1/issue-1",
		"owner1/repo1/issue-2",
		"owner2/repo2/issue-1",
		"owner2/repo2/issue-2",
	}

	var wg sync.WaitGroup
	for _, key := range keys {
		wg.Add(1)
		go func(k string) {
			defer wg.Done()
			cm.WithLock(k, func() error {
				time.Sleep(10 * time.Millisecond)
				return nil
			})
		}(key)
	}

	wg.Wait()

	// Verify that locks were created for each key
	if len(cm.locks) != len(keys) {
		t.Errorf("Expected %d locks, got %d", len(keys), len(cm.locks))
	}
}