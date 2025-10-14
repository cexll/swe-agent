package executor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/provider/claude"
	"github.com/cexll/swe/internal/webhook"
)

// TestExecutor_WithDisallowedTools tests the disallowed tools functionality
func TestExecutor_WithDisallowedTools(t *testing.T) {
	mockGH := github.NewMockGHClient()
	mockAuth := &mockAppAuth{}
	mockProvider := &mockProvider{
		name: "test-provider",
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			// Verify disallowed_tools is passed in context
			if tools, ok := req.Context["disallowed_tools"]; ok {
				if tools != "Bash(rm:*),Bash(sudo:*)" {
					t.Errorf("Expected disallowed_tools to be passed, got: %s", tools)
				}
			} else {
				t.Error("disallowed_tools not found in context")
			}
			return &claude.CodeResponse{
				Files:   []claude.FileChange{},
				Summary: "Test response",
			}, nil
		},
	}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)
	executor.WithDisallowedTools("Bash(rm:*),Bash(sudo:*)")

	// Create task
	task := &webhook.Task{
		ID:     "test-1",
		Repo:   "owner/repo",
		Number: 1,
		Branch: "main",
		Prompt: "test prompt",
	}

	// Execute with custom clone function that avoids actual git operations
	executor.cloneFn = func(repo, branch, token string) (workdir string, cleanup func(), err error) {
		_ = token
		tmpDir, _ := os.MkdirTemp("", "test-clone-*")
		// Initialize git repo
		cmd := exec.Command("git", "init")
		cmd.Dir = tmpDir
		cmd.Run()
		return tmpDir, func() { os.RemoveAll(tmpDir) }, nil
	}

	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 123, nil
	}
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return nil
	}

	err := executor.Execute(context.Background(), task)
	// We expect some error because we're using mocks, but the important thing is that
	// disallowed_tools was passed through
	if err != nil && !strings.Contains(err.Error(), "push") {
		// Errors other than push failures are unexpected in this test
		t.Logf("Execute() error (expected): %v", err)
	}
}

// TestExecutor_SmartBranchStrategy tests the intelligent branch selection logic
func TestExecutor_SmartBranchStrategy_OpenPR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-smart-branch-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo
	commands := [][]string{
		{"git", "init"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "checkout", "-b", "main"},
	}
	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Git command failed: %v", err)
		}
	}

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("initial"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	// Create existing PR branch
	exec.Command("git", "-C", tmpDir, "checkout", "-b", "feature-branch").Run()
	os.WriteFile(testFile, []byte("feature"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "feature").Run()

	// Setup mock executor
	mockGH := github.NewMockGHClient()
	mockAuth := &mockAppAuth{}
	mockProvider := &mockProvider{
		name: "test",
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			return &claude.CodeResponse{
				Files: []claude.FileChange{
					{Path: "test.txt", Content: "updated content"},
				},
				Summary: "Updated file",
			}, nil
		},
	}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)
	executor.cloneFn = func(repo, branch, token string) (workdir string, cleanup func(), err error) {
		_ = token
		return tmpDir, func() {}, nil
	}

	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 123, nil
	}
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return nil
	}

	// Test 1: Open PR should push to existing branch
	task := &webhook.Task{
		ID:       "test-open-pr",
		Repo:     "owner/repo",
		Number:   1,
		Branch:   "main",
		Prompt:   "test",
		IsPR:     true,
		PRBranch: "feature-branch",
		PRState:  "open",
	}

	err = executor.Execute(context.Background(), task)
	// Push will fail without remote, but we can verify the branch logic
	if err != nil && !strings.Contains(err.Error(), "push") && !strings.Contains(err.Error(), "remote") {
		t.Logf("Execute() completed with expected error: %v", err)
	}

	// Verify we're on the PR branch
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = tmpDir
	output, _ := cmd.Output()
	currentBranch := strings.TrimSpace(string(output))
	if currentBranch != "feature-branch" {
		t.Errorf("Expected to be on feature-branch, got: %s", currentBranch)
	}
}

// TestExecutor_SmartBranchStrategy_NewBranch tests that issues create new branches
func TestExecutor_SmartBranchStrategy_NewBranch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-new-branch-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo
	commands := [][]string{
		{"git", "init"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "user.email", "test@test.com"},
	}
	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Git command failed: %v", err)
		}
	}

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("initial"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	// Setup executor
	mockGH := github.NewMockGHClient()
	mockAuth := &mockAppAuth{}
	mockProvider := &mockProvider{
		name: "test",
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			return &claude.CodeResponse{
				Files: []claude.FileChange{
					{Path: "new.txt", Content: "new content"},
				},
				Summary: "Added new file",
			}, nil
		},
	}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)
	executor.cloneFn = func(repo, branch, token string) (workdir string, cleanup func(), err error) {
		_ = token
		return tmpDir, func() {}, nil
	}

	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 123, nil
	}
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return nil
	}

	// Test: Issue should create new swe branch
	task := &webhook.Task{
		ID:     "test-issue",
		Repo:   "owner/repo",
		Number: 42,
		Branch: "main",
		Prompt: "test",
		IsPR:   false, // Issue, not PR
	}

	err = executor.Execute(context.Background(), task)
	// Push will fail without remote
	if err != nil && !strings.Contains(err.Error(), "push") && !strings.Contains(err.Error(), "remote") {
		t.Logf("Execute() completed with expected error: %v", err)
	}

	// Verify new branch was created with swe/issue prefix
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = tmpDir
	output, _ := cmd.Output()
	currentBranch := strings.TrimSpace(string(output))
	if !strings.HasPrefix(currentBranch, "swe/issue-42-") {
		t.Errorf("Expected branch to start with 'swe/issue-42-', got: %s", currentBranch)
	}
}
