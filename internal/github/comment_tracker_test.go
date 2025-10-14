package github

import (
	"strings"
	"testing"
	"time"
)

func TestNewCommentTracker(t *testing.T) {
	tracker := NewCommentTracker("owner/repo", 123, "testuser")

	if tracker == nil {
		t.Fatal("NewCommentTracker returned nil")
	}

	if tracker.Repo != "owner/repo" {
		t.Errorf("Repo = %q, want %q", tracker.Repo, "owner/repo")
	}

	if tracker.Number != 123 {
		t.Errorf("Number = %d, want %d", tracker.Number, 123)
	}

	if tracker.CommentID != -1 {
		t.Errorf("CommentID = %d, want %d", tracker.CommentID, -1)
	}

	if tracker.State == nil {
		t.Fatal("State is nil")
	}

	if tracker.State.Status != StatusWorking {
		t.Errorf("Initial status = %v, want %v", tracker.State.Status, StatusWorking)
	}

	if tracker.State.Username != "testuser" {
		t.Errorf("Username = %q, want %q", tracker.State.Username, "testuser")
	}

	if tracker.State.Context == nil {
		t.Error("Context map should be initialized")
	}
}

func TestCommentTracker_SetMethods(t *testing.T) {
	tracker := NewCommentTracker("owner/repo", 123, "user")

	// Test SetWorking
	tracker.SetWorking()
	if tracker.State.Status != StatusWorking {
		t.Errorf("After SetWorking, status = %v, want %v", tracker.State.Status, StatusWorking)
	}

	// Test SetQueued
	trackerQueued := NewCommentTracker("owner/repo", 789, "user")
	trackerQueued.SetQueued()
	if trackerQueued.State.Status != StatusQueued {
		t.Errorf("After SetQueued, status = %v, want %v", trackerQueued.State.Status, StatusQueued)
	}

	// Test SetCompleted
	files := []string{"file1.go", "file2.go"}
	tracker.SetCompleted("test summary", files, 0.05)
	if tracker.State.Status != StatusCompleted {
		t.Errorf("After SetCompleted, status = %v, want %v", tracker.State.Status, StatusCompleted)
	}
	if tracker.State.Summary != "test summary" {
		t.Errorf("Summary = %q, want %q", tracker.State.Summary, "test summary")
	}
	if len(tracker.State.ModifiedFiles) != 2 {
		t.Errorf("ModifiedFiles length = %d, want %d", len(tracker.State.ModifiedFiles), 2)
	}
	if tracker.State.CostUSD != 0.05 {
		t.Errorf("CostUSD = %v, want %v", tracker.State.CostUSD, 0.05)
	}

	// Test SetFailed
	tracker2 := NewCommentTracker("owner/repo", 456, "user")
	tracker2.SetFailed("test error")
	if tracker2.State.Status != StatusFailed {
		t.Errorf("After SetFailed, status = %v, want %v", tracker2.State.Status, StatusFailed)
	}
	if tracker2.State.ErrorDetails != "test error" {
		t.Errorf("ErrorDetails = %q, want %q", tracker2.State.ErrorDetails, "test error")
	}

	// Test SetBranch
	tracker.SetBranch("test-branch", "https://github.com/owner/repo/tree/test-branch")
	if tracker.State.BranchName != "test-branch" {
		t.Errorf("BranchName = %q, want %q", tracker.State.BranchName, "test-branch")
	}
	if tracker.State.BranchURL != "https://github.com/owner/repo/tree/test-branch" {
		t.Errorf("BranchURL = %q, want %q", tracker.State.BranchURL, "https://github.com/owner/repo/tree/test-branch")
	}
	if tracker.State.Context["claude_branch"] != "test-branch" {
		t.Errorf("Context claude_branch = %q, want test-branch", tracker.State.Context["claude_branch"])
	}

	// Test SetPRURL
	tracker.SetPRURL("https://github.com/owner/repo/pull/1")
	if tracker.State.PRURL != "https://github.com/owner/repo/pull/1" {
		t.Errorf("PRURL = %q, want %q", tracker.State.PRURL, "https://github.com/owner/repo/pull/1")
	}

	// Test SetJobURL
	tracker.SetJobURL("https://github.com/owner/repo/actions/runs/123")
	if tracker.State.JobURL != "https://github.com/owner/repo/actions/runs/123" {
		t.Errorf("JobURL = %q, want %q", tracker.State.JobURL, "https://github.com/owner/repo/actions/runs/123")
	}
}

