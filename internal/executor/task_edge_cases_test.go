package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/provider/claude"
	"github.com/cexll/swe/internal/webhook"
)

// ===== Edge Case Tests =====

// TestExecutor_GetChangedFiles_DeletedFiles tests detection of deleted files
func TestExecutor_GetChangedFiles_DeletedFiles(t *testing.T) {
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

	// Create initial commit with 3 files
	for _, filename := range []string{"file1.go", "file2.go", "file3.go"} {
		os.WriteFile(filepath.Join(tmpDir, filename), []byte("package test"), 0644)
	}
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	// Delete file1.go and modify file2.go
	os.Remove(filepath.Join(tmpDir, "file1.go"))
	os.WriteFile(filepath.Join(tmpDir, "file2.go"), []byte("package test // modified"), 0644)

	executor := &Executor{}
	changes, err := executor.getChangedFiles(tmpDir)

	if err != nil {
		t.Fatalf("getChangedFiles() error = %v", err)
	}

	// Should only detect file2.go (modified), file1.go deletion is skipped
	if len(changes) != 1 {
		t.Errorf("getChangedFiles() returned %d files, want 1 (deleted files should be skipped)", len(changes))
	}

	if len(changes) > 0 && changes[0].Path != "file2.go" {
		t.Errorf("Expected to find file2.go, got %s", changes[0].Path)
	}
}

// TestExecutor_GetChangedFiles_BinaryFiles tests handling of binary files
func TestExecutor_GetChangedFiles_BinaryFiles(t *testing.T) {
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
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("initial"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	// Add text file and binary file
	os.WriteFile(filepath.Join(tmpDir, "text.go"), []byte("package main"), 0644)
	// Create a simple binary file (PNG header)
	binaryData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	os.WriteFile(filepath.Join(tmpDir, "image.png"), binaryData, 0644)

	executor := &Executor{}
	changes, err := executor.getChangedFiles(tmpDir)

	if err != nil {
		t.Fatalf("getChangedFiles() error = %v", err)
	}

	// Should detect both files
	if len(changes) != 2 {
		t.Errorf("getChangedFiles() returned %d files, want 2", len(changes))
	}

	// Verify text file
	foundText := false
	foundBinary := false
	for _, change := range changes {
		if change.Path == "text.go" {
			foundText = true
			if change.Content != "package main" {
				t.Errorf("text.go content incorrect")
			}
		}
		if change.Path == "image.png" {
			foundBinary = true
			// Binary content should be read as-is
			if len(change.Content) != len(binaryData) {
				t.Errorf("image.png content length = %d, want %d", len(change.Content), len(binaryData))
			}
		}
	}

	if !foundText {
		t.Error("Should detect text.go")
	}
	if !foundBinary {
		t.Error("Should detect image.png")
	}
}

// TestExecutor_GetChangedFiles_EmptyDirectory tests handling of empty directories
func TestExecutor_GetChangedFiles_EmptyDirectory(t *testing.T) {
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
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("initial"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	// Create empty directory (git won't track it, but filesystem has it)
	os.MkdirAll(filepath.Join(tmpDir, "empty_dir"), 0755)
	// Create directory with .gitkeep
	os.MkdirAll(filepath.Join(tmpDir, "tracked_empty"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "tracked_empty/.gitkeep"), []byte(""), 0644)

	executor := &Executor{}
	changes, err := executor.getChangedFiles(tmpDir)

	if err != nil {
		t.Fatalf("getChangedFiles() error = %v", err)
	}

	// Should only detect .gitkeep file
	if len(changes) != 1 {
		t.Errorf("getChangedFiles() returned %d files, want 1 (.gitkeep)", len(changes))
	}

	if len(changes) > 0 && changes[0].Path != "tracked_empty/.gitkeep" {
		t.Errorf("Expected tracked_empty/.gitkeep, got %s", changes[0].Path)
	}
}

// TestExecutor_CommitSubPR_GitResetFailure tests handling of git reset failure
func TestExecutor_CommitSubPR_GitResetFailure(t *testing.T) {
	tmpDir := t.TempDir()

	// Don't initialize git repo - this will cause git reset to fail
	executor := &Executor{}

	subPR := github.SubPR{
		Index:       0,
		Name:        "Test PR",
		Description: "Test",
		Files: []claude.FileChange{
			{Path: "test.go", Content: "package test"},
		},
		Category: github.CategoryTests,
	}

	task := &webhook.Task{
		Number:   123,
		IsPR:     false,
		Username: "testuser",
	}

	err := executor.commitSubPR(tmpDir, "test-branch", subPR, task)

	// Should fail with git error
	if err == nil {
		t.Error("commitSubPR() should fail when git is not initialized")
	}

	if err != nil && !strings.Contains(err.Error(), "git") {
		t.Errorf("commitSubPR() error should mention git, got: %v", err)
	}
}

// TestExecutor_CommitSubPR_BranchAlreadyExists tests handling of existing branch
func TestExecutor_CommitSubPR_BranchAlreadyExists(t *testing.T) {
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
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("initial"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	// Create a branch
	branchName := "test-branch"
	exec.Command("git", "-C", tmpDir, "checkout", "-b", branchName).Run()
	exec.Command("git", "-C", tmpDir, "checkout", "master").Run()

	executor := &Executor{}
	subPR := github.SubPR{
		Index:       0,
		Name:        "Test PR",
		Description: "Test",
		Files: []claude.FileChange{
			{Path: "test.go", Content: "package test"},
		},
		Category: github.CategoryTests,
	}

	task := &webhook.Task{
		Number:   456,
		IsPR:     false,
		Username: "testuser",
	}

	// Try to create branch with same name
	err := executor.commitSubPR(tmpDir, branchName, subPR, task)

	// Should fail because branch already exists
	if err == nil {
		t.Error("commitSubPR() should fail when branch already exists")
	}

	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Logf("commitSubPR() error = %v", err)
		// Note: The error might also be about checkout failure
	}
}

// TestExecutor_ExecuteMultiPR_GitHubAPIFailure tests GitHub API error handling
func TestExecutor_ExecuteMultiPR_GitHubAPIFailure(t *testing.T) {
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
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("initial"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	// Setup mocks with API failure
	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 0, fmt.Errorf("API rate limit exceeded")
	}
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return fmt.Errorf("API timeout")
	}

	mockAuth := &mockAppAuth{}
	mockProvider := &mockProvider{
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			// Return enough files to trigger split
			files := []claude.FileChange{
				{Path: "file1_test.go", Content: "package test"},
				{Path: "file2_test.go", Content: "package test"},
				{Path: "file3_test.go", Content: "package test"},
				{Path: "file4_test.go", Content: "package test"},
				{Path: "file5_test.go", Content: "package test"},
				{Path: "file6_test.go", Content: "package test"},
				{Path: "file7_test.go", Content: "package test"},
				{Path: "file8_test.go", Content: "package test"},
				{Path: "file9_test.go", Content: "package test"},
			}
			return &claude.CodeResponse{
				Files:   files,
				Summary: "Test changes",
				CostUSD: 0.05,
			}, nil
		},
	}

	mockClone := func(repo, branch, token string) (string, func(), error) {
		_ = token
		return tmpDir, func() {}, nil
	}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)
	executor.cloneFn = mockClone

	task := &webhook.Task{
		Repo:     "owner/repo",
		Number:   999,
		Branch:   "main",
		Prompt:   "Test",
		Username: "testuser",
	}

	// Execute should handle API failures gracefully
	err := executor.Execute(context.Background(), task)

	// Even if comment API fails, execution should continue
	// Error will be from git push (no remote), not from API
	if err != nil && !strings.Contains(err.Error(), "push") && !strings.Contains(err.Error(), "remote") {
		t.Errorf("Execute() failed with unexpected error: %v", err)
	}

	// Verify CreateComment was attempted but failed
	if len(mockGH.CreateCommentCalls) == 0 {
		t.Error("CreateComment should have been called")
	}
}

// TestExecutor_ExecuteMultiPR_ProviderFailure tests AI provider failure handling
func TestExecutor_ExecuteMultiPR_ProviderFailure(t *testing.T) {
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
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("initial"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 12345, nil
	}
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return nil
	}

	mockAuth := &mockAppAuth{}

	// Provider returns error
	mockProvider := &mockProvider{
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			return nil, fmt.Errorf("API quota exceeded")
		},
	}

	mockClone := func(repo, branch, token string) (string, func(), error) {
		_ = token
		return tmpDir, func() {}, nil
	}

	executor := NewWithClient(mockProvider, mockAuth, mockGH)
	executor.cloneFn = mockClone

	task := &webhook.Task{
		Repo:     "owner/repo",
		Number:   888,
		Branch:   "main",
		Prompt:   "Test",
		Username: "testuser",
	}

	// Execute should handle provider failure
	err := executor.Execute(context.Background(), task)

	if err == nil {
		t.Error("Execute() should fail when provider fails")
	}

	if err != nil && !strings.Contains(err.Error(), "mock") {
		t.Errorf("Execute() error should mention provider, got: %v", err)
	}

	// Verify UpdateComment was called to report error
	if len(mockGH.UpdateCommentCalls) == 0 {
		t.Error("UpdateComment should be called to report provider error")
	}
}

// TestExecutor_GetChangedFiles_LargeDirectory tests handling of directories with many files
func TestExecutor_GetChangedFiles_LargeDirectory(t *testing.T) {
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
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("initial"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	// Create directory with 50 files
	os.MkdirAll(filepath.Join(tmpDir, "large_dir"), 0755)
	for i := 0; i < 50; i++ {
		filename := fmt.Sprintf("file%02d.go", i)
		os.WriteFile(filepath.Join(tmpDir, "large_dir", filename), []byte(fmt.Sprintf("package pkg%d", i)), 0644)
	}

	executor := &Executor{}
	changes, err := executor.getChangedFiles(tmpDir)

	if err != nil {
		t.Fatalf("getChangedFiles() error = %v", err)
	}

	// Should detect all 50 files
	if len(changes) != 50 {
		t.Errorf("getChangedFiles() returned %d files, want 50", len(changes))
	}

	// Verify all files are under large_dir/
	for _, change := range changes {
		if !strings.HasPrefix(change.Path, "large_dir/") {
			t.Errorf("File %s should be under large_dir/", change.Path)
		}
	}
}
