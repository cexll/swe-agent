package concurrency

import (
	"sync"
	"testing"
	"time"
)

func TestManager_TryAcquire(t *testing.T) {
	m := NewManager()
	key := "test-repo#123"

	// First acquisition should succeed
	if !m.TryAcquire(key) {
		t.Error("First TryAcquire should succeed")
	}

	// Second acquisition should fail (lock held)
	if m.TryAcquire(key) {
		t.Error("Second TryAcquire should fail while lock is held")
	}

	// Release and try again
	m.Release(key)
	if !m.TryAcquire(key) {
		t.Error("TryAcquire should succeed after Release")
	}

	m.Release(key)
}

func TestManager_Release_Idempotent(t *testing.T) {
	m := NewManager()
	key := "test-repo#456"

	// Release without acquiring should not panic
	m.Release(key)
	m.Release(key)

	// Acquire, release multiple times
	m.TryAcquire(key)
	m.Release(key)
	m.Release(key) // Should be safe
	m.Release(key) // Should be safe

	// Should be able to acquire again
	if !m.TryAcquire(key) {
		t.Error("TryAcquire should succeed after multiple releases")
	}
	m.Release(key)
}

func TestManager_ConcurrentAccess(t *testing.T) {
	m := NewManager()
	key := "concurrent-repo#789"

	const numGoroutines = 10
	successCount := 0
	var mu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			if m.TryAcquire(key) {
				mu.Lock()
				successCount++
				mu.Unlock()
				time.Sleep(10 * time.Millisecond) // Simulate work
				m.Release(key)
			}
		}()
	}

	wg.Wait()

	// At most 1 goroutine should have acquired the lock successfully
	// (Others might retry and succeed after release, but at any moment only 1)
	if successCount == 0 {
		t.Error("At least one goroutine should have acquired the lock")
	}
}

func TestManager_DifferentKeys(t *testing.T) {
	m := NewManager()
	key1 := "repo1#100"
	key2 := "repo2#200"

	// Both should succeed - different keys, independent locks
	if !m.TryAcquire(key1) {
		t.Error("TryAcquire for key1 should succeed")
	}
	if !m.TryAcquire(key2) {
		t.Error("TryAcquire for key2 should succeed")
	}

	// Both should be independently locked
	if m.TryAcquire(key1) {
		t.Error("key1 should still be locked")
	}
	if m.TryAcquire(key2) {
		t.Error("key2 should still be locked")
	}

	m.Release(key1)
	m.Release(key2)
}

func TestManager_KeyFormat(t *testing.T) {
	m := NewManager()

	testCases := []string{
		"owner/repo#123",
		"facebook/react#456",
		"org-name/repo-name#789",
		"user/my-project#1",
	}

	for _, key := range testCases {
		if !m.TryAcquire(key) {
			t.Errorf("TryAcquire should succeed for key: %s", key)
		}
		if m.TryAcquire(key) {
			t.Errorf("Second TryAcquire should fail for key: %s", key)
		}
		m.Release(key)
	}
}