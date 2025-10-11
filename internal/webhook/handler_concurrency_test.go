package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestHandler_ConcurrencySameIssue verifies that multiple commands on the same issue
// are executed serially (not concurrently)
func TestHandler_ConcurrencySameIssue(t *testing.T) {
	secret := "test-secret"
	
	var executionCount atomic.Int32
	var concurrentExecutions atomic.Int32
	var maxConcurrent atomic.Int32

	// Mock executor that tracks concurrent executions
	executor := &mockExecutor{
		executeFunc: func(ctx context.Context, task *Task) error {
			// Increment concurrent counter
			concurrent := concurrentExecutions.Add(1)
			
			// Track max concurrent
			for {
				current := maxConcurrent.Load()
				if concurrent <= current || maxConcurrent.CompareAndSwap(current, concurrent) {
					break
				}
			}

			// Simulate work
			time.Sleep(50 * time.Millisecond)
			
			// Decrement concurrent counter
			concurrentExecutions.Add(-1)
			executionCount.Add(1)
			
			return nil
		},
	}

	handler := NewHandler(secret, "/pilot", executor)

	// Create event for same issue
	createEvent := func() *IssueCommentEvent {
		return &IssueCommentEvent{
			Action: "created",
			Issue: Issue{
				Number: 123,
				Title:  "Test Issue",
				Body:   "Test body",
			},
			Comment: Comment{
				ID:   1,
				Body: "/pilot test command",
				User: User{Login: "testuser", Type: "User"},
			},
			Repository: Repository{
				FullName:      "owner/repo",
				DefaultBranch: "main",
			},
		}
	}

	// Send 5 concurrent requests for the same issue
	const numRequests = 5
	var wg sync.WaitGroup

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(requestNum int) {
			defer wg.Done()

			event := createEvent()
			payload, _ := json.Marshal(event)
			
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(payload)
			signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

			req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
			req.Header.Set("X-Hub-Signature-256", signature)

			w := httptest.NewRecorder()
			handler.HandleIssueComment(w, req)

			if w.Code != http.StatusAccepted {
				t.Errorf("Request %d: Status = %d, want %d", requestNum, w.Code, http.StatusAccepted)
			}
		}(i)
	}

	wg.Wait()

	// Wait for all executions to complete
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for executions to complete (completed: %d/%d)", 
				executionCount.Load(), numRequests)
		case <-ticker.C:
			if executionCount.Load() == numRequests {
				goto done
			}
		}
	}
done:

	// Verify: all requests should have executed
	if executionCount.Load() != numRequests {
		t.Errorf("Expected %d executions, got %d", numRequests, executionCount.Load())
	}

	// Verify: max 1 concurrent execution (serialized)
	maxConc := maxConcurrent.Load()
	if maxConc != 1 {
		t.Errorf("Expected max 1 concurrent execution, got %d (not properly serialized)", maxConc)
	}

	t.Logf("✅ All %d requests executed serially (max concurrent: %d)", numRequests, maxConc)
}

// TestHandler_ConcurrencyDifferentIssues verifies that commands on different issues
// can execute concurrently (in parallel)
func TestHandler_ConcurrencyDifferentIssues(t *testing.T) {
	secret := "test-secret"
	
	var executionCount atomic.Int32
	var concurrentExecutions atomic.Int32
	var maxConcurrent atomic.Int32

	// Mock executor that tracks concurrent executions
	executor := &mockExecutor{
		executeFunc: func(ctx context.Context, task *Task) error {
			// Increment concurrent counter
			concurrent := concurrentExecutions.Add(1)
			
			// Track max concurrent
			for {
				current := maxConcurrent.Load()
				if concurrent <= current || maxConcurrent.CompareAndSwap(current, concurrent) {
					break
				}
			}

			// Simulate work
			time.Sleep(100 * time.Millisecond)
			
			// Decrement concurrent counter
			concurrentExecutions.Add(-1)
			executionCount.Add(1)
			
			return nil
		},
	}

	handler := NewHandler(secret, "/pilot", executor)

	// Create events for different issues
	createEvent := func(issueNumber int) *IssueCommentEvent {
		return &IssueCommentEvent{
			Action: "created",
			Issue: Issue{
				Number: issueNumber,
				Title:  "Test Issue",
				Body:   "Test body",
			},
			Comment: Comment{
				ID:   1,
				Body: "/pilot test command",
				User: User{Login: "testuser", Type: "User"},
			},
			Repository: Repository{
				FullName:      "owner/repo",
				DefaultBranch: "main",
			},
		}
	}

	// Send concurrent requests for different issues
	const numIssues = 3
	var wg sync.WaitGroup

	startTime := time.Now()

	for i := 1; i <= numIssues; i++ {
		wg.Add(1)
		go func(issueNum int) {
			defer wg.Done()

			event := createEvent(issueNum)
			payload, _ := json.Marshal(event)
			
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(payload)
			signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

			req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
			req.Header.Set("X-Hub-Signature-256", signature)

			w := httptest.NewRecorder()
			handler.HandleIssueComment(w, req)

			if w.Code != http.StatusAccepted {
				t.Errorf("Issue %d: Status = %d, want %d", issueNum, w.Code, http.StatusAccepted)
			}
		}(i)
	}

	wg.Wait()

	// Wait for all executions to complete
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for executions to complete (completed: %d/%d)", 
				executionCount.Load(), numIssues)
		case <-ticker.C:
			if executionCount.Load() == numIssues {
				goto done
			}
		}
	}