func TestCommentTracker_MarkEnd(t *testing.T) {
	tracker := NewCommentTracker("owner/repo", 123, "user")

	if tracker.State.EndTime != nil {
		t.Error("EndTime should be nil initially")
	}

	tracker.MarkEnd()

	if tracker.State.EndTime == nil {
		t.Error("EndTime should not be nil after MarkEnd")
	}
}

func TestCommentTracker_BuildHeader(t *testing.T) {
	tests := []struct {
		name         string
		status       CommentStatus
		username     string
		duration     string
		wantContains []string
	}{
		{
			name:     "working status",
			status:   StatusWorking,
			username: "alice",
			wantContains: []string{
				"SWE Agent is working",
				"@alice",
			},
		},
		{
			name:     "queued status",
			status:   StatusQueued,
			username: "zoe",
			wantContains: []string{
				"queued",
				"@zoe",
			},
		},
		{
			name:     "completed status without duration",
			status:   StatusCompleted,
			username: "bob",
			wantContains: []string{
				"SWE Agent finished",
				"@bob",
			},
		},
		{
			name:     "failed status",
			status:   StatusFailed,
			username: "charlie",
			wantContains: []string{
				"SWE Agent encountered an error",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewCommentTracker("owner/repo", 123, tt.username)
			tracker.State.Status = tt.status

			if tt.duration != "" {
				// Set duration for completed/failed states
				start := time.Now()
				end := start.Add(2*time.Minute + 30*time.Second)
				tracker.State.StartTime = start
				tracker.State.EndTime = &end
			}

			header := tracker.buildHeader()

			for _, substr := range tt.wantContains {
				if !strings.Contains(header, substr) {
					t.Errorf("buildHeader() = %q, want to contain %q", header, substr)
				}
			}
		})
	}
}

func TestCommentTracker_BuildLinks(t *testing.T) {
	tests := []struct {
		name         string
		branchName   string
		branchURL    string
		prURL        string
		jobURL       string
		createdPRs   []CreatedPR
		wantContains []string
		wantEmpty    bool
	}{
		{
			name:      "no links",
			wantEmpty: true,
		},
		{
			name:       "branch link only",
			branchName: "test-branch",
			branchURL:  "https://github.com/owner/repo/tree/test-branch",
			wantContains: []string{
				"——",
				"`test-branch`",
			},
		},
		{
			name:  "PR link only",
			prURL: "https://github.com/owner/repo/pull/1",
			wantContains: []string{
				"——",
				"Create PR ➔",
			},
		},
		{
			name:       "all links",
			branchName: "feature",
			branchURL:  "https://github.com/owner/repo/tree/feature",
			prURL:      "https://github.com/owner/repo/pull/2",
			jobURL:     "https://github.com/owner/repo/actions/runs/123",
			wantContains: []string{
				"——",
				"`feature`",
				"Create PR ➔",
				"View job",
				"•",
			},
		},
		{
			name: "multiple split PR links",
			createdPRs: []CreatedPR{
				{Index: 0, Name: "Add tests", URL: "https://github.com/owner/repo/pulls/10", Status: "created"},
				{Index: 1, Name: "Add docs", URL: "https://github.com/owner/repo/pulls/11", Status: "created"},
			},
			wantContains: []string{
				"Create PR: Add tests",
				"Create PR: Add docs",
				"•",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewCommentTracker("owner/repo", 123, "user")
			tracker.State.BranchName = tt.branchName
			tracker.State.BranchURL = tt.branchURL
			tracker.State.PRURL = tt.prURL
			tracker.State.JobURL = tt.jobURL
			tracker.State.CreatedPRs = tt.createdPRs

			links := tracker.buildLinks()

			if tt.wantEmpty {
				if links != "" {
					t.Errorf("buildLinks() = %q, want empty string", links)
				}
				return
			}

			for _, substr := range tt.wantContains {
				if !strings.Contains(links, substr) {
					t.Errorf("buildLinks() = %q, want to contain %q", links, substr)
				}
			}
		})
	}
}

