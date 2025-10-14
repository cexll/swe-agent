package executor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/provider/claude"
	"github.com/cexll/swe/internal/taskstore"
	"github.com/cexll/swe/internal/webhook"
)

// mockProvider is a mock implementation of provider.Provider
type mockProvider struct {
	generateFunc func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error)
	name         string
}

func (m *mockProvider) GenerateCode(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
	if m.generateFunc != nil {
		return m.generateFunc(ctx, req)
	}
	return &claude.CodeResponse{
		Files: []claude.FileChange{
			{Path: "test.go", Content: "package test"},
		},
		Summary: "Test changes",
		CostUSD: 0.001,
	}, nil
}

func (m *mockProvider) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock"
}

func TestNew(t *testing.T) {
	provider := &mockProvider{}
	executor := New(provider, nil)

	if executor == nil {
		t.Fatal("New() returned nil")
	}

	if executor.provider != provider {
		t.Error("New() did not set provider correctly")
	}
}

func TestApplyChanges(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		changes []claude.FileChange
		wantErr bool
	}{
		{
			name: "single file",
			changes: []claude.FileChange{
				{Path: "main.go", Content: "package main\n\nfunc main() {}\n"},
			},
			wantErr: false,
		},
		{
			name: "multiple files",
			changes: []claude.FileChange{
				{Path: "main.go", Content: "package main"},
				{Path: "utils.go", Content: "package utils"},
			},
			wantErr: false,
		},
		{
			name: "nested directory",
			changes: []claude.FileChange{
				{Path: "pkg/utils/helper.go", Content: "package utils"},
			},
			wantErr: false,
		},
		{
			name: "overwrite existing file",
			changes: []claude.FileChange{
				{Path: "existing.go", Content: "new content"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh test directory
			testDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			// For overwrite test, create existing file
			if tt.name == "overwrite existing file" {
				existingPath := filepath.Join(testDir, "existing.go")
				if err := os.WriteFile(existingPath, []byte("old content"), 0644); err != nil {
					t.Fatalf("Failed to create existing file: %v", err)
				}
			}

			executor := &Executor{}
			err := executor.applyChanges(testDir, tt.changes)

			if (err != nil) != tt.wantErr {
				t.Errorf("applyChanges() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify files were created
			if !tt.wantErr {
				for _, change := range tt.changes {
					filePath := filepath.Join(testDir, change.Path)
					content, err := os.ReadFile(filePath)
					if err != nil {
						t.Errorf("Failed to read file %s: %v", change.Path, err)
						continue
					}
					if string(content) != change.Content {
						t.Errorf("File %s content = %q, want %q", change.Path, string(content), change.Content)
					}
				}
			}
		})
	}
}

func TestCreatePRLink(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		name     string
		repo     string
		head     string
		base     string
		title    string
		wantURL  string
		checkURL func(string) bool
	}{
		{
			name:  "basic PR link",
			repo:  "owner/repo",
			head:  "feature-branch",
			base:  "main",
			title: "Add new feature",
			checkURL: func(url string) bool {
				return len(url) > 0 &&
					url[:len("https://github.com/owner/repo/compare/")] == "https://github.com/owner/repo/compare/" &&
					contains(url, "main...feature-branch") &&
					contains(url, "expand=1")
			},
		},
		{
			name:  "title with spaces",
			repo:  "owner/repo",
			head:  "fix-bug",
			base:  "develop",
			title: "Fix login bug",
			checkURL: func(url string) bool {
				return contains(url, "Fix+login+bug") || contains(url, "title=")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := executor.createPRLink(tt.repo, tt.head, tt.base, tt.title)
			if err != nil {
				t.Errorf("createPRLink() error = %v", err)
				return
			}

			if tt.checkURL != nil && !tt.checkURL(url) {
				t.Errorf("createPRLink() URL = %s does not match expected pattern", url)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestCommitAndPush_PathValidation(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		name          string
		workdir       string
		branchName    string
		commitMessage string
		expectPanic   bool
	}{
		{
			name:          "valid parameters",
			workdir:       "/tmp/test",
			branchName:    "feature-branch",
			commitMessage: "Add feature",
			expectPanic:   false,
		},
		{
			name:          "empty branch name",
			workdir:       "/tmp/test",
			branchName:    "",
			commitMessage: "Message",
			expectPanic:   false,
		},
		{
			name:          "empty commit message",
			workdir:       "/tmp/test",
			branchName:    "feature",
			commitMessage: "",
			expectPanic:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// commitAndPush will fail in test environment without git,
			// but we're testing parameter validation
			err := executor.commitAndPush(tt.workdir, tt.branchName, tt.commitMessage, true)
			// Error is expected since we don't have a git repo
			_ = err
		})
	}
}

func TestApplyChanges_ErrorHandling(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		name    string
		changes []claude.FileChange
		setup   func() string
		wantErr bool
	}{
		{
			name: "invalid path with null bytes",
			changes: []claude.FileChange{
				{Path: "test\x00.go", Content: "content"},
			},
			setup: func() string {
				return t.TempDir()
			},
			wantErr: true,
		},
		{
			name: "path traversal attempt",
			changes: []claude.FileChange{
				{Path: "../../../etc/passwd", Content: "malicious"},
			},
			setup: func() string {
				return t.TempDir()
			},
			wantErr: true,
		},
		{
			name: "normalized relative path remains inside workdir",
			changes: []claude.FileChange{
				{Path: "nested/../safe.txt", Content: "safe"},
			},
			setup: func() string {
				return t.TempDir()
			},
			wantErr: false,
		},
		{
			name: "absolute path rejected",
			changes: []claude.FileChange{
				{Path: "/tmp/hijack.go", Content: "nope"},
			},
			setup: func() string {
				return t.TempDir()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workdir := tt.setup()
			err := executor.applyChanges(workdir, tt.changes)

			if (err != nil) != tt.wantErr {
				t.Errorf("applyChanges() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreatePRLink_URLFormat(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		name       string
		repo       string
		head       string
		base       string
		title      string
		wantSubstr []string
	}{
		{
			name:  "standard format",
			repo:  "owner/repo",
			head:  "feature",
			base:  "main",
			title: "Add feature",
			wantSubstr: []string{
				"https://github.com/owner/repo/compare/",
				"main...feature",
				"expand=1",
				"title=Add+feature",
			},
		},
		{
			name:  "special characters in title",
			repo:  "owner/repo",
			head:  "fix",
			base:  "develop",
			title: "Fix bug #123",
			wantSubstr: []string{
				"github.com",
				"compare",
				"title=Fix+bug+%23123",
			},
		},
		{
			name:  "branch with slash encodes correctly",
			repo:  "owner/repo",
			head:  "swe/issue-123-456",
			base:  "release/v1",
			title: "Feature work",
			wantSubstr: []string{
				"release%2Fv1...swe%2Fissue-123-456",
				"title=Feature+work",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := executor.createPRLink(tt.repo, tt.head, tt.base, tt.title)
			if err != nil {
				t.Errorf("createPRLink() error = %v", err)
				return
			}

			for _, substr := range tt.wantSubstr {
				if !contains(url, substr) {
					t.Errorf("createPRLink() URL = %s, want to contain %q", url, substr)
				}
			}
		})
	}
}

func TestProvider_Interface(t *testing.T) {
	// Test that mockProvider implements the interface correctly
	var _ interface {
		GenerateCode(context.Context, *claude.CodeRequest) (*claude.CodeResponse, error)
		Name() string
	} = (*mockProvider)(nil)

	provider := &mockProvider{
		name: "test-provider",
	}

	if provider.Name() != "test-provider" {
		t.Errorf("Name() = %s, want test-provider", provider.Name())
	}

	// Test default generate function
	resp, err := provider.GenerateCode(context.Background(), &claude.CodeRequest{})
	if err != nil {
		t.Errorf("GenerateCode() error = %v", err)
	}
	if resp == nil {
		t.Error("GenerateCode() returned nil response")
	}
}

func TestDetectGitChanges(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		name        string
		setup       func() (string, error)
		wantChanges bool
		wantErr     bool
	}{
		{
			name: "no changes in clean repo",
			setup: func() (string, error) {
				tmpDir := t.TempDir()
				// Initialize git repo
				if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("initial"), 0644); err != nil {
					return "", err
				}
				cmds := [][]string{
					{"git", "init"},
					{"git", "config", "user.name", "Test"},
					{"git", "config", "user.email", "test@test.com"},
					{"git", "add", "."},
					{"git", "commit", "-m", "initial"},
				}
				for _, args := range cmds {
					cmd := exec.Command(args[0], args[1:]...)
					cmd.Dir = tmpDir
					if err := cmd.Run(); err != nil {
						return "", err
					}
				}
				return tmpDir, nil
			},
			wantChanges: false,
			wantErr:     false,
		},
		{
			name: "modified file detected",
			setup: func() (string, error) {
				tmpDir := t.TempDir()
				// Initialize git repo
				if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("initial"), 0644); err != nil {
					return "", err
				}
				cmds := [][]string{
					{"git", "init"},
					{"git", "config", "user.name", "Test"},
					{"git", "config", "user.email", "test@test.com"},
					{"git", "add", "."},
					{"git", "commit", "-m", "initial"},
				}
				for _, args := range cmds {
					cmd := exec.Command(args[0], args[1:]...)
					cmd.Dir = tmpDir
					if err := cmd.Run(); err != nil {
						return "", err
					}
				}
				// Modify file
				if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("modified"), 0644); err != nil {
					return "", err
				}
				return tmpDir, nil
			},
			wantChanges: true,
			wantErr:     false,
		},
		{
			name: "new file detected",
			setup: func() (string, error) {
				tmpDir := t.TempDir()
				// Initialize git repo
				cmds := [][]string{
					{"git", "init"},
					{"git", "config", "user.name", "Test"},
					{"git", "config", "user.email", "test@test.com"},
				}
				for _, args := range cmds {
					cmd := exec.Command(args[0], args[1:]...)
					cmd.Dir = tmpDir
					if err := cmd.Run(); err != nil {
						return "", err
					}
				}
				// Add new file
				if err := os.WriteFile(filepath.Join(tmpDir, "new.txt"), []byte("new"), 0644); err != nil {
					return "", err
				}
				return tmpDir, nil
			},
			wantChanges: true,
			wantErr:     false,
		},
		{
			name: "non-git directory",
			setup: func() (string, error) {
				return t.TempDir(), nil
			},
			wantChanges: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workdir, err := tt.setup()
			if err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			hasChanges, err := executor.detectGitChanges(workdir)

			if (err != nil) != tt.wantErr {
				t.Errorf("detectGitChanges() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && hasChanges != tt.wantChanges {
				t.Errorf("detectGitChanges() hasChanges = %v, want %v", hasChanges, tt.wantChanges)
			}
		})
	}
}

func TestExtractFilePaths(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		name  string
		files []claude.FileChange
		want  []string
	}{
		{
			name:  "empty files",
			files: []claude.FileChange{},
			want:  []string{},
		},
		{
			name: "single file",
			files: []claude.FileChange{
				{Path: "main.go", Content: "package main"},
			},
			want: []string{"main.go"},
		},
		{
			name: "multiple files",
			files: []claude.FileChange{
				{Path: "main.go", Content: "package main"},
				{Path: "utils.go", Content: "package utils"},
				{Path: "test.go", Content: "package test"},
			},
			want: []string{"main.go", "utils.go", "test.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := executor.extractFilePaths(tt.files)

			if len(got) != len(tt.want) {
				t.Errorf("extractFilePaths() returned %d paths, want %d", len(got), len(tt.want))
				return
			}

			for i, path := range got {
				if path != tt.want[i] {
					t.Errorf("extractFilePaths()[%d] = %q, want %q", i, path, tt.want[i])
				}
			}
		})
	}
}

func TestHandleError(t *testing.T) {
	executor := &Executor{}

	tracker := github.NewCommentTracker("owner/repo", 123, "testuser")
	errorMsg := "test error message"

	// handleError should return an error
	err := executor.handleError(nil, tracker, "test-token", errorMsg)

	if err == nil {
		t.Error("handleError() should return an error")
	}

	if err.Error() != errorMsg {
		t.Errorf("handleError() error = %q, want %q", err.Error(), errorMsg)
	}

	// Verify tracker state was updated
	if tracker.State.Status != github.StatusFailed {
		t.Errorf("After handleError, status = %v, want %v", tracker.State.Status, github.StatusFailed)
	}

	if tracker.State.ErrorDetails != errorMsg {
		t.Errorf("After handleError, errorDetails = %q, want %q", tracker.State.ErrorDetails, errorMsg)
	}

	if tracker.State.EndTime == nil {
		t.Error("After handleError, endTime should be set")
	}
}

func TestHandleError_NonRetryable(t *testing.T) {
	executor := &Executor{}

	tracker := github.NewCommentTracker("owner/repo", 123, "testuser")
	errorMsg := `Claude CLI error: claude CLI execution failed: exit status 1 (output preview: {"result":"API Error: 401 {\"error\":{\"message\":\"无效的令牌\"}}"})`

	err := executor.handleError(nil, tracker, "test-token", errorMsg)
	if err == nil {
		t.Fatal("expected non-nil error")
	}

	var nr *NonRetryableError
	if !errors.As(err, &nr) {
		t.Fatalf("expected NonRetryableError, got %T (%v)", err, err)
	}

	if nr.Error() != errorMsg {
		t.Fatalf("unexpected error message, got %q want %q", nr.Error(), errorMsg)
	}

	if tracker.State.Status != github.StatusFailed {
		t.Fatalf("expected tracker status failed, got %v", tracker.State.Status)
	}
}

func TestHandleResponseOnly(t *testing.T) {
	executor := &Executor{}

	tracker := github.NewCommentTracker("owner/repo", 123, "testuser")

	result := &claude.CodeResponse{
		Summary: "Analysis complete",
		CostUSD: 0.01,
		Files:   []claude.FileChange{}, // No files
	}

	// handleResponseOnly should return nil error
	err := executor.handleResponseOnly(nil, tracker, "test-token", result)

	if err != nil {
		t.Errorf("handleResponseOnly() unexpected error = %v", err)
	}

	// Verify tracker state was updated
	if tracker.State.Status != github.StatusCompleted {
		t.Errorf("After handleResponseOnly, status = %v, want %v", tracker.State.Status, github.StatusCompleted)
	}

	if tracker.State.Summary != "Analysis complete" {
		t.Errorf("After handleResponseOnly, summary = %q, want %q", tracker.State.Summary, "Analysis complete")
	}

	if tracker.State.CostUSD != 0.01 {
		t.Errorf("After handleResponseOnly, costUSD = %v, want %v", tracker.State.CostUSD, 0.01)
	}

	if len(tracker.State.ModifiedFiles) != 0 {
		t.Errorf("After handleResponseOnly, modifiedFiles should be empty, got %d files", len(tracker.State.ModifiedFiles))
	}

	if tracker.State.EndTime == nil {
		t.Error("After handleResponseOnly, endTime should be set")
	}
}

// mockAppAuth is a mock implementation of github.AuthProvider
type mockAppAuth struct {
	GetInstallationTokenFunc func(repo string) (*github.InstallationToken, error)
	GetInstallationOwnerFunc func(repo string) (string, error)
}

func (m *mockAppAuth) GetInstallationToken(repo string) (*github.InstallationToken, error) {
	if m.GetInstallationTokenFunc != nil {
		return m.GetInstallationTokenFunc(repo)
	}
	return &github.InstallationToken{
		Token:     "mock-token",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}, nil
}

func (m *mockAppAuth) GetInstallationOwner(repo string) (string, error) {
	if m.GetInstallationOwnerFunc != nil {
		return m.GetInstallationOwnerFunc(repo)
	}
	return "mock-owner", nil
}

func TestExecutor_Execute_SuccessWithFileChanges(t *testing.T) {
	// Setup mocks
	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 12345, nil
	}

	mockAuth := &mockAppAuth{}

	mockProvider := &mockProvider{
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			return &claude.CodeResponse{
				Files: []claude.FileChange{
					{Path: "main.go", Content: "package main\n\nfunc main() {}"},
				},
				Summary: "Added main function",
				CostUSD: 0.05,
			}, nil
		},
	}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)

	// Create task
	task := &webhook.Task{
		Repo:       "owner/repo",
		Number:     123,
		Branch:     "main",
		Prompt:     "Add main function",
		IssueTitle: "Feature request",
		IssueBody:  "Please add main",
		Username:   "testuser",
	}

	// Note: This test will fail at git operations since we don't have a real git repo
	// We're testing up to that point to verify the mock interactions
	err := executor.Execute(context.Background(), task)

	// We expect an error from git operations, not from our mocked parts
	if err != nil {
		// Verify it's a git/clone error, not a mock error
		if !strings.Contains(err.Error(), "clone") && !strings.Contains(err.Error(), "git") {
			t.Errorf("Expected git/clone error, got: %v", err)
		}
	}

	// Verify mock interactions
	if len(mockGH.CreateCommentCalls) != 1 {
		t.Errorf("Expected 1 CreateComment call, got %d", len(mockGH.CreateCommentCalls))
	}

	if len(mockGH.AddLabelCalls) != 1 {
		t.Errorf("Expected 1 AddLabel call, got %d", len(mockGH.AddLabelCalls))
	}

	// Verify CreateComment was called with correct params
	if len(mockGH.CreateCommentCalls) > 0 {
		call := mockGH.CreateCommentCalls[0]
		if call.Repo != "owner/repo" {
			t.Errorf("CreateComment repo = %s, want owner/repo", call.Repo)
		}
		if call.Number != 123 {
			t.Errorf("CreateComment number = %d, want 123", call.Number)
		}
	}
}

