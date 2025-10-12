package store

import (
	"fmt"
	"sync"
	"time"
)

// TaskStatus represents the execution status of a task
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
)

// Task represents a code generation task
type Task struct {
	ID          string
	Title       string
	Status      TaskStatus
	Owner       string
	Repo        string
	IssueNumber int
	Logs        []LogEntry
	CreatedAt   time.Time
	UpdatedAt   time.Time
	PRURLs      []string
	ErrorMsg    string
}

// LogEntry represents a single log message
type LogEntry struct {
	Timestamp time.Time
	Level     string // info, error, success
	Message   string
}

// TaskStore manages task storage with thread-safe operations
type TaskStore struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

// NewTaskStore creates a new task store
func NewTaskStore() *TaskStore {
	return &TaskStore{
		tasks: make(map[string]*Task),
	}
}

// Create adds a new task to the store
func (s *TaskStore) Create(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task.ID == "" {
		return fmt.Errorf("task ID cannot be empty")
	}

	if _, exists := s.tasks[task.ID]; exists {
		return fmt.Errorf("task with ID %s already exists", task.ID)
	}

	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	if task.Logs == nil {
		task.Logs = []LogEntry{}
	}
	if task.PRURLs == nil {
		task.PRURLs = []string{}
	}

	s.tasks[task.ID] = task
	return nil
}

// Get retrieves a task by ID
func (s *TaskStore) Get(id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[id]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", id)
	}

	return task, nil
}

// List returns all tasks sorted by creation time (newest first)
func (s *TaskStore) List() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}

	// Sort by created time, newest first
	for i := 0; i < len(tasks)-1; i++ {
		for j := i + 1; j < len(tasks); j++ {
			if tasks[i].CreatedAt.Before(tasks[j].CreatedAt) {
				tasks[i], tasks[j] = tasks[j], tasks[i]
			}
		}
	}

	return tasks
}

// UpdateStatus updates task status
func (s *TaskStore) UpdateStatus(id string, status TaskStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	task.Status = status
	task.UpdatedAt = time.Now()
	return nil
}

// AppendLog adds a log entry to task
func (s *TaskStore) AppendLog(id string, level, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	task.Logs = append(task.Logs, LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	})
	task.UpdatedAt = time.Now()
	return nil
}

// SetError sets error message for task
func (s *TaskStore) SetError(id string, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	task.ErrorMsg = errMsg
	task.UpdatedAt = time.Now()
	return nil
}

// AddPRURL adds a PR URL to task
func (s *TaskStore) AddPRURL(id string, prURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	task.PRURLs = append(task.PRURLs, prURL)
	task.UpdatedAt = time.Now()
	return nil
}