package taskstore

import (
	"sort"
	"sync"
	"time"
)

type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
)

type Task struct {
	ID          string
	Title       string
	Status      TaskStatus
	RepoOwner   string
	RepoName    string
	IssueNumber int
	Actor       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Logs        []LogEntry
}

type LogEntry struct {
	Timestamp time.Time
	Level     string // info, error, success
	Message   string
}

type Store struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

func NewStore() *Store {
	return &Store{
		tasks: make(map[string]*Task),
	}
}

func (s *Store) Create(task *Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	s.tasks[task.ID] = task
}

func (s *Store) Get(id string) (*Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[id]
	return task, ok
}

func (s *Store) List() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tasks := make([]*Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	// Sort by created time descending
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})
	return tasks
}

func (s *Store) UpdateStatus(id string, status TaskStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task, ok := s.tasks[id]; ok {
		task.Status = status
		task.UpdatedAt = time.Now()
	}
}

func (s *Store) AddLog(id string, level, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task, ok := s.tasks[id]; ok {
		task.Logs = append(task.Logs, LogEntry{
			Timestamp: time.Now(),
			Level:     level,
			Message:   message,
		})
		task.UpdatedAt = time.Now()
	}
}