func TestExecutor_Execute_AuthenticationFailure(t *testing.T) {
	mockGH := github.NewMockGHClient()
	mockAuth := &mockAppAuth{
		GetInstallationTokenFunc: func(repo string) (*github.InstallationToken, error) {
			return nil, fmt.Errorf("authentication failed")
		},
	}
	mockProvider := &mockProvider{}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)

	task := &webhook.Task{
		Repo:   "owner/repo",
		Number: 123,
		Branch: "main",
		Prompt: "test",
	}

	err := executor.Execute(context.Background(), task)

	if err == nil {
		t.Error("Execute() should return error when authentication fails")
	}

	if !strings.Contains(err.Error(), "authentication") {
		t.Errorf("Error should mention authentication, got: %v", err)
	}

	// No gh operations should be called if auth fails
	if len(mockGH.CreateCommentCalls) != 0 {
		t.Errorf("CreateComment should not be called when auth fails, got %d calls", len(mockGH.CreateCommentCalls))
	}
}

func TestExecutor_Execute_ProviderError(t *testing.T) {
	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 12345, nil
	}

	mockAuth := &mockAppAuth{}

	mockProvider := &mockProvider{
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			return nil, fmt.Errorf("provider API error")
		},
	}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)

	task := &webhook.Task{
		Repo:     "owner/repo",
		Number:   123,
		Branch:   "main",
		Prompt:   "test",
		Username: "testuser",
	}

	// Since we have a real git clone dependency, we'll get an error earlier
	// This test verifies the provider integration
	err := executor.Execute(context.Background(), task)

	if err == nil {
		t.Error("Execute() should return error when provider fails")
	}

	// Comment should be created (tracking comment)
	if len(mockGH.CreateCommentCalls) == 0 {
		t.Error("CreateComment should be called to create tracking comment")
	}

	// Update comment should be called with error (if we got past clone)
	// Due to clone dependency, we might not reach this point in test
}

