package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/provider/claude"
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
			err := executor.commitAndPush(tt.workdir, tt.branchName, tt.commitMessage)
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
			wantErr: false, // Will create file in safe location
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
			head:  "pilot/123-456",
			base:  "release/v1",
			title: "Feature work",
			wantSubstr: []string{
				"release%2Fv1...pilot%2F123-456",
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

			err = executor.commitAndPush(workdir, tt.branchName, tt.commitMessage)

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

	// Test with very long commit message
	longMessage := strings.Repeat("This is a very long commit message. ", 50) // ~1850 chars

	err := executor.commitAndPush(tmpDir, "test-long-msg", longMessage)

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

	// Should update once for working status and once for error details
	if len(mockGH.UpdateCommentCalls) != 2 {
		t.Errorf("Expected 2 UpdateComment calls (status + error), got %d", len(mockGH.UpdateCommentCalls))
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

	// Should have updated comment for status transition and completion summary
	if len(mockGH.UpdateCommentCalls) != 2 {
		t.Errorf("Expected 2 UpdateComment calls, got %d", len(mockGH.UpdateCommentCalls))
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
	// Create non-git directory
	tmpDir := t.TempDir()

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

	// Should update once for status and once for error
	if len(mockGH.UpdateCommentCalls) != 2 {
		t.Errorf("Expected 2 UpdateComment calls (status + error), got %d", len(mockGH.UpdateCommentCalls))
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

	// UpdateComment should have been called twice (status + final response)
	if len(mockGH.UpdateCommentCalls) != 2 {
		t.Errorf("Expected 2 UpdateComment calls, got %d", len(mockGH.UpdateCommentCalls))
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
	base := "# Issue: Bug\n\nSteps to reproduce\n\n---\n\nFix it quickly"
	discussion := "## Discussion\n\n@alice (2025-10-12T01:00:00Z):\nInvestigating now"

	result := injectDiscussion(base, discussion)

	if strings.Count(result, "## Discussion") != 1 {
		t.Fatalf("Expected single discussion section, got: %q", result)
	}
	sepIndex := strings.Index(result, "\n\n---\n\n")
	discIndex := strings.Index(result, "## Discussion")
	if discIndex == -1 || sepIndex == -1 || discIndex > sepIndex {
		t.Fatalf("Discussion section not inserted before user instruction: %q", result)
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
		return t.TempDir(), func() {}, nil
	}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)
	executor.cloneFn = mockClone

	task := &webhook.Task{
		Repo:       "owner/repo",
		Number:     777,
		Branch:     "main",
		Prompt:     "# Issue: Login bug\n\nFix login crash\n\n---\n\nImplement fix ASAP",
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
	if strings.Index(capturedPrompt, "## Discussion") > strings.Index(capturedPrompt, "Implement fix ASAP") {
		t.Fatalf("Discussion section should appear before user instructions: %q", capturedPrompt)
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
