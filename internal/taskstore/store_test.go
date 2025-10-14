package taskstore

import (
	"testing"
	"time"
)

func TestStore_CreateGetAndList(t *testing.T) {
	store := NewStore()

	taskA := &Task{ID: "a", Title: "first"}
	store.Create(taskA)
	time.Sleep(5 * time.Millisecond)
	taskB := &Task{ID: "b", Title: "second"}
	store.Create(taskB)

	got, ok := store.Get("a")
	if !ok {
		t.Fatal("Get should return true for existing task")
	}
	if got.Title != "first" {
		t.Fatalf("Get returned title %q, want %q", got.Title, "first")
	}

	list := store.List()
	if len(list) != 2 {
		t.Fatalf("List length = %d, want 2", len(list))
	}
	if list[0].ID != "b" || list[1].ID != "a" {
		t.Fatalf("List order = [%s, %s], want [b, a]", list[0].ID, list[1].ID)
	}
	if list[0].CreatedAt.Before(list[1].CreatedAt) {
		t.Fatal("List should be sorted by CreatedAt descending")
	}
}

func TestStore_UpdateStatusAndAddLog(t *testing.T) {
	store := NewStore()
	task := &Task{ID: "task-1"}
	store.Create(task)

	beforeUpdate := task.UpdatedAt
	store.UpdateStatus("task-1", StatusFailed)

	got, _ := store.Get("task-1")
	if got.Status != StatusFailed {
		t.Fatalf("Status = %s, want %s", got.Status, StatusFailed)
	}
	if !got.UpdatedAt.After(beforeUpdate) {
		t.Fatal("UpdatedAt should change after status update")
	}

	store.AddLog("task-1", "info", "processing")
	if len(got.Logs) != 1 {
		t.Fatalf("Logs length = %d, want 1", len(got.Logs))
	}
	if got.Logs[0].Level != "info" || got.Logs[0].Message != "processing" {
		t.Fatalf("Log entry = %+v, want level=info message=processing", got.Logs[0])
	}
	if got.Logs[0].Timestamp.IsZero() {
		t.Fatal("Log timestamp should be set")
	}
}