func TestExecutor_NewWithClient(t *testing.T) {
	mockGH := github.NewMockGHClient()
	mockAuth := &mockAppAuth{}
	mockProvider := &mockProvider{}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)

	if executor == nil {
		t.Fatal("NewWithClient() returned nil")
	}

	if executor.provider != mockProvider {
		t.Error("NewWithClient() did not set provider correctly")
	}

	if executor.appAuth != mockAuth {
		t.Error("NewWithClient() did not set appAuth correctly")
	}

	if executor.ghClient != mockGH {
		t.Error("NewWithClient() did not set ghClient correctly")
	}
}

func TestExecutor_HandleError_Integration(t *testing.T) {
	mockGH := github.NewMockGHClient()
	executor := &Executor{
		ghClient: mockGH,
	}

	tracker := github.NewCommentTrackerWithClient("owner/repo", 123, "user", mockGH)
	tracker.CommentID = 999 // Simulate already created comment

	err := executor.handleError(nil, tracker, "token", "test error message")

	// Should return the error
	if err == nil {
		t.Error("handleError() should return an error")
	}

	if !strings.Contains(err.Error(), "test error message") {
		t.Errorf("Error should contain message, got: %v", err)
	}

	// Should call UpdateComment
	if len(mockGH.UpdateCommentCalls) != 1 {
		t.Errorf("Expected 1 UpdateComment call, got %d", len(mockGH.UpdateCommentCalls))
	}

	// Verify tracker state
	if tracker.State.Status != github.StatusFailed {
		t.Errorf("Tracker status should be Failed, got %v", tracker.State.Status)
	}
}

func TestExecutor_HandleResponseOnly_Integration(t *testing.T) {
	mockGH := github.NewMockGHClient()
	executor := &Executor{
		ghClient: mockGH,
	}

	tracker := github.NewCommentTrackerWithClient("owner/repo", 123, "user", mockGH)
	tracker.CommentID = 888

	result := &claude.CodeResponse{
		Summary: "Analysis complete",
		CostUSD: 0.02,
		Files:   []claude.FileChange{},
	}

	err := executor.handleResponseOnly(nil, tracker, "token", result)

	if err != nil {
		t.Errorf("handleResponseOnly() unexpected error: %v", err)
	}

	// Should call UpdateComment
	if len(mockGH.UpdateCommentCalls) != 1 {
		t.Errorf("Expected 1 UpdateComment call, got %d", len(mockGH.UpdateCommentCalls))
	}

	// Verify tracker state
	if tracker.State.Status != github.StatusCompleted {
		t.Errorf("Tracker status should be Completed, got %v", tracker.State.Status)
	}

	if tracker.State.Summary != "Analysis complete" {
		t.Errorf("Tracker summary = %s, want 'Analysis complete'", tracker.State.Summary)
	}
}

func TestExecutor_ReusesExistingTrackingComment(t *testing.T) {
	mockGH := github.NewMockGHClient()
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return nil
	}
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		t.Errorf("CreateComment should not be called when comment ID already known")
		return 0, fmt.Errorf("unexpected create")
	}

	mockAuth := &mockAppAuth{
		GetInstallationTokenFunc: func(repo string) (*github.InstallationToken, error) {
			return &github.InstallationToken{Token: "token", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
	}

	executor := NewWithClient(&mockProvider{name: "mock"}, mockAuth, mockGH)
	executor.cloneFn = func(repo, branch string) (string, func(), error) {
		return "", func() {}, fmt.Errorf("clone failure")
	}

	task := &webhook.Task{
		Repo:          "owner/repo",
		Number:        42,
		Branch:        "main",
		Prompt:        "do something",
		Username:      "tester",
		PromptContext: map[string]string{"claude_comment_id": "12345"},
	}

	err := executor.Execute(context.Background(), task)
	if err == nil {
		t.Fatal("Execute() should propagate clone failure")
	}

	if len(mockGH.CreateCommentCalls) != 0 {
		t.Fatalf("CreateComment should not be called, got %d calls", len(mockGH.CreateCommentCalls))
	}

	if len(mockGH.UpdateCommentCalls) == 0 {
		t.Fatal("Expected UpdateComment to be called for existing tracking comment")
	}

	call := mockGH.UpdateCommentCalls[0]
	if call.CommentID != 12345 {
		t.Errorf("UpdateComment used commentID %d, want 12345", call.CommentID)
	}
}

func TestExecutor_Execute_ResponseOnlyFlow(t *testing.T) {
	// Ensure git is available for initializing repositories
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	tmpDir := t.TempDir()

	commands := [][]string{
		{"git", "init"},
		{"git", "config", "user.name", "Test Bot"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "checkout", "-b", "main"},
	}
	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpDir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git command failed: %v\n%s", err, output)
		}
	}

	readme := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readme, []byte("# repo"), 0o644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}
	if err := exec.Command("git", "-C", tmpDir, "add", ".").Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 123, nil
	}
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return nil
	}
	mockGH.AddLabelFunc = func(repo string, number int, label, token string) error {
		return fmt.Errorf("label failure")
	}

	mockAuth := &mockAppAuth{}
	mockProvider := &mockProvider{
		name: "analysis-only",
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			return &claude.CodeResponse{
				Files:   nil,
				Summary: "No code changes required",
				CostUSD: 0.01,
			}, nil
		},
	}

	store := taskstore.NewStore()
	executor := NewWithClient(mockProvider, mockAuth, mockGH)
	executor.WithStore(store)
	executor.cloneFn = func(repo, branch string) (string, func(), error) {
		return tmpDir, func() {}, nil
	}

	task := &webhook.Task{
		ID:            "task-42",
		Repo:          "owner/repo",
		Number:        42,
		Branch:        "main",
		Prompt:        "Provide analysis only",
		PromptSummary: "Summarized instructions",
		PromptContext: map[string]string{
			"claude_comment_id": "555",
		},
		Username: "analyst",
	}

	t.Setenv("DEBUG_GIT_DETECTION", "true")
	store.Create(&taskstore.Task{ID: task.ID})

	if err := executor.Execute(context.Background(), task); err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}

	if len(mockGH.UpdateCommentCalls) == 0 {
		t.Fatalf("expected tracking comment to be updated at least once")
	}
	if entry, ok := store.Get(task.ID); !ok || entry.Status != taskstore.StatusCompleted {
		t.Fatalf("store status = %+v, want completed entry", entry)
	}
}

func TestExecutor_Execute_SinglePRWorkflow(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	tmpRoot := t.TempDir()
	remoteDir := filepath.Join(tmpRoot, "remote.git")
	runGit(t, "", "git", "init", "--bare", remoteDir)

	seedDir := filepath.Join(tmpRoot, "seed")
	runGit(t, "", "git", "clone", remoteDir, seedDir)
	runGit(t, seedDir, "git", "config", "user.name", "Seed User")
	runGit(t, seedDir, "git", "config", "user.email", "seed@example.com")
	if err := exec.Command("git", "-C", seedDir, "checkout", "-b", "main").Run(); err != nil {
		runGit(t, seedDir, "git", "checkout", "main")
	}

	seedFile := filepath.Join(seedDir, "README.md")
	if err := os.WriteFile(seedFile, []byte("# seed"), 0o644); err != nil {
		t.Fatalf("failed to write seed file: %v", err)
	}
	runGit(t, seedDir, "git", "add", ".")
	runGit(t, seedDir, "git", "commit", "-m", "initial commit")
	runGit(t, seedDir, "git", "push", "-u", "origin", "main")

	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 321, nil
	}
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return nil
	}
	mockGH.AddLabelFunc = func(repo string, number int, label, token string) error {
		return nil
	}

	mockProvider := &mockProvider{
		name: "codegen",
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			return &claude.CodeResponse{
				Files: []claude.FileChange{
					{Path: "lib/service.go", Content: "package lib\n\nfunc Service() string { return \"ok\" }\n"},
				},
				Summary: "Add service helper",
				CostUSD: 0.02,
			}, nil
		},
	}

	mockAuth := &mockAppAuth{}
	store := taskstore.NewStore()
	executor := NewWithClient(mockProvider, mockAuth, mockGH)
	executor.WithStore(store)
	executor.cloneFn = func(repo, branch string) (string, func(), error) {
		cloneDir := filepath.Join(tmpRoot, fmt.Sprintf("clone-%d", time.Now().UnixNano()))
		if err := os.MkdirAll(cloneDir, 0o755); err != nil {
			return "", nil, err
		}
		cmd := exec.Command("git", "clone", "--branch", branch, remoteDir, cloneDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", nil, fmt.Errorf("git clone failed: %w\n%s", err, output)
		}
		return cloneDir, func() { os.RemoveAll(cloneDir) }, nil
	}

	task := &webhook.Task{
		ID:       "task-single-pr",
		Repo:     "owner/repo",
		Number:   7,
		Branch:   "main",
		Prompt:   "Implement service helper",
		Username: "builder",
	}

	store.Create(&taskstore.Task{ID: task.ID})

	if err := executor.Execute(context.Background(), task); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	entry, ok := store.Get(task.ID)
	if !ok || entry.Status != taskstore.StatusCompleted {
		t.Fatalf("task store status = %+v, want completed", entry)
	}

	// Verify that a new branch was pushed to the remote.
	listCmd := exec.Command("git", "-C", remoteDir, "for-each-ref", "--format=%(refname)", "refs/heads")
	output, err := listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to list remote branches: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "swe/") {
		t.Fatalf("remote branches = %s, expected swe branch", string(output))
	}
}

