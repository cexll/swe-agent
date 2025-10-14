package github

import (
	"fmt"
	"time"
)

// CommentStatus represents the execution status of a task
type CommentStatus string

const (
	StatusQueued    CommentStatus = "queued"
	StatusWorking   CommentStatus = "working"
	StatusCompleted CommentStatus = "completed"
	StatusFailed    CommentStatus = "failed"
)

// CreatedPR represents a PR that was created as part of a split
type CreatedPR struct {
	Index      int
	Name       string
	BranchName string
	URL        string
	BranchURL  string
	Status     string // "created", "pending", "merged"
	Category   PRCategory
}

// CommentState holds all information needed to render a task comment
// This data structure eliminates special cases by making all states
// variations of the same structure rather than separate code paths
type CommentState struct {
	// Status of the task
	Status CommentStatus

	// Timing information
	StartTime time.Time
	EndTime   *time.Time

	// Execution metadata
	CostUSD      float64
	Username     string
	OriginalBody string
	Context      map[string]string

	// Results
	Summary       string
	ModifiedFiles []string

	// Links
	BranchName string
	BranchURL  string
	PRURL      string
	JobURL     string

	// Error information (only for failed status)
	ErrorDetails string

	// Multi-PR support (for split plans)
	SplitPlan  *SplitPlan
	CreatedPRs []CreatedPR

	// Task progress tracking (checkbox UI)
	Tasks []TaskStep
}

// TaskStep represents a step in the execution with checkbox status
type TaskStep struct {
	Name      string
	Status    string // "pending", "running", "completed", "failed"
	Timestamp time.Time
}

// Duration calculates the execution duration
// Returns empty string if task is still in progress
func (s *CommentState) Duration() string {
	if s.EndTime == nil {
		return ""
	}

	totalSeconds := int(s.EndTime.Sub(s.StartTime).Seconds())
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60

	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// IsInProgress returns true if the task is still running
func (s *CommentState) IsInProgress() bool {
	return s.Status == StatusWorking || s.Status == StatusQueued
}

// IsCompleted returns true if the task finished successfully
func (s *CommentState) IsCompleted() bool {
	return s.Status == StatusCompleted
}

// IsFailed returns true if the task encountered an error
func (s *CommentState) IsFailed() bool {
	return s.Status == StatusFailed
}