func TestCommentTracker_BuildModifiedFilesList(t *testing.T) {
	tracker := NewCommentTracker("owner/repo", 123, "user")
	tracker.State.ModifiedFiles = []string{"file1.go", "file2.go", "file3.go"}

	list := tracker.buildModifiedFilesList()

	expectedSubstrings := []string{
		"**Modified Files:** (3)",
		"`file1.go`",
		"`file2.go`",
		"`file3.go`",
	}

	for _, substr := range expectedSubstrings {
		if !strings.Contains(list, substr) {
			t.Errorf("buildModifiedFilesList() missing %q", substr)
		}
	}
}

func TestCommentTracker_BuildFooter(t *testing.T) {
	tests := []struct {
		name         string
		status       CommentStatus
		costUSD      float64
		wantContains []string
	}{
		{
			name:    "completed with cost",
			status:  StatusCompleted,
			costUSD: 0.0234,
			wantContains: []string{
				"Generated with [SWE Agent](https://github.com/cexll/swe-agent)",
				"Cost: $0.0234",
			},
		},
		{
			name:    "completed without cost",
			status:  StatusCompleted,
			costUSD: 0,
			wantContains: []string{
				"Generated with [SWE Agent](https://github.com/cexll/swe-agent)",
			},
		},
		{
			name:   "failed status",
			status: StatusFailed,
			wantContains: []string{
				"Generated with [SWE Agent](https://github.com/cexll/swe-agent)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewCommentTracker("owner/repo", 123, "user")
			tracker.State.Status = tt.status
			tracker.State.CostUSD = tt.costUSD

			footer := tracker.buildFooter()

			for _, substr := range tt.wantContains {
				if !strings.Contains(footer, substr) {
					t.Errorf("buildFooter() = %q, want to contain %q", footer, substr)
				}
			}
		})
	}
}

func TestCommentTracker_RenderBody(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(*CommentTracker)
		wantContains []string
	}{
		{
			name: "working state",
			setup: func(tracker *CommentTracker) {
				tracker.State.OriginalBody = "Fix the bug"
			},
			wantContains: []string{
				"SWE Agent is working on @testuser's task <img src=\"https://github.githubassets.com/images/spinners/octocat-spinner-32.gif\" width=\"20\" height=\"20\" alt=\"loading\" />",
			},
		},
		{
			name: "completed state with files",
			setup: func(tracker *CommentTracker) {
				tracker.SetCompleted("Fixed the bug", []string{"main.go", "utils.go"}, 0.05)
				tracker.SetBranch("fix-branch", "https://github.com/owner/repo/tree/fix-branch")
				tracker.SetPRURL("https://github.com/owner/repo/pull/1")
				start := time.Now()
				end := start.Add(2 * time.Minute)
				tracker.State.StartTime = start
				tracker.State.EndTime = &end
			},
			wantContains: []string{
				"**SWE Agent finished @testuser's task in 2m 0s**",
				"[`fix-branch`](https://github.com/owner/repo/tree/fix-branch)",
				"[Create PR ➔](https://github.com/owner/repo/pull/1)",
				"Fixed the bug",
				"**Modified Files:** (2)",
				"`main.go`",
				"`utils.go`",
				"Generated with [SWE Agent](https://github.com/cexll/swe-agent)",
				"Cost: $0.0500",
			},
		},
		{
			name: "failed state",
			setup: func(tracker *CommentTracker) {
				tracker.SetFailed("Something went wrong")
				start := time.Now()
				end := start.Add(30 * time.Second)
				tracker.State.StartTime = start
				tracker.State.EndTime = &end
			},
			wantContains: []string{
				"**SWE Agent encountered an error after 30s**",
				"```",
				"Something went wrong",
				"Generated with [SWE Agent](https://github.com/cexll/swe-agent)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewCommentTracker("owner/repo", 123, "testuser")
			tt.setup(tracker)

			body := tracker.renderBody()

			for _, substr := range tt.wantContains {
				if !strings.Contains(body, substr) {
					t.Errorf("renderBody() missing %q\nGot:\n%s", substr, body)
				}
			}

		})
	}
}