func TestExecutor_CommitSubPR_WritesFiles(t *testing.T) {
	var commands [][]string
	stubExecCommand(t, func(name string, args ...string) *exec.Cmd {
		cmd := append([]string{name}, args...)
		commands = append(commands, cmd)
		return exec.Command("bash", "-lc", "true")
	})

	tmpDir := t.TempDir()
	executor := &Executor{}
	subPR := github.SubPR{
		Name:        "Docs update",
		Description: "Update docs",
		Files: []claude.FileChange{
			{Path: "docs/readme.md", Content: "updated docs"},
		},
	}
	task := &webhook.Task{Prompt: "docs"}

	if err := executor.commitSubPR(tmpDir, "swe/docs", subPR, task); err != nil {
		t.Fatalf("commitSubPR error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "docs/readme.md"))
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	if string(content) != "updated docs" {
		t.Fatalf("file content = %q, want updated docs", string(content))
	}
	if len(commands) < 6 {
		t.Fatalf("expected git commands to run, got %d", len(commands))
	}
}

func TestExecutor_ExecuteMultiPR_CreatesAndSkips(t *testing.T) {
	stubExecCommand(t, func(string, ...string) *exec.Cmd {
		return exec.Command("bash", "-lc", "true")
	})

	mockGH := github.NewMockGHClient()
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error { return nil }

	executor := NewWithClient(&mockProvider{name: "multi"}, &mockAppAuth{}, mockGH)
	executor.WithStore(taskstore.NewStore())

	task := &webhook.Task{
		ID:       "multi",
		Repo:     "owner/repo",
		Number:   9,
		Branch:   "main",
		Username: "user",
	}

	subPRs := []github.SubPR{
		{
			Index:       0,
			Name:        "Docs PR",
			Description: "Update docs",
			Category:    github.CategoryDocs,
			Files: []claude.FileChange{
				{Path: "docs/readme.md", Content: "docs"},
			},
		},
		{
			Index:       1,
			Name:        "Tests PR",
			Description: "Update tests",
			Category:    github.CategoryTests,
			DependsOn:   []int{0},
			Files: []claude.FileChange{
				{Path: "tests/example_test.go", Content: "package tests"},
			},
		},
	}

	plan := &github.SplitPlan{
		SubPRs:        subPRs,
		CreationOrder: []int{0, 1},
	}

	tracker := github.NewCommentTrackerWithClient(task.Repo, task.Number, task.Username, mockGH)
	tracker.CommentID = 777

	result := &claude.CodeResponse{
		Summary: "Split summary",
		CostUSD: 0.15,
	}

	if err := executor.executeMultiPR(context.Background(), task, t.TempDir(), plan, result, tracker, "token"); err != nil {
		t.Fatalf("executeMultiPR error: %v", err)
	}

	if tracker.State.Status != github.StatusCompleted {
		t.Fatalf("tracker status = %s, want completed", tracker.State.Status)
	}
	if len(tracker.State.CreatedPRs) == 0 {
		t.Fatal("expected created PR records")
	}
}

func TestExecutor_GetChangedFiles_DirectoryAndMissing(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "dir"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "dir", "included.txt"), []byte("dir file"), 0o644); err != nil {
		t.Fatalf("write dir file failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "top.txt"), []byte("top"), 0o644); err != nil {
		t.Fatalf("write top file failed: %v", err)
	}

	stubExecCommand(t, func(name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) >= 2 && args[0] == "status" {
			return exec.Command("bash", "-lc", "printf '?? dir/\\n M top.txt\\n?? missing.txt\\n?? s\\n'")
		}
		return exec.Command("bash", "-lc", "true")
	})

	executor := &Executor{}
	changes, err := executor.getChangedFiles(tmpDir)
	if err != nil {
		t.Fatalf("getChangedFiles error: %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("changes len = %d, want 2", len(changes))
	}
	paths := []string{changes[0].Path, changes[1].Path}
	joined := strings.Join(paths, ",")
	if !strings.Contains(joined, "dir/included.txt") || !strings.Contains(joined, "top.txt") {
		t.Fatalf("unexpected paths: %v", paths)
	}
}

func runGit(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, output)
	}
}

func stubExecCommand(t *testing.T, handler func(name string, args ...string) *exec.Cmd) {
	t.Helper()
	original := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return handler(name, args...)
	}
	t.Cleanup(func() {
		execCommand = original
	})
}

func TestApplyChanges_AdditionalEdgeCases(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		name    string
		setup   func() (string, error)
		changes []claude.FileChange
		wantErr bool
		cleanup func(string)
	}{
		{
			name: "empty content file",
			setup: func() (string, error) {
				return t.TempDir(), nil
			},
			changes: []claude.FileChange{
				{Path: "empty.txt", Content: ""},
			},
			wantErr: false,
		},
		{
			name: "deeply nested path",
			setup: func() (string, error) {
				return t.TempDir(), nil
			},
			changes: []claude.FileChange{
				{Path: "a/b/c/d/e/f/deep.go", Content: "package deep"},
			},
			wantErr: false,
		},
		{
			name: "file with unicode name",
			setup: func() (string, error) {
				return t.TempDir(), nil
			},
			changes: []claude.FileChange{
				{Path: "测试文件.go", Content: "package test"},
			},
			wantErr: false,
		},
		{
			name: "large content file",
			setup: func() (string, error) {
				return t.TempDir(), nil
			},
			changes: []claude.FileChange{
				{Path: "large.txt", Content: strings.Repeat("x", 1024*1024)}, // 1MB
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workdir, err := tt.setup()
			if err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			if tt.cleanup != nil {
				defer tt.cleanup(workdir)
			}

			err = executor.applyChanges(workdir, tt.changes)

			if (err != nil) != tt.wantErr {
				t.Errorf("applyChanges() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify files were created if no error expected
			if !tt.wantErr {
				for _, change := range tt.changes {
					filePath := filepath.Join(workdir, change.Path)
					content, err := os.ReadFile(filePath)
					if err != nil {
						t.Errorf("Failed to read file %s: %v", change.Path, err)
						continue
					}
					if string(content) != change.Content {
						if len(change.Content) < 100 {
							t.Errorf("File %s content = %q, want %q", change.Path, string(content), change.Content)
						} else {
							t.Errorf("File %s content length = %d, want %d", change.Path, len(content), len(change.Content))
						}
					}
				}
			}
		})
	}
}

func TestCommitAndPush_EdgeCases(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		name          string
		setup         func() (string, error)
		branchName    string
		commitMessage string
		wantErrMsg    string
	}{
		{
			name: "empty commit message",
			setup: func() (string, error) {
				tmpDir := t.TempDir()
				// Initialize git repo
				cmds := [][]string{
					{"git", "init"},
					{"git", "config", "user.name", "Test"},
					{"git", "config", "user.email", "test@test.com"},
				}
				for _, args := range cmds {
					cmd := exec.Command(args[0], args[1:]...)
					cmd.Dir = tmpDir
					if err := cmd.Run(); err != nil {
						return "", err
					}
				}
				// Create initial commit
				if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0644); err != nil {
					return "", err
				}
				cmd := exec.Command("git", "add", ".")
				cmd.Dir = tmpDir
				if err := cmd.Run(); err != nil {
					return "", err
				}
				cmd = exec.Command("git", "commit", "-m", "init")
				cmd.Dir = tmpDir
				if err := cmd.Run(); err != nil {
					return "", err
				}
				return tmpDir, nil
			},
			branchName:    "test-branch",
			commitMessage: "",
			wantErrMsg:    "commit",
		},
		{
			name: "special characters in branch name",
			setup: func() (string, error) {
				tmpDir := t.TempDir()
				cmds := [][]string{
					{"git", "init"},
					{"git", "config", "user.name", "Test"},
					{"git", "config", "user.email", "test@test.com"},
				}
				for _, args := range cmds {
					cmd := exec.Command(args[0], args[1:]...)
					cmd.Dir = tmpDir
					if err := cmd.Run(); err != nil {
						return "", err
					}
				}
				return tmpDir, nil
			},
			branchName:    "test/branch-123",
			commitMessage: "test",
			wantErrMsg:    "", // Git allows slashes in branch names
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workdir, err := tt.setup()
			if err != nil {
				t.Skipf("Setup failed: %v (git may not be available)", err)
			}

			// Make a change to commit
			if err := os.WriteFile(filepath.Join(workdir, "new.txt"), []byte("new content"), 0644); err != nil {
				t.Fatalf("Failed to create file: %v", err)
			}

			if tt.branchName != "" {
				cmd := exec.Command("git", "checkout", "-b", tt.branchName)
				cmd.Dir = workdir
				if err := cmd.Run(); err != nil {
					t.Fatalf("Failed to create branch %s: %v", tt.branchName, err)
				}
			}

			err = executor.commitAndPush(workdir, tt.branchName, tt.commitMessage, true)

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Errorf("commitAndPush() should return error containing %q", tt.wantErrMsg)
				} else if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("commitAndPush() error = %v, want error containing %q", err, tt.wantErrMsg)
				}
			} else if err != nil && !strings.Contains(err.Error(), "push") {
				// Push failure is expected (no remote), but other errors are not
				t.Errorf("commitAndPush() unexpected error: %v", err)
			}
		})
	}
}

func TestCommitAndPush_LongCommitMessage(t *testing.T) {
	executor := &Executor{}

	tmpDir := t.TempDir()

	// Initialize git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "user.email", "test@test.com"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Skipf("Git setup failed: %v", err)
		}
	}

	// Create a file to commit
	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	checkoutCmd := exec.Command("git", "checkout", "-b", "test-long-msg")
	checkoutCmd.Dir = tmpDir
	if err := checkoutCmd.Run(); err != nil {
		t.Fatalf("Failed to create branch: %v", err)
	}

	// Test with very long commit message
	longMessage := strings.Repeat("This is a very long commit message. ", 50) // ~1850 chars

	err := executor.commitAndPush(tmpDir, "test-long-msg", longMessage, true)

	// Push will fail (no remote) but commit should succeed
	if err != nil && !strings.Contains(err.Error(), "push") {
		t.Errorf("commitAndPush() failed before push: %v", err)
	}

	// Verify commit was created
	cmd := exec.Command("git", "log", "--oneline", "-1")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to check git log: %v", err)
	}

	if len(output) == 0 {
		t.Error("No commit was created")
	}
}

// TestExecutor_Execute_HappyPath_WithMockClone tests the complete Execute() workflow with mocked clone
func TestExecutor_Execute_HappyPath_WithMockClone(t *testing.T) {
	// Setup: Create a real temporary git repo for the mock to return
	tmpDir := t.TempDir()

	// Initialize git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "user.email", "test@test.com"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Skipf("Git not available: %v", err)
		}
	}

	// Create initial file and commit
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("initial"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add: %v", err)
	}
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Setup mocks
	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 12345, nil
	}
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return nil
	}

	mockAuth := &mockAppAuth{}

	mockProvider := &mockProvider{
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			// Simulate provider creating a file in the repo
			newFile := filepath.Join(req.RepoPath, "generated.go")
			if err := os.WriteFile(newFile, []byte("package main\n\nfunc Generated() {}"), 0644); err != nil {
				return nil, err
			}

			return &claude.CodeResponse{
				Files: []claude.FileChange{
					{Path: "generated.go", Content: "package main\n\nfunc Generated() {}"},
				},
				Summary: "Added generated function",
				CostUSD: 0.05,
			}, nil
		},
	}

	// Mock clone function
	cleanupCalled := false
	mockClone := func(repo, branch string) (string, func(), error) {
		cleanup := func() {
			cleanupCalled = true
		}
		return tmpDir, cleanup, nil
	}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)
	executor.cloneFn = mockClone

	// Create task
	task := &webhook.Task{
		Repo:       "owner/repo",
		Number:     123,
		Branch:     "main",
		Prompt:     "Add generated function",
		IssueTitle: "Feature request",
		IssueBody:  "Please add function",
		Username:   "testuser",
	}

	// Execute
	err := executor.Execute(context.Background(), task)

	// Verify: Push will fail (no remote) but we should get to that point
	if err != nil && !strings.Contains(err.Error(), "push") {
		t.Errorf("Execute() failed before push stage: %v", err)
	}

	// Verify mock interactions
	if len(mockGH.CreateCommentCalls) != 1 {
		t.Errorf("Expected 1 CreateComment call, got %d", len(mockGH.CreateCommentCalls))
	}

	if len(mockGH.AddLabelCalls) != 1 {
		t.Errorf("Expected 1 AddLabel call, got %d", len(mockGH.AddLabelCalls))
	}

	// Cleanup should have been called
	if !cleanupCalled {
		t.Error("Clone cleanup function was not called")
	}
}

// TestExecutor_Execute_CloneFailure tests Execute() handling of clone errors
func TestExecutor_Execute_CloneFailure(t *testing.T) {
	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 12345, nil
	}
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return nil
	}

	mockAuth := &mockAppAuth{}
	mockProvider := &mockProvider{}

	// Mock clone function that fails
	mockClone := func(repo, branch string) (string, func(), error) {
		return "", nil, fmt.Errorf("clone failed: repository not found")
	}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)
	executor.cloneFn = mockClone

	task := &webhook.Task{
		Repo:     "owner/repo",
		Number:   123,
		Branch:   "main",
		Prompt:   "test",
		Username: "testuser",
	}

	err := executor.Execute(context.Background(), task)

	if err == nil {
		t.Error("Execute() should return error when clone fails")
	}

	if !strings.Contains(err.Error(), "clone") {
		t.Errorf("Error should mention clone failure, got: %v", err)
	}

	// Should update comment multiple times:
	// 1. Working status with initial progress
	// 2. Progress updates during execution
	// 3. Final error status
	// With progress tracking, there are more updates than before
	if len(mockGH.UpdateCommentCalls) < 2 {
		t.Errorf("Expected at least 2 UpdateComment calls, got %d", len(mockGH.UpdateCommentCalls))
	}
}

// TestExecutor_Execute_NoFileChanges tests Execute() when provider returns response but no code changes
func TestExecutor_Execute_NoFileChanges_WithMockClone(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "user.email", "test@test.com"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Skipf("Git not available: %v", err)
		}
	}

	// Create initial file and commit
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("initial"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = tmpDir
	cmd.Run()

	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 12345, nil
	}
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return nil
	}

	mockAuth := &mockAppAuth{}

	// Provider returns response but doesn't modify any files
	mockProvider := &mockProvider{
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			return &claude.CodeResponse{
				Files:   []claude.FileChange{}, // No files
				Summary: "Analysis: The code looks good, no changes needed",
				CostUSD: 0.01,
			}, nil
		},
	}

	mockClone := func(repo, branch string) (string, func(), error) {
		return tmpDir, func() {}, nil
	}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)
	executor.cloneFn = mockClone

	task := &webhook.Task{
		Repo:       "owner/repo",
		Number:     123,
		Branch:     "main",
		Prompt:     "Review the code",
		IssueTitle: "Code review",
		IssueBody:  "Please review",
		Username:   "testuser",
	}

	err := executor.Execute(context.Background(), task)

	if err != nil {
		t.Errorf("Execute() should succeed for response-only (no code changes): %v", err)
	}

	// Should have updated comment for status transitions, progress updates, and completion
	// With progress tracking, there are more updates than before
	if len(mockGH.UpdateCommentCalls) < 2 {
		t.Errorf("Expected at least 2 UpdateComment calls, got %d", len(mockGH.UpdateCommentCalls))
	}

	// Verify the final update call contains the summary
	if len(mockGH.UpdateCommentCalls) > 0 {
		call := mockGH.UpdateCommentCalls[len(mockGH.UpdateCommentCalls)-1]
		if !strings.Contains(call.Body, "Analysis") {
			t.Error("Update comment should contain the analysis summary")
		}
	}
}

// TestExecutor_Execute_DetectChangesError tests Execute() handling of detectGitChanges errors
func TestExecutor_Execute_DetectChangesError_WithMockClone(t *testing.T) {
	// Prepare temporary git repo so branch creation succeeds
	tmpDir := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "user.email", "test@test.com"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Skipf("Git not available: %v", err)
		}
	}

	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 12345, nil
	}
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return nil
	}

	mockAuth := &mockAppAuth{}

	tt := t

	mockProvider := &mockProvider{
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			// Remove git metadata to force detectGitChanges failure later
			if err := os.RemoveAll(filepath.Join(req.RepoPath, ".git")); err != nil {
				tt.Fatalf("failed to remove git directory: %v", err)
			}
			return &claude.CodeResponse{
				Files:   []claude.FileChange{{Path: "test.go", Content: "package test"}},
				Summary: "Added file",
				CostUSD: 0.01,
			}, nil
		},
	}

	mockClone := func(repo, branch string) (string, func(), error) {
		return tmpDir, func() {}, nil
	}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)
	executor.cloneFn = mockClone

	task := &webhook.Task{
		Repo:     "owner/repo",
		Number:   123,
		Branch:   "main",
		Prompt:   "Add file",
		Username: "testuser",
	}

	err := executor.Execute(context.Background(), task)

	if err == nil {
		t.Error("Execute() should return error when git status fails")
	}

	if !strings.Contains(err.Error(), "detect changes") {
		t.Errorf("Error should mention detect changes failure, got: %v", err)
	}
}

// TestExecutor_Execute_ProviderGenerateError tests Execute() handling of provider errors
func TestExecutor_Execute_ProviderGenerateError_WithMockClone(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "user.email", "test@test.com"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Skipf("Git not available: %v", err)
		}
	}

	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 12345, nil
	}
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return nil
	}

	mockAuth := &mockAppAuth{}

	// Provider that fails
	mockProvider := &mockProvider{
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			return nil, fmt.Errorf("API rate limit exceeded")
		},
	}

	mockClone := func(repo, branch string) (string, func(), error) {
		return tmpDir, func() {}, nil
	}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)
	executor.cloneFn = mockClone

	task := &webhook.Task{
		Repo:     "owner/repo",
		Number:   123,
		Branch:   "main",
		Prompt:   "Add feature",
		Username: "testuser",
	}

	err := executor.Execute(context.Background(), task)

	if err == nil {
		t.Error("Execute() should return error when provider fails")
	}

	if !strings.Contains(err.Error(), "API rate limit") && !strings.Contains(err.Error(), "mock error") {
		t.Errorf("Error should mention provider failure, got: %v", err)
	}

	// Should update for status transitions, progress, and error
	// With progress tracking, there are more updates than before
	if len(mockGH.UpdateCommentCalls) < 2 {
		t.Errorf("Expected at least 2 UpdateComment calls, got %d", len(mockGH.UpdateCommentCalls))
	}
}

func TestExecutor_WithStore(t *testing.T) {
	executor := New(&mockProvider{}, nil)
	store := taskstore.NewStore()

	if executor.WithStore(store) != executor {
		t.Fatal("WithStore should return the executor instance for chaining")
	}
	if executor.store != store {
		t.Fatal("WithStore should assign the provided store")
	}
}

func TestExecutor_UpdateStatusAndAddLog(t *testing.T) {
	store := taskstore.NewStore()
	executor := New(&mockProvider{}, nil).WithStore(store)

	store.Create(&taskstore.Task{ID: "task-1"})
	task := &webhook.Task{ID: "task-1"}

	executor.updateStatus(task, taskstore.StatusRunning)
	stored, ok := store.Get("task-1")
	if !ok {
		t.Fatal("Expected task to exist in store")
	}
	if stored.Status != taskstore.StatusRunning {
		t.Fatalf("Status = %s, want %s", stored.Status, taskstore.StatusRunning)
	}

	executor.addLog(task, "info", "hello %s", "world")
	if len(stored.Logs) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(stored.Logs))
	}
	if stored.Logs[0].Message != "hello world" {
		t.Fatalf("Log message = %q, want %q", stored.Logs[0].Message, "hello world")
	}
}

func TestExecutor_UpdateStatus_NoStoreOrTask(t *testing.T) {
	executor := New(&mockProvider{}, nil)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("updateStatus should not panic without store, panic: %v", r)
		}
	}()
	executor.updateStatus(nil, taskstore.StatusCompleted)
}

