package github

import (
	"testing"
	"time"
)

func TestCommentState_Duration(t *testing.T) {
	tests := []struct {
		name      string
		startTime time.Time
		endTime   *time.Time
		want      string
	}{
		{
			name:      "task in progress",
			startTime: time.Now(),
			endTime:   nil,
			want:      "",
		},
		{
			name:      "task completed in seconds",
			startTime: time.Now(),
			endTime:   timePtr(time.Now().Add(45 * time.Second)),
			want:      "45s",
		},
		{
			name:      "task completed in minutes",
			startTime: time.Now(),
			endTime:   timePtr(time.Now().Add(2*time.Minute + 30*time.Second)),
			want:      "2m 30s",
		},
		{
			name:      "task completed in exactly 1 minute",
			startTime: time.Now(),
			endTime:   timePtr(time.Now().Add(1 * time.Minute)),
			want:      "1m 0s",
		},
		{
			name:      "task completed instantly",
			startTime: time.Now(),
			endTime:   timePtr(time.Now()),
			want:      "0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &CommentState{
				StartTime: tt.startTime,
				EndTime:   tt.endTime,
			}
			got := s.Duration()
			if got != tt.want {
				t.Errorf("Duration() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCommentState_StatusChecks(t *testing.T) {
	tests := []struct {
		name          string
		status        CommentStatus
		wantProgress  bool
		wantCompleted bool
		wantFailed    bool
	}{
		{
			name:          "working status",
			status:        StatusWorking,
			wantProgress:  true,
			wantCompleted: false,
			wantFailed:    false,
		},
		{
			name:          "completed status",
			status:        StatusCompleted,
			wantProgress:  false,
			wantCompleted: true,
			wantFailed:    false,
		},
		{
			name:          "failed status",
			status:        StatusFailed,
			wantProgress:  false,
			wantCompleted: false,
			wantFailed:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &CommentState{
				Status: tt.status,
			}

			if got := s.IsInProgress(); got != tt.wantProgress {
				t.Errorf("IsInProgress() = %v, want %v", got, tt.wantProgress)
			}
			if got := s.IsCompleted(); got != tt.wantCompleted {
				t.Errorf("IsCompleted() = %v, want %v", got, tt.wantCompleted)
			}
			if got := s.IsFailed(); got != tt.wantFailed {
				t.Errorf("IsFailed() = %v, want %v", got, tt.wantFailed)
			}
		})
	}
}

func TestCommentStatus_Constants(t *testing.T) {
	// Verify status constants are defined correctly
	if StatusWorking != "working" {
		t.Errorf("StatusWorking = %q, want 'working'", StatusWorking)
	}
	if StatusCompleted != "completed" {
		t.Errorf("StatusCompleted = %q, want 'completed'", StatusCompleted)
	}
	if StatusFailed != "failed" {
		t.Errorf("StatusFailed = %q, want 'failed'", StatusFailed)
	}
}

func TestCommentState_Fields(t *testing.T) {
	// Test that all fields can be set and retrieved
	now := time.Now()
	endTime := now.Add(1 * time.Minute)

	state := &CommentState{
		Status:        StatusCompleted,
		StartTime:     now,
		EndTime:       &endTime,
		CostUSD:       0.05,
		Username:      "testuser",
		OriginalBody:  "test prompt",
		Summary:       "test summary",
		ModifiedFiles: []string{"file1.go", "file2.go"},
		BranchName:    "test-branch",
		BranchURL:     "https://github.com/owner/repo/tree/test-branch",
		PRURL:         "https://github.com/owner/repo/pull/1",
		JobURL:        "https://github.com/owner/repo/actions/runs/123",
		ErrorDetails:  "",
	}

	// Verify fields
	if state.Status != StatusCompleted {
		t.Errorf("Status = %v, want %v", state.Status, StatusCompleted)
	}
	if state.CostUSD != 0.05 {
		t.Errorf("CostUSD = %v, want %v", state.CostUSD, 0.05)
	}
	if state.Username != "testuser" {
		t.Errorf("Username = %v, want %v", state.Username, "testuser")
	}
	if len(state.ModifiedFiles) != 2 {
		t.Errorf("ModifiedFiles length = %v, want %v", len(state.ModifiedFiles), 2)
	}
}

// Helper function to create time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}
