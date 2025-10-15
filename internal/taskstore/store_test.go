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

func TestStore_SupersedeOlder_NoMatches(t *testing.T) {
	store := NewStore()
	// task in other repo/issue should not be touched
	store.Create(&Task{ID: "t1", RepoOwner: "o1", RepoName: "r1", IssueNumber: 1, Status: StatusPending})

	n := store.SupersedeOlder("o2", "r2", 2, "t1")
	if n != 0 {
		t.Fatalf("affected = %d, want 0", n)
	}
	got, _ := store.Get("t1")
	if got.Status != StatusPending {
		t.Fatalf("status changed unexpectedly: %v", got.Status)
	}
}

func TestStore_SupersedeOlder_MarkOlder(t *testing.T) {
	store := NewStore()
	// three tasks for same repo/issue, one excluded id
	a := &Task{ID: "a", RepoOwner: "o", RepoName: "r", IssueNumber: 5, Status: StatusPending}
	b := &Task{ID: "b", RepoOwner: "o", RepoName: "r", IssueNumber: 5, Status: StatusPending}
	c := &Task{ID: "c", RepoOwner: "o", RepoName: "r", IssueNumber: 5, Status: StatusRunning}
	store.Create(a)
	store.Create(b)
	store.Create(c)

	// supersede all except b
	n := store.SupersedeOlder("o", "r", 5, "b")
	if n != 1 { // only 'a' is pending and not excluded; 'c' is running
		t.Fatalf("affected = %d, want 1", n)
	}

	// a should be failed with superseded log
	gotA, _ := store.Get("a")
	if gotA.Status != StatusFailed {
		t.Fatalf("a status = %s, want failed", gotA.Status)
	}
	if len(gotA.Logs) == 0 || gotA.Logs[len(gotA.Logs)-1].Message != "Superseded by newer /code comment" {
		t.Fatalf("a logs missing superseded entry: %+v", gotA.Logs)
	}
	if gotA.UpdatedAt.IsZero() {
		t.Fatal("a UpdatedAt should be set")
	}

	// b should remain pending (excluded)
	gotB, _ := store.Get("b")
	if gotB.Status != StatusPending {
		t.Fatalf("b status = %s, want pending", gotB.Status)
	}

	// c was running; should not be force-failed
	gotC, _ := store.Get("c")
	if gotC.Status != StatusRunning {
		t.Fatalf("c status = %s, want running", gotC.Status)
	}
}

func TestStore_SupersedeOlder_MultipleOlder(t *testing.T) {
	store := NewStore()
	ids := []string{"x1", "x2", "x3", "x4"}
	for _, id := range ids {
		store.Create(&Task{ID: id, RepoOwner: "o", RepoName: "r", IssueNumber: 8, Status: StatusPending})
	}
	// exclude latest id x4
	n := store.SupersedeOlder("o", "r", 8, "x4")
	if n != 3 {
		t.Fatalf("affected = %d, want 3", n)
	}
	for _, id := range ids[:3] {
		got, _ := store.Get(id)
		if got.Status != StatusFailed {
			t.Fatalf("%s status = %s, want failed", id, got.Status)
		}
	}
	gotX4, _ := store.Get("x4")
	if gotX4.Status != StatusPending {
		t.Fatalf("x4 status = %s, want pending", gotX4.Status)
	}
}