func TestCommentTracker_UpdateWithoutCreate(t *testing.T) {
	tracker := NewCommentTracker("owner/repo", 123, "user")

	// Try to update before creating
	err := tracker.Update("test-token")
	if err == nil {
		t.Error("Update() should return error when comment not yet created")
	}

	if !strings.Contains(err.Error(), "not yet created") {
		t.Errorf("Update() error = %v, want to contain 'not yet created'", err)
	}
}

func TestCommentTracker_WorkingBodyExact(t *testing.T) {
	tracker := NewCommentTracker("owner/repo", 77, "alice")
	body := tracker.renderBody()
	want := "SWE Agent is working on @alice's task <img src=\"https://github.githubassets.com/images/spinners/octocat-spinner-32.gif\" width=\"20\" height=\"20\" alt=\"loading\" />"
	if body != want {
		t.Fatalf("Working body mismatch:\n got: %q\nwant: %q", body, want)
	}
}

func TestCommentTracker_QueuedBodyExact(t *testing.T) {
	tracker := NewCommentTracker("owner/repo", 55, "zoe")
	tracker.SetQueued()
	body := tracker.renderBody()
	want := "SWE Agent is queued for @zoe's task <img src=\"https://github.githubassets.com/images/spinners/octocat-spinner-32.gif\" width=\"20\" height=\"20\" alt=\"loading\" />"
	if body != want {
		t.Fatalf("Queued body mismatch:\n got: %q\nwant: %q", body, want)
	}
}

func TestCommentTracker_BuildHeaderWithDuration(t *testing.T) {
	tracker := NewCommentTracker("owner/repo", 123, "user")

	// Test completed with duration
	start := time.Now()
	end := start.Add(2*time.Minute + 30*time.Second)
	tracker.State.StartTime = start
	tracker.State.EndTime = &end
	tracker.State.Status = StatusCompleted

	header := tracker.buildHeader()

	if !strings.Contains(header, "2m 30s") {
		t.Errorf("buildHeader() = %q, want to contain duration '2m 30s'", header)
	}

	// Test failed with duration
	tracker2 := NewCommentTracker("owner/repo", 456, "user2")
	start2 := time.Now()
	end2 := start2.Add(45 * time.Second)
	tracker2.State.StartTime = start2
	tracker2.State.EndTime = &end2
	tracker2.State.Status = StatusFailed

	header2 := tracker2.buildHeader()

	if !strings.Contains(header2, "45s") {
		t.Errorf("buildHeader() = %q, want to contain duration '45s'", header2)
	}
}

func TestCommentTracker_BuildLinksWithBranchOnly(t *testing.T) {
	tracker := NewCommentTracker("owner/repo", 123, "user")
	tracker.State.BranchName = "test-branch"
	// No BranchURL

	links := tracker.buildLinks()

	if !strings.Contains(links, "`test-branch`") {
		t.Errorf("buildLinks() = %q, want to contain branch name", links)
	}

	// Should not have link syntax since no URL
	if strings.Count(links, "(") != 0 || strings.Count(links, ")") != 0 {
		t.Errorf("buildLinks() = %q, should not have link syntax without URL", links)
	}
}

func TestCommentTracker_RenderBodyEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*CommentTracker)
		check func(string) bool
	}{
		{
			name: "completed with empty cost",
			setup: func(tracker *CommentTracker) {
				tracker.SetCompleted("Done", []string{}, 0)
			},
			check: func(body string) bool {
				return strings.Contains(body, "SWE Agent finished") &&
					strings.Contains(body, "Done") &&
					!strings.Contains(body, "Cost")
			},
		},
		{
			name: "failed with empty error details",
			setup: func(tracker *CommentTracker) {
				tracker.SetFailed("")
			},
			check: func(body string) bool {
				return strings.Contains(body, "SWE Agent encountered an error")
			},
		},
		{
			name: "working with original body",
			setup: func(tracker *CommentTracker) {
				tracker.State.OriginalBody = "Original request text"
			},
			check: func(body string) bool {
				return strings.Contains(body, "SWE Agent is working")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewCommentTracker("owner/repo", 123, "user")
			tt.setup(tracker)

			body := tracker.renderBody()

			if !tt.check(body) {
				t.Errorf("renderBody() check failed for %s\nGot:\n%s", tt.name, body)
			}
		})
	}
}

func TestCommentTracker_RenderBody_NoPromptDump(t *testing.T) {
	tracker := NewCommentTracker("owner/repo", 123, "user")
	tracker.State.OriginalBody = "Fix the bug"

	body := tracker.renderBody()

	forbidden := []string{
		"Follow these steps:",
		"<file path=\"path/to/file\">",
		"mcp__github_comment__update_claude_comment",
	}
	for _, s := range forbidden {
		if strings.Contains(body, s) {
			t.Fatalf("working comment should not leak prompt content: found %q in body\n%s", s, body)
		}
	}
}

func TestCommentTracker_EmptyUsername(t *testing.T) {
	tracker := NewCommentTracker("owner/repo", 123, "")

	header := tracker.buildHeader()

	// Should use "user" as default
	if !strings.Contains(header, "@user") {
		t.Errorf("buildHeader() with empty username should use @user, got: %s", header)
	}
}

func TestNewCommentTrackerWithClient(t *testing.T) {
	mockClient := NewMockGHClient()
	tracker := NewCommentTrackerWithClient("owner/repo", 456, "user2", mockClient)

	if tracker.Repo != "owner/repo" {
		t.Errorf("Repo = %s, want owner/repo", tracker.Repo)
	}
	if tracker.Number != 456 {
		t.Errorf("Number = %d, want 456", tracker.Number)
	}
	if tracker.CommentID != -1 {
		t.Errorf("CommentID = %d, want -1", tracker.CommentID)
	}
	if tracker.State.Username != "user2" {
		t.Errorf("Username = %s, want user2", tracker.State.Username)
	}
	if tracker.ghClient != mockClient {
		t.Error("ghClient should be the mock client")
	}
}

func TestCommentTracker_CreateSuccess(t *testing.T) {
	mockClient := NewMockGHClient()
	mockClient.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 99999, nil
	}

	tracker := NewCommentTrackerWithClient("test/repo", 100, "alice", mockClient)
	tracker.State.OriginalBody = "Test task"

	err := tracker.Create("test-token")

	if err != nil {
		t.Errorf("Create() unexpected error = %v", err)
	}

	if tracker.CommentID != 99999 {
		t.Errorf("CommentID = %d, want 99999", tracker.CommentID)
	}

	if len(mockClient.CreateCommentCalls) != 1 {
		t.Fatalf("Expected 1 CreateComment call, got %d", len(mockClient.CreateCommentCalls))
	}

	call := mockClient.CreateCommentCalls[0]
	if call.Repo != "test/repo" {
		t.Errorf("CreateComment repo = %s, want test/repo", call.Repo)
	}
	if call.Number != 100 {
		t.Errorf("CreateComment number = %d, want 100", call.Number)
	}
	if !strings.Contains(call.Body, "SWE Agent is working") {
		t.Error("Comment body should contain working status")
	}
	if !strings.Contains(call.Body, "@alice") {
		t.Error("Comment body should mention username @alice")
	}
}

func TestCommentTracker_UpdateWithMockClient(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*MockGHClient)
		setupState  func(*CommentTracker)
		expectError bool
		verifyBody  func(*testing.T, string)
	}{
		{
			name: "update working status",
			setupMock: func(m *MockGHClient) {
				m.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
					return nil
				}
			},
			setupState: func(t *CommentTracker) {
				t.CommentID = 555
				t.State.Status = StatusWorking
			},
			expectError: false,
			verifyBody: func(t *testing.T, body string) {
				if !strings.Contains(body, "working") {
					t.Error("Body should contain working status")
				}
			},
		},
		{
			name: "update to completed",
			setupMock: func(m *MockGHClient) {
				m.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
					return nil
				}
			},
			setupState: func(t *CommentTracker) {
				t.CommentID = 666
				t.SetCompleted("Task finished", []string{"main.go", "utils.go"}, 0.05)
			},
			expectError: false,
			verifyBody: func(t *testing.T, body string) {
				if !strings.Contains(body, "finished") {
					t.Error("Body should contain finished status")
				}
				if !strings.Contains(body, "main.go") {
					t.Error("Body should list modified files")
				}
			},
		},
		{
			name: "update to failed",
			setupMock: func(m *MockGHClient) {
				m.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
					return nil
				}
			},
			setupState: func(t *CommentTracker) {
				t.CommentID = 777
				t.SetFailed("Something went wrong")
			},
			expectError: false,
			verifyBody: func(t *testing.T, body string) {
				if !strings.Contains(body, "error") {
					t.Error("Body should mention error")
				}
				if !strings.Contains(body, "Something went wrong") {
					t.Error("Body should contain error details")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := NewMockGHClient()
			tt.setupMock(mockClient)

			tracker := NewCommentTrackerWithClient("owner/repo", 200, "bob", mockClient)
			tt.setupState(tracker)

			err := tracker.Update("test-token")

			if (err != nil) != tt.expectError {
				t.Errorf("Update() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !tt.expectError {
				if len(mockClient.UpdateCommentCalls) != 1 {
					t.Fatalf("Expected 1 UpdateComment call, got %d", len(mockClient.UpdateCommentCalls))
				}

				call := mockClient.UpdateCommentCalls[0]
				if call.Repo != "owner/repo" {
					t.Errorf("UpdateComment repo = %s, want owner/repo", call.Repo)
				}

				if tt.verifyBody != nil {
					tt.verifyBody(t, call.Body)
				}
			}
		})
	}
}

func TestCommentTracker_CreateAndUpdateFlow(t *testing.T) {
	mockClient := NewMockGHClient()
	mockClient.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 12345, nil
	}
	mockClient.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return nil
	}

	tracker := NewCommentTrackerWithClient("flow/test", 999, "testuser", mockClient)

	// Create comment
	err := tracker.Create("token1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Update to completed
	tracker.SetCompleted("Work done", []string{"file.go"}, 0.01)
	err = tracker.Update("token2")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify calls
	if len(mockClient.CreateCommentCalls) != 1 {
		t.Errorf("Expected 1 Create call, got %d", len(mockClient.CreateCommentCalls))
	}
	if len(mockClient.UpdateCommentCalls) != 1 {
		t.Errorf("Expected 1 Update call, got %d", len(mockClient.UpdateCommentCalls))
	}
}