func TestExecutor_ComposeDiscussionSection(t *testing.T) {
	mockGH := github.NewMockGHClient()
	mockGH.ListIssueCommentsFunc = func(repo string, number int, token string) ([]github.IssueComment, error) {
		return []github.IssueComment{
			{
				Author:    "alice",
				Body:      "Looks good",
				CreatedAt: time.Date(2025, 10, 10, 10, 0, 0, 0, time.UTC),
			},
		}, nil
	}
	mockGH.ListReviewCommentsFunc = func(repo string, number int, token string) ([]github.ReviewComment, error) {
		return []github.ReviewComment{
			{
				Author:    "bob",
				Body:      "Need a fix",
				Path:      "main.go",
				DiffHunk:  "@@ -1 +1 @@",
				CreatedAt: time.Date(2025, 10, 10, 11, 0, 0, 0, time.UTC),
			},
		}, nil
	}

	executor := NewWithClient(&mockProvider{}, nil, mockGH)

	task := &webhook.Task{Repo: "owner/repo", Number: 1, IsPR: true}
	result := executor.composeDiscussionSection(task, "token")

	if !strings.Contains(result, "## Discussion") {
		t.Fatalf("composeDiscussionSection output missing header:\n%s", result)
	}
	if !strings.Contains(result, "@alice (2025-10-10T10:00:00Z):") {
		t.Fatalf("Issue comment not rendered: %s", result)
	}
	if !strings.Contains(result, "_File: main.go_") {
		t.Fatalf("Review metadata missing: %s", result)
	}
	if !strings.Contains(result, "```diff") {
		t.Fatalf("Diff hunk missing fenced code block:\n%s", result)
	}
}

func TestFormatDiscussion_SortingAndDefaults(t *testing.T) {
	issueComments := []github.IssueComment{
		{
			Author:    "",
			Body:      "First",
			CreatedAt: time.Date(2025, 10, 9, 10, 0, 0, 0, time.UTC),
		},
	}
	reviewComments := []github.ReviewComment{
		{
			Author:    "carol",
			Body:      "Second",
			Path:      "core.go",
			CreatedAt: time.Time{}, // zero timestamp to check fallback ordering
		},
	}

	output := formatDiscussion(issueComments, reviewComments)

	if !strings.Contains(output, "@unknown (2025-10-09T10:00:00Z):") {
		t.Fatalf("Missing normalized author or timestamp:\n%s", output)
	}
	if !strings.Contains(output, "@carol (unknown):") {
		t.Fatalf("Missing fallback timestamp:\n%s", output)
	}

	firstIdx := strings.Index(output, "First")
	secondIdx := strings.Index(output, "Second")
	if firstIdx == -1 || secondIdx == -1 || secondIdx > firstIdx {
		t.Fatalf("Entries not ordered as expected:\n%s", output)
	}
}

func TestInjectDiscussion(t *testing.T) {
	base := "Intro\n\n---\n\nBody"
	discussion := "## Discussion\n@alice"
	out := injectDiscussion(base, discussion)

	if !strings.Contains(out, discussion) {
		t.Fatalf("Discussion block missing in output:\n%s", out)
	}
	if strings.Count(out, "---") != 1 {
		t.Fatalf("Separator count incorrect:\n%s", out)
	}

	noSeparator := "Intro only"
	out = injectDiscussion(noSeparator, discussion)
	if !strings.Contains(out, "Intro only") || !strings.Contains(out, discussion) {
		t.Fatalf("Expected discussion appended when separator missing:\n%s", out)
	}

	if injectDiscussion("   ", "   ") != "   " {
		t.Fatalf("Expected original prompt when discussion empty")
	}
}

// TestExecutor_Execute_ApplyChangesError tests Execute() handling of applyChanges errors
func TestExecutor_Execute_ApplyChangesError_WithMockClone(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "user.email", "test@test.com"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Skipf("Git not available: %v", err)
		}
	}

	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 12345, nil
	}
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return nil
	}

	mockAuth := &mockAppAuth{}

	// Provider returns file with invalid path (contains null byte)
	mockProvider := &mockProvider{
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			return &claude.CodeResponse{
				Files: []claude.FileChange{
					{Path: "test\x00.go", Content: "package test"},
				},
				Summary: "Added file",
				CostUSD: 0.01,
			}, nil
		},
	}

	mockClone := func(repo, branch string) (string, func(), error) {
		return tmpDir, func() {}, nil
	}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)
	executor.cloneFn = mockClone

	task := &webhook.Task{
		Repo:     "owner/repo",
		Number:   123,
		Branch:   "main",
		Prompt:   "Add file",
		Username: "testuser",
	}

	err := executor.Execute(context.Background(), task)

	if err == nil {
		t.Error("Execute() should return error when applyChanges fails")
	}

	if !strings.Contains(err.Error(), "apply changes") {
		t.Errorf("Error should mention apply changes failure, got: %v", err)
	}
}

// TestExecutor_Execute_UpdateCommentWarning tests Execute() when comment update fails (non-fatal)
func TestExecutor_Execute_UpdateCommentWarning_WithMockClone(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "user.email", "test@test.com"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Skipf("Git not available: %v", err)
		}
	}

	// Create initial commit
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("initial"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = tmpDir
	cmd.Run()

	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 12345, nil
	}
	// UpdateComment fails but this should not stop execution
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return fmt.Errorf("network error")
	}

	mockAuth := &mockAppAuth{}

	// Provider returns response with no files
	mockProvider := &mockProvider{
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			return &claude.CodeResponse{
				Files:   []claude.FileChange{},
				Summary: "Code is good",
				CostUSD: 0.01,
			}, nil
		},
	}

	mockClone := func(repo, branch string) (string, func(), error) {
		return tmpDir, func() {}, nil
	}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)
	executor.cloneFn = mockClone

	task := &webhook.Task{
		Repo:       "owner/repo",
		Number:     123,
		Branch:     "main",
		Prompt:     "Review code",
		IssueTitle: "Code review",
		IssueBody:  "Please review",
		Username:   "testuser",
	}

	// Execute should succeed even though UpdateComment failed
	err := executor.Execute(context.Background(), task)

	if err != nil {
		t.Errorf("Execute() should succeed even when UpdateComment fails: %v", err)
	}

	// UpdateComment should have been called multiple times (status + progress + final response)
	// With progress tracking, there are more updates than before
	if len(mockGH.UpdateCommentCalls) < 2 {
		t.Errorf("Expected at least 2 UpdateComment calls, got %d", len(mockGH.UpdateCommentCalls))
	}
}

// TestExecutor_Execute_CreateCommentWarning tests Execute() when comment creation fails (non-fatal)
func TestExecutor_Execute_CreateCommentWarning_WithMockClone(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "user.email", "test@test.com"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Skipf("Git not available: %v", err)
		}
	}

	// Create initial commit
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("initial"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = tmpDir
	cmd.Run()

	mockGH := github.NewMockGHClient()
	// CreateComment fails
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 0, fmt.Errorf("permission denied")
	}

	mockAuth := &mockAppAuth{}

	mockProvider := &mockProvider{
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			return &claude.CodeResponse{
				Files:   []claude.FileChange{},
				Summary: "Analysis complete",
				CostUSD: 0.01,
			}, nil
		},
	}

	mockClone := func(repo, branch string) (string, func(), error) {
		return tmpDir, func() {}, nil
	}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)
	executor.cloneFn = mockClone

	task := &webhook.Task{
		Repo:     "owner/repo",
		Number:   123,
		Branch:   "main",
		Prompt:   "Analyze",
		Username: "testuser",
	}

	// Execute should succeed even though CreateComment failed
	err := executor.Execute(context.Background(), task)

	if err != nil {
		t.Errorf("Execute() should succeed even when CreateComment fails: %v", err)
	}

	// CreateComment should have been called
	if len(mockGH.CreateCommentCalls) != 1 {
		t.Errorf("Expected 1 CreateComment call, got %d", len(mockGH.CreateCommentCalls))
	}
}

// TestExecutor_Execute_AddLabelWarning tests Execute() when AddLabel fails (non-fatal)
func TestExecutor_Execute_AddLabelWarning_WithMockClone(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "user.email", "test@test.com"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Skipf("Git not available: %v", err)
		}
	}

	// Create initial commit
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("initial"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = tmpDir
	cmd.Run()

	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 12345, nil
	}
	// AddLabel fails
	mockGH.AddLabelFunc = func(repo string, number int, label, token string) error {
		return fmt.Errorf("label does not exist")
	}

	mockAuth := &mockAppAuth{}

	mockProvider := &mockProvider{
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			return &claude.CodeResponse{
				Files:   []claude.FileChange{},
				Summary: "Done",
				CostUSD: 0.01,
			}, nil
		},
	}

	mockClone := func(repo, branch string) (string, func(), error) {
		return tmpDir, func() {}, nil
	}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)
	executor.cloneFn = mockClone

	task := &webhook.Task{
		Repo:     "owner/repo",
		Number:   123,
		Branch:   "main",
		Prompt:   "Task",
		Username: "testuser",
	}

	// Execute should succeed even though AddLabel failed
	err := executor.Execute(context.Background(), task)

	if err != nil {
		t.Errorf("Execute() should succeed even when AddLabel fails: %v", err)
	}

	// AddLabel should have been called
	if len(mockGH.AddLabelCalls) != 1 {
		t.Errorf("Expected 1 AddLabel call, got %d", len(mockGH.AddLabelCalls))
	}
}

func TestFormatDiscussion(t *testing.T) {
	issueComments := []github.IssueComment{
		{
			Author:    "alice",
			Body:      "Initial analysis",
			CreatedAt: time.Date(2025, 10, 10, 10, 0, 0, 0, time.UTC),
		},
	}
	reviewComments := []github.ReviewComment{
		{
			Author:    "bob",
			Body:      "Looks good overall",
			Path:      "main.go",
			CreatedAt: time.Date(2025, 10, 10, 11, 0, 0, 0, time.UTC),
		},
	}

	section := formatDiscussion(issueComments, reviewComments)

	if !strings.Contains(section, "## Discussion") {
		t.Fatalf("Discussion section missing header: %q", section)
	}
	if !strings.Contains(section, "@alice") || !strings.Contains(section, "@bob") {
		t.Fatalf("Discussion section missing authors: %q", section)
	}
	if !strings.Contains(section, "_File: main.go_") {
		t.Fatalf("Discussion section missing review metadata: %q", section)
	}

	idxAlice := strings.Index(section, "@alice")
	idxBob := strings.Index(section, "@bob")
	if idxAlice == -1 || idxBob == -1 || idxAlice > idxBob {
		t.Fatalf("Discussion entries not ordered chronologically: %q", section)
	}
}

func TestFormatDiscussion_WithDiffHunk(t *testing.T) {
	reviewComments := []github.ReviewComment{
		{
			Author:    "reviewer",
			Body:      "This looks wrong",
			Path:      "internal/app/handler.go",
			DiffHunk:  "@@ -10,7 +10,7 @@\n func Process() error {\n-\treturn nil\n+\treturn fmt.Errorf(\"not implemented\")\n }",
			CreatedAt: time.Date(2025, 10, 10, 12, 0, 0, 0, time.UTC),
		},
	}

	section := formatDiscussion(nil, reviewComments)

	// Should contain file path
	if !strings.Contains(section, "_File: internal/app/handler.go_") {
		t.Fatalf("Discussion section missing file path: %q", section)
	}

	// Should contain diff hunk as code block
	if !strings.Contains(section, "```diff") {
		t.Fatalf("Discussion section missing diff code block: %q", section)
	}
	if !strings.Contains(section, "return fmt.Errorf") {
		t.Fatalf("Discussion section missing diff content: %q", section)
	}
	if !strings.Contains(section, "```\nThis looks wrong") {
		t.Fatalf("Diff code block not properly closed before comment body: %q", section)
	}
}

func TestInjectDiscussionWithSeparator(t *testing.T) {
	base := "Fix it quickly\n\n---\n\n# Issue Context\n\n## Title\nBug\n\n## Body\nSteps to reproduce"
	discussion := "## Discussion\n\n@alice (2025-10-12T01:00:00Z):\nInvestigating now"

	result := injectDiscussion(base, discussion)

	if strings.Count(result, "## Discussion") != 1 {
		t.Fatalf("Expected single discussion section, got: %q", result)
	}
	instructionIndex := strings.Index(result, "Fix it quickly")
	discIndex := strings.Index(result, "## Discussion")
	sepIndex := strings.Index(result, "\n\n---\n\n")
	if discIndex == -1 || sepIndex == -1 || instructionIndex == -1 {
		t.Fatalf("Missing expected sections: %q", result)
	}
	if !(instructionIndex < discIndex && discIndex < sepIndex) {
		t.Fatalf("Discussion should appear between instruction and context: %q", result)
	}
}

func TestComposeDiscussionSection(t *testing.T) {
	mockGH := github.NewMockGHClient()
	mockGH.ListIssueCommentsFunc = func(repo string, number int, token string) ([]github.IssueComment, error) {
		return []github.IssueComment{
			{
				Author:    "alice",
				Body:      "First comment",
				CreatedAt: time.Date(2025, 10, 9, 8, 0, 0, 0, time.UTC),
			},
		}, nil
	}

	exec := &Executor{ghClient: mockGH}
	task := &webhook.Task{
		Repo:   "owner/repo",
		Number: 1,
	}

	section := exec.composeDiscussionSection(task, "token")
	if section == "" {
		t.Fatal("composeDiscussionSection() returned empty string")
	}
	if len(mockGH.ListIssueCommentsCalls) != 1 {
		t.Fatalf("Expected ListIssueComments to be called once, got %d", len(mockGH.ListIssueCommentsCalls))
	}
	if len(mockGH.ListReviewCommentsCalls) != 0 {
		t.Fatalf("Review comments should not be fetched for issues, got %d calls", len(mockGH.ListReviewCommentsCalls))
	}
}

func TestComposeDiscussionSection_PRIncludesReviews(t *testing.T) {
	mockGH := github.NewMockGHClient()
	mockGH.ListIssueCommentsFunc = func(repo string, number int, token string) ([]github.IssueComment, error) {
		return []github.IssueComment{
			{
				Author:    "alice",
				Body:      "General comment",
				CreatedAt: time.Date(2025, 10, 9, 8, 0, 0, 0, time.UTC),
			},
		}, nil
	}
	mockGH.ListReviewCommentsFunc = func(repo string, number int, token string) ([]github.ReviewComment, error) {
		return []github.ReviewComment{
			{
				Author:    "bob",
				Body:      "Inline suggestion",
				Path:      "main.go",
				CreatedAt: time.Date(2025, 10, 9, 9, 0, 0, 0, time.UTC),
			},
		}, nil
	}

	exec := &Executor{ghClient: mockGH}
	task := &webhook.Task{
		Repo:   "owner/repo",
		Number: 42,
		IsPR:   true,
	}

	section := exec.composeDiscussionSection(task, "token")
	if !strings.Contains(section, "Inline suggestion") {
		t.Fatalf("Review comment missing from discussion: %q", section)
	}
	if len(mockGH.ListReviewCommentsCalls) != 1 {
		t.Fatalf("Expected ListReviewComments to be called once, got %d", len(mockGH.ListReviewCommentsCalls))
	}
}

func TestExecutor_Execute_IncludesDiscussionContext(t *testing.T) {
	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 12345, nil
	}
	mockGH.ListIssueCommentsFunc = func(repo string, number int, token string) ([]github.IssueComment, error) {
		return []github.IssueComment{
			{
				Author:    "alice",
				Body:      "Please handle edge cases",
				CreatedAt: time.Date(2025, 10, 10, 10, 0, 0, 0, time.UTC),
			},
		}, nil
	}
	mockGH.ListReviewCommentsFunc = func(repo string, number int, token string) ([]github.ReviewComment, error) {
		return []github.ReviewComment{
			{
				Author:    "bob",
				Body:      "Nit: rename variable",
				Path:      "main.go",
				CreatedAt: time.Date(2025, 10, 10, 11, 0, 0, 0, time.UTC),
			},
		}, nil
	}

	mockAuth := &mockAppAuth{}

	var capturedPrompt string
	mockProvider := &mockProvider{
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			capturedPrompt = req.Prompt
			return nil, fmt.Errorf("stop after prompt capture")
		},
	}

	mockClone := func(repo, branch string) (string, func(), error) {
		dir := t.TempDir()
		cmds := [][]string{
			{"git", "init"},
			{"git", "config", "user.name", "Test"},
			{"git", "config", "user.email", "test@test.com"},
		}
		for _, args := range cmds {
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Dir = dir
			if err := cmd.Run(); err != nil {
				return "", nil, err
			}
		}
		if branch != "" {
			cmd := exec.Command("git", "checkout", "-b", branch)
			cmd.Dir = dir
			if err := cmd.Run(); err != nil {
				return "", nil, err
			}
		}
		return dir, func() {}, nil
	}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)
	executor.cloneFn = mockClone

	task := &webhook.Task{
		Repo:       "owner/repo",
		Number:     777,
		Branch:     "main",
		Prompt:     "Implement fix ASAP\n\n---\n\n# Issue Context\n\n## Title\nLogin bug\n\n## Body\nFix login crash",
		IssueTitle: "Login bug",
		IssueBody:  "Fix login crash",
		IsPR:       true,
		Username:   "tester",
	}

	err := executor.Execute(context.Background(), task)
	if err == nil || !strings.Contains(err.Error(), "stop after prompt capture") {
		t.Fatalf("Execute() should return provider error, got %v", err)
	}

	if !strings.Contains(capturedPrompt, "## Discussion") {
		t.Fatalf("Provider prompt missing discussion section: %q", capturedPrompt)
	}
	if !strings.Contains(capturedPrompt, "Please handle edge cases") {
		t.Fatalf("Issue comment not included in prompt: %q", capturedPrompt)
	}
	if !strings.Contains(capturedPrompt, "Nit: rename variable") {
		t.Fatalf("Review comment not included in prompt: %q", capturedPrompt)
	}
	if strings.Index(capturedPrompt, "## Discussion") < strings.Index(capturedPrompt, "Implement fix ASAP") {
		t.Fatalf("Discussion section should follow user instructions: %q", capturedPrompt)
	}
	if strings.Count(capturedPrompt, "\n\n---\n\n") != 1 {
		t.Fatalf("Instruction separator should remain single occurrence: %q", capturedPrompt)
	}
	if len(mockGH.ListIssueCommentsCalls) != 1 {
		t.Fatalf("Expected ListIssueComments to be called once, got %d", len(mockGH.ListIssueCommentsCalls))
	}
	if len(mockGH.ListReviewCommentsCalls) != 1 {
		t.Fatalf("Expected ListReviewComments to be called once, got %d", len(mockGH.ListReviewCommentsCalls))
	}
}

// TestDetectManualChanges tests the new manual change detection functionality
func TestDetectManualChanges(t *testing.T) {
	executor := New(&mockProvider{}, nil)
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		setup       func(string) []claude.FileChange
		wantChanges int
		wantPaths   []string
	}{
		{
			name: "detect new file",
			setup: func(dir string) []claude.FileChange {
				return []claude.FileChange{
					{Path: "new.go", Content: "package main"},
				}
			},
			wantChanges: 1,
			wantPaths:   []string{"new.go"},
		},
		{
			name: "detect modified file",
			setup: func(dir string) []claude.FileChange {
				// Create existing file
				existingPath := filepath.Join(dir, "existing.go")
				os.WriteFile(existingPath, []byte("old content"), 0644)

				return []claude.FileChange{
					{Path: "existing.go", Content: "new content"},
				}
			},
			wantChanges: 1,
			wantPaths:   []string{"existing.go"},
		},
		{
			name: "ignore unchanged file",
			setup: func(dir string) []claude.FileChange {
				// Create existing file with same content
				existingPath := filepath.Join(dir, "unchanged.go")
				content := "same content"
				os.WriteFile(existingPath, []byte(content), 0644)

				return []claude.FileChange{
					{Path: "unchanged.go", Content: content},
				}
			},
			wantChanges: 0,
			wantPaths:   []string{},
		},
		{
			name: "mix of changed and unchanged files",
			setup: func(dir string) []claude.FileChange {
				// Create one existing file
				existingPath := filepath.Join(dir, "existing.go")
				os.WriteFile(existingPath, []byte("old content"), 0644)

				// Create another existing file with same content as expected
				unchangedPath := filepath.Join(dir, "unchanged.go")
				sameContent := "same content"
				os.WriteFile(unchangedPath, []byte(sameContent), 0644)

				return []claude.FileChange{
					{Path: "existing.go", Content: "new content"}, // Changed
					{Path: "unchanged.go", Content: sameContent},  // Unchanged
					{Path: "new.go", Content: "package main"},     // New
				}
			},
			wantChanges: 2,
			wantPaths:   []string{"existing.go", "new.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh test directory
			testDir := filepath.Join(tmpDir, tt.name)
			os.MkdirAll(testDir, 0755)

			parsedFiles := tt.setup(testDir)
			changes := executor.detectManualChanges(testDir, parsedFiles)

			if len(changes) != tt.wantChanges {
				t.Errorf("detectManualChanges() returned %d changes, want %d", len(changes), tt.wantChanges)
			}

			// Check that we got the expected file paths
			gotPaths := make([]string, len(changes))
			for i, change := range changes {
				gotPaths[i] = change.Path
			}

			for _, expectedPath := range tt.wantPaths {
				found := false
				for _, gotPath := range gotPaths {
					if gotPath == expectedPath {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected path %s not found in results: %v", expectedPath, gotPaths)
				}
			}
		})
	}
}

// TestDetectGitChangesWithDebug tests enhanced git detection with debug logging
func TestDetectGitChangesWithDebug(t *testing.T) {
	// Enable debug logging
	oldValue := os.Getenv("DEBUG_GIT_DETECTION")
	os.Setenv("DEBUG_GIT_DETECTION", "true")
	defer func() {
		if oldValue == "" {
			os.Unsetenv("DEBUG_GIT_DETECTION")
		} else {
			os.Setenv("DEBUG_GIT_DETECTION", oldValue)
		}
	}()

	executor := New(&mockProvider{}, nil)

	tests := []struct {
		name       string
		setup      func(string) error
		wantChange bool
	}{
		{
			name: "detect untracked file",
			setup: func(dir string) error {
				// Initialize git repo
				if err := exec.Command("git", "init").Run(); err != nil {
					return err
				}
				if err := exec.Command("git", "config", "user.name", "Test").Run(); err != nil {
					return err
				}
				if err := exec.Command("git", "config", "user.email", "test@test.com").Run(); err != nil {
					return err
				}

				// Create untracked file
				return os.WriteFile(filepath.Join(dir, "untracked.go"), []byte("package main"), 0644)
			},
			wantChange: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			originalDir, _ := os.Getwd()
			os.Chdir(tmpDir)
			defer os.Chdir(originalDir)

			if err := tt.setup(tmpDir); err != nil {
				t.Skipf("Setup failed: %v", err)
			}

			hasChanges, err := executor.detectGitChanges(tmpDir)
			if err != nil {
				t.Errorf("detectGitChanges() error = %v", err)
			}

			if hasChanges != tt.wantChange {
				t.Errorf("detectGitChanges() = %v, want %v", hasChanges, tt.wantChange)
			}
		})
	}
}

// TestApplyChangesEnhanced tests the enhanced applyChanges with validation
func TestApplyChangesEnhanced(t *testing.T) {
	executor := New(&mockProvider{}, nil)
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		changes   []claude.FileChange
		wantError bool
		validate  func(string) error
	}{
		{
			name: "successful file creation with validation",
			changes: []claude.FileChange{
				{Path: "test.go", Content: "package test\n\nfunc Test() {}\n"},
			},
			wantError: false,
			validate: func(dir string) error {
				content, err := os.ReadFile(filepath.Join(dir, "test.go"))
				if err != nil {
					return err
				}
				expected := "package test\n\nfunc Test() {}\n"
				if string(content) != expected {
					return fmt.Errorf("content mismatch: got %q, want %q", string(content), expected)
				}
				return nil
			},
		},
		{
			name: "handle empty file path gracefully",
			changes: []claude.FileChange{
				{Path: "", Content: "some content"},
				{Path: "valid.go", Content: "package main"},
			},
			wantError: false, // Should skip empty path, continue with valid one
			validate: func(dir string) error {
				// Should only create the valid file
				if _, err := os.Stat(filepath.Join(dir, "valid.go")); err != nil {
					return fmt.Errorf("valid.go should exist: %v", err)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := filepath.Join(tmpDir, tt.name)
			os.MkdirAll(testDir, 0755)

			err := executor.applyChanges(testDir, tt.changes)

			if (err != nil) != tt.wantError {
				t.Errorf("applyChanges() error = %v, wantError %v", err, tt.wantError)
			}

			if tt.validate != nil {
				if err := tt.validate(testDir); err != nil {
					t.Errorf("Validation failed: %v", err)
				}
			}
		})
	}
}

func TestFormatCommitMessage(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		name    string
		summary string
		task    *webhook.Task
		want    []string // Expected parts in the commit message
		notWant []string // Parts that should NOT be in the commit message
	}{
		{
			name:    "basic summary only",
			summary: "Fix bug in parser",
			task: &webhook.Task{
				Number:   0,
				IsPR:     false,
				Username: "",
			},
			want: []string{
				"Fix bug in parser",
				"Generated with [SWE Agent](https://github.com/cexll/swe-agent)",
			},
			notWant: []string{
				"Fixes #",
				"Co-authored-by:",
			},
		},
		{
			name:    "with issue number",
			summary: "Add new feature",
			task: &webhook.Task{
				Number:   123,
				IsPR:     false,
				Username: "",
			},
			want: []string{
				"Add new feature",
				"Fixes #123",
				"Generated with [SWE Agent](https://github.com/cexll/swe-agent)",
			},
		},
		{
			name:    "with PR number (should not include Fixes)",
			summary: "Update documentation",
			task: &webhook.Task{
				Number:   456,
				IsPR:     true,
				Username: "",
			},
			want: []string{
				"Update documentation",
				"Generated with [SWE Agent](https://github.com/cexll/swe-agent)",
			},
			notWant: []string{
				"Fixes #456",
			},
		},
		{
			name:    "with username",
			summary: "Refactor code",
			task: &webhook.Task{
				Number:   789,
				IsPR:     false,
				Username: "testuser",
			},
			want: []string{
				"Refactor code",
				"Fixes #789",
				"Generated with [SWE Agent](https://github.com/cexll/swe-agent)",
				"Co-authored-by: testuser <testuser@users.noreply.github.com>",
			},
		},
		{
			name:    "with Unknown username (should not include Co-authored-by)",
			summary: "Fix typo",
			task: &webhook.Task{
				Number:   101,
				IsPR:     false,
				Username: "Unknown",
			},
			want: []string{
				"Fix typo",
				"Fixes #101",
				"Generated with [SWE Agent](https://github.com/cexll/swe-agent)",
			},
			notWant: []string{
				"Co-authored-by: Unknown",
			},
		},
		{
			name:    "full example with all fields",
			summary: "Implement authentication\n\nThis adds JWT-based authentication to the API.",
			task: &webhook.Task{
				Number:   999,
				IsPR:     false,
				Username: "developer",
			},
			want: []string{
				"Implement authentication",
				"This adds JWT-based authentication to the API.",
				"Fixes #999",
				"Generated with [SWE Agent](https://github.com/cexll/swe-agent)",
				"Co-authored-by: developer <developer@users.noreply.github.com>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.formatCommitMessage(tt.summary, tt.task)

			// Check that all wanted parts are present
			for _, want := range tt.want {
				if !strings.Contains(result, want) {
					t.Errorf("formatCommitMessage() missing expected part:\nwant: %q\ngot: %q", want, result)
				}
			}

			// Check that unwanted parts are NOT present
			for _, notWant := range tt.notWant {
				if strings.Contains(result, notWant) {
					t.Errorf("formatCommitMessage() contains unexpected part:\ndon't want: %q\ngot: %q", notWant, result)
				}
			}

			// Verify the message has proper double-newline separation
			if tt.task.Number > 0 || tt.task.Username != "" && tt.task.Username != "Unknown" {
				if !strings.Contains(result, "\n\n") {
					t.Errorf("formatCommitMessage() should have double-newline separation, got: %q", result)
				}
			}
		})
	}
}

func TestPrepareBranchCreatesNewBranchForIssue(t *testing.T) {
	tmpDir := t.TempDir()

	runGit(t, tmpDir, "git", "init")
	runGit(t, tmpDir, "git", "config", "user.name", "Tester")
	runGit(t, tmpDir, "git", "config", "user.email", "tester@example.com")

	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("initial\n"), 0o644); err != nil {
		t.Fatalf("failed to write seed file: %v", err)
	}
	runGit(t, tmpDir, "git", "add", ".")
	runGit(t, tmpDir, "git", "commit", "-m", "seed")
	runGit(t, tmpDir, "git", "branch", "-M", "main")

	executor := &Executor{}
	task := &webhook.Task{
		Number: 987,
		IsPR:   false,
	}

	branchName, isNewBranch, err := executor.prepareBranch(tmpDir, task)
	if err != nil {
		t.Fatalf("prepareBranch returned error: %v", err)
	}
	if !isNewBranch {
		t.Fatal("prepareBranch should mark new branch for issues")
	}
	expectedPrefix := fmt.Sprintf("swe/issue-%d-", task.Number)
	if !strings.HasPrefix(branchName, expectedPrefix) {
		t.Fatalf("branch name %q should start with %q", branchName, expectedPrefix)
	}

	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --show-current failed: %v\n%s", err, out)
	}
	current := strings.TrimSpace(string(out))
	if current != branchName {
		t.Fatalf("expected HEAD on %q, got %q", branchName, current)
	}
}