done:

	elapsed := time.Since(startTime)

	// Verify: all requests should have executed
	if executionCount.Load() != numIssues {
		t.Errorf("Expected %d executions, got %d", numIssues, executionCount.Load())
	}

	// Verify: at least 2 concurrent executions (running in parallel)
	maxConc := maxConcurrent.Load()
	if maxConc < 2 {
		t.Errorf("Expected at least 2 concurrent executions, got %d (not running in parallel)", maxConc)
	}

	// Verify: total time should be ~100ms (parallel), not ~300ms (serial)
	if elapsed > 200*time.Millisecond {
		t.Errorf("Different issues should run in parallel, took %v (too slow)", elapsed)
	}

	t.Logf("✅ %d issues executed in parallel (max concurrent: %d, total time: %v)", 
		numIssues, maxConc, elapsed)
}

// TestHandler_ConcurrencyMixedScenario verifies mixed scenario:
// - Multiple commands on issue-1 (should serialize)
// - Multiple commands on issue-2 (should serialize)
// - issue-1 and issue-2 should run in parallel
func TestHandler_ConcurrencyMixedScenario(t *testing.T) {
	secret := "test-secret"
	
	var executionCount atomic.Int32
	executions := make(map[int][]time.Time)
	var executionsMu sync.Mutex

	// Mock executor that records execution times
	executor := &mockExecutor{
		executeFunc: func(ctx context.Context, task *Task) error {
			startTime := time.Now()
			
			// Record start time
			executionsMu.Lock()
			executions[task.Number] = append(executions[task.Number], startTime)
			executionsMu.Unlock()

			// Simulate work
			time.Sleep(50 * time.Millisecond)
			
			executionCount.Add(1)
			return nil
		},
	}

	handler := NewHandler(secret, "/pilot", executor)

	// Create event for specific issue
	createEvent := func(issueNumber int) *IssueCommentEvent {
		return &IssueCommentEvent{
			Action: "created",
			Issue: Issue{
				Number: issueNumber,
				Title:  "Test Issue",
				Body:   "Test body",
			},
			Comment: Comment{
				ID:   1,
				Body: "/pilot test command",
				User: User{Login: "testuser", Type: "User"},
			},
			Repository: Repository{
				FullName:      "owner/repo",
				DefaultBranch: "main",
			},
		}
	}

	// Send 3 commands for issue-1 and 3 commands for issue-2
	const commandsPerIssue = 3
	const numIssues = 2
	var wg sync.WaitGroup

	for issueNum := 1; issueNum <= numIssues; issueNum++ {
		for cmd := 0; cmd < commandsPerIssue; cmd++ {
			wg.Add(1)
			go func(issue int) {
				defer wg.Done()

				event := createEvent(issue)
				payload, _ := json.Marshal(event)
				
				mac := hmac.New(sha256.New, []byte(secret))
				mac.Write(payload)
				signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

				req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
				req.Header.Set("X-Hub-Signature-256", signature)

				w := httptest.NewRecorder()
				handler.HandleIssueComment(w, req)
			}(issueNum)
		}
	}

	wg.Wait()

	// Wait for all executions to complete
	expectedExecutions := commandsPerIssue * numIssues
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for executions (completed: %d/%d)", 
				executionCount.Load(), expectedExecutions)
		case <-ticker.C:
			if executionCount.Load() == int32(expectedExecutions) {
				goto done
			}
		}
	}
done:

	// Verify execution times
	executionsMu.Lock()
	defer executionsMu.Unlock()

	for issueNum := 1; issueNum <= numIssues; issueNum++ {
		times := executions[issueNum]
		if len(times) != commandsPerIssue {
			t.Errorf("Issue %d: expected %d executions, got %d", issueNum, commandsPerIssue, len(times))
			continue
		}

		// Check that executions for same issue are serialized
		// (each starts after previous finishes, ~50ms apart)
		for i := 1; i < len(times); i++ {
			gap := times[i].Sub(times[i-1])
			// Allow some tolerance for scheduling
			if gap < 40*time.Millisecond {
				t.Errorf("Issue %d: executions %d and %d too close (%v), not properly serialized", 
					issueNum, i-1, i, gap)
			}
		}
	}

	t.Logf("✅ Mixed scenario verified: %d issues × %d commands executed correctly", 
		numIssues, commandsPerIssue)
}