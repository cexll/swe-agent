package store

import (
	"sync"
	"testing"
	"time"
)

func TestTaskStore_Create(t *testing.T) {
	store := NewTaskStore()

	task := &Task{
		ID:          "task-1",
		Title:       "Test Task",
		Status:      StatusPending,
		Owner:       "user",
		Repo:        "repo",
		IssueNumber: 1,
	}

	err := store.Create(task)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify task was created
	retrieved, err := store.Get("task-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.ID != task.ID {
		t.Errorf("Expected ID %s, got %s", task.ID, retrieved.ID)
	}
}

func TestTaskStore_CreateDuplicate(t *testing.T) {
	store := NewTaskStore()

	task := &Task{
		ID:     "task-1",
		Status: StatusPending,
	}

	err := store.Create(task)
	if err != nil {
		t.Fatalf("First Create failed: %v", err)
	}

	// Try to create duplicate
	err = store.Create(task)
	if err == nil {
		t.Error("Expected error for duplicate task, got nil")
	}
}

func TestTaskStore_GetNotFound(t *testing.T) {
	store := NewTaskStore()

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent task, got nil")
	}
}

func TestTaskStore_List(t *testing.T) {
	store := NewTaskStore()

	// Create multiple tasks
	for i := 1; i <= 3; i++ {
		task := &Task{
			ID:     string(rune('0' + i)),
			Status: StatusPending,
		}
		time.Sleep(time.Millisecond) // Ensure different timestamps
		if err := store.Create(task); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	tasks := store.List()
	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}

	// Verify newest first
	if tasks[0].ID != "3" {
		t.Errorf("Expected newest task first, got ID %s", tasks[0].ID)
	}
}

func TestTaskStore_UpdateStatus(t *testing.T) {
	store := NewTaskStore()

	task := &Task{
		ID:     "task-1",
		Status: StatusPending,
	}
	store.Create(task)

	err := store.UpdateStatus("task-1", StatusRunning)
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	retrieved, _ := store.Get("task-1")
	if retrieved.Status != StatusRunning {
		t.Errorf("Expected status %s, got %s", StatusRunning, retrieved.Status)
	}
}

func TestTaskStore_AppendLog(t *testing.T) {
	store := NewTaskStore()

	task := &Task{
		ID:     "task-1",
		Status: StatusPending,
	}
	store.Create(task)

	err := store.AppendLog("task-1", "info", "Test log message")
	if err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	retrieved, _ := store.Get("task-1")
	if len(retrieved.Logs) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(retrieved.Logs))
	}

	if retrieved.Logs[0].Message != "Test log message" {
		t.Errorf("Expected 'Test log message', got '%s'", retrieved.Logs[0].Message)
	}
}

func TestTaskStore_SetError(t *testing.T) {
	store := NewTaskStore()

	task := &Task{
		ID:     "task-1",
		Status: StatusRunning,
	}
	store.Create(task)

	err := store.SetError("task-1", "Something went wrong")
	if err != nil {
		t.Fatalf("SetError failed: %v", err)
	}

	retrieved, _ := store.Get("task-1")
	if retrieved.ErrorMsg != "Something went wrong" {
		t.Errorf("Expected error message, got '%s'", retrieved.ErrorMsg)
	}
}

func TestTaskStore_AddPRURL(t *testing.T) {
	store := NewTaskStore()

	task := &Task{
		ID:     "task-1",
		Status: StatusRunning,
	}
	store.Create(task)

	err := store.AddPRURL("task-1", "https://github.com/owner/repo/pull/1")
	if err != nil {
		t.Fatalf("AddPRURL failed: %v", err)
	}

	retrieved, _ := store.Get("task-1")
	if len(retrieved.PRURLs) != 1 {
		t.Fatalf("Expected 1 PR URL, got %d", len(retrieved.PRURLs))
	}
}

func TestTaskStore_Concurrency(t *testing.T) {
	store := NewTaskStore()

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent creates
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			task := &Task{
				ID:     string(rune('a' + id)),
				Status: StatusPending,
			}
			store.Create(task)
		}(i)
	}

	wg.Wait()

	tasks := store.List()
	if len(tasks) != numGoroutines {
		t.Errorf("Expected %d tasks, got %d", numGoroutines, len(tasks))
	}
}