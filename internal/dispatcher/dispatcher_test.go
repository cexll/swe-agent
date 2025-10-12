package dispatcher

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cexll/swe/internal/webhook"
)

type mockExecutor struct {
	fn func(ctx context.Context, task *webhook.Task) error
}

func (m *mockExecutor) Execute(ctx context.Context, task *webhook.Task) error {
	if m.fn == nil {
		return nil
	}
	return m.fn(ctx, task)
}

func TestDispatcherEnqueueRunsTask(t *testing.T) {
	done := make(chan struct{})
	exec := &mockExecutor{
		fn: func(ctx context.Context, task *webhook.Task) error {
			close(done)
			return nil
		},
	}

	d := New(exec, Config{
		Workers:           1,
		QueueSize:         2,
		MaxAttempts:       1,
		InitialBackoff:    10 * time.Millisecond,
		BackoffMultiplier: 2,
		MaxBackoff:        20 * time.Millisecond,
	})
	defer d.Shutdown(context.Background())

	if err := d.Enqueue(&webhook.Task{Repo: "owner/repo", Number: 1}); err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timed out waiting for task execution")
	}
}

func TestDispatcherSerializesSamePR(t *testing.T) {
	var mu sync.Mutex
	active := map[string]int{}
	maxActive := map[string]int{}
	done := make(chan struct{}, 3)

	exec := &mockExecutor{
		fn: func(ctx context.Context, task *webhook.Task) error {
			key := fmt.Sprintf("%s#%d", task.Repo, task.Number)
			mu.Lock()
			active[key]++
			if active[key] > maxActive[key] {
				maxActive[key] = active[key]
			}
			mu.Unlock()

			time.Sleep(20 * time.Millisecond)

			mu.Lock()
			active[key]--
			mu.Unlock()

			done <- struct{}{}
			return nil
		},
	}

	d := New(exec, Config{
		Workers:           3,
		QueueSize:         3,
		MaxAttempts:       1,
		InitialBackoff:    10 * time.Millisecond,
		BackoffMultiplier: 2,
		MaxBackoff:        20 * time.Millisecond,
	})
	defer d.Shutdown(context.Background())

	task := &webhook.Task{Repo: "owner/repo", Number: 99}

	for i := 0; i < 3; i++ {
		if err := d.Enqueue(task); err != nil {
			t.Fatalf("Enqueue returned error: %v", err)
		}
	}

	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Timed out waiting for serialized tasks")
		}
	}

	key := fmt.Sprintf("%s#%d", task.Repo, task.Number)
	if maxActive[key] != 1 {
		t.Fatalf("Expected max concurrent executions 1 for key %s, got %d", key, maxActive[key])
	}
}

func TestDispatcherRetries(t *testing.T) {
	var attemptsMu sync.Mutex
	var attempts []int
	done := make(chan struct{})

	exec := &mockExecutor{
		fn: func(ctx context.Context, task *webhook.Task) error {
			attemptsMu.Lock()
			attempts = append(attempts, task.Attempt)
			attemptsMu.Unlock()

			if task.Attempt == 1 {
				return errors.New("first attempt fails")
			}

			close(done)
			return nil
		},
	}

	d := New(exec, Config{
		Workers:           1,
		QueueSize:         2,
		MaxAttempts:       2,
		InitialBackoff:    10 * time.Millisecond,
		BackoffMultiplier: 2,
		MaxBackoff:        20 * time.Millisecond,
	})
	defer d.Shutdown(context.Background())

	if err := d.Enqueue(&webhook.Task{Repo: "owner/repo", Number: 7}); err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for retry success")
	}

	attemptsMu.Lock()
	defer attemptsMu.Unlock()

	if len(attempts) != 2 {
		t.Fatalf("Expected 2 attempts, got %d", len(attempts))
	}
	if attempts[0] != 1 || attempts[1] != 2 {
		t.Fatalf("Unexpected attempt sequence: %v", attempts)
	}
}

func TestDispatcherEnqueueAfterShutdown(t *testing.T) {
	exec := &mockExecutor{}

	d := New(exec, Config{
		Workers:           1,
		QueueSize:         1,
		MaxAttempts:       1,
		InitialBackoff:    10 * time.Millisecond,
		BackoffMultiplier: 2,
		MaxBackoff:        20 * time.Millisecond,
	})

	d.Shutdown(context.Background())

	err := d.Enqueue(&webhook.Task{Repo: "owner/repo", Number: 1})
	if !errors.Is(err, webhook.ErrQueueClosed) {
		t.Fatalf("Expected ErrQueueClosed, got %v", err)
	}
}

func TestDispatcherQueueFull(t *testing.T) {
	d := &Dispatcher{
		queue:  make(chan *queueItem, 1),
		stopCh: make(chan struct{}),
	}

	d.queue <- &queueItem{task: &webhook.Task{}}

	err := d.Enqueue(&webhook.Task{})
	if !errors.Is(err, webhook.ErrQueueFull) {
		t.Fatalf("Expected ErrQueueFull, got %v", err)
	}
}
