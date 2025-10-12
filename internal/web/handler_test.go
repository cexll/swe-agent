package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stellarlink/pilot-swe/internal/store"
)

func TestHandler_TaskList(t *testing.T) {
	taskStore := store.NewTaskStore()

	// Create test tasks
	task1 := &store.Task{
		ID:          "task-1",
		Title:       "Test Task 1",
		Status:      store.StatusCompleted,
		Owner:       "user",
		Repo:        "repo",
		IssueNumber: 1,
	}
	taskStore.Create(task1)

	handler, err := NewHandler(taskStore)
	if err != nil {
		t.Fatalf("NewHandler failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.handleTaskList(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("Expected non-empty response body")
	}
}

func TestHandler_TaskDetail(t *testing.T) {
	taskStore := store.NewTaskStore()

	task := &store.Task{
		ID:          "task-1",
		Title:       "Test Task",
		Status:      store.StatusRunning,
		Owner:       "user",
		Repo:        "repo",
		IssueNumber: 1,
	}
	taskStore.Create(task)
	taskStore.AppendLog("task-1", "info", "Starting task")

	handler, err := NewHandler(taskStore)
	if err != nil {
		t.Fatalf("NewHandler failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/task/task-1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "task-1"})
	w := httptest.NewRecorder()

	handler.handleTaskDetail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandler_TaskDetailNotFound(t *testing.T) {
	taskStore := store.NewTaskStore()
	handler, _ := NewHandler(taskStore)

	req := httptest.NewRequest("GET", "/task/nonexistent", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})
	w := httptest.NewRecorder()

	handler.handleTaskDetail(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestStatusColor(t *testing.T) {
	tests := []struct {
		status   store.TaskStatus
		expected string
	}{
		{store.StatusPending, "#6c757d"},
		{store.StatusRunning, "#0d6efd"},
		{store.StatusCompleted, "#198754"},
		{store.StatusFailed, "#dc3545"},
	}

	for _, tt := range tests {
		result := statusColor(tt.status)
		if result != tt.expected {
			t.Errorf("statusColor(%s) = %s, want %s", tt.status, result, tt.expected)
		}
	}
}

func TestLogLevelColor(t *testing.T) {
	tests := []struct {
		level    string
		expected string
	}{
		{"error", "#dc3545"},
		{"success", "#198754"},
		{"info", "#0d6efd"},
		{"unknown", "#6c757d"},
	}

	for _, tt := range tests {
		result := logLevelColor(tt.level)
		if result != tt.expected {
			t.Errorf("logLevelColor(%s) = %s, want %s", tt.level, result, tt.expected)
		}
	}
}