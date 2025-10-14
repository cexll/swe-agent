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

// ===== Phase 2: Multi-PR Workflow Tests =====

// TestExecutor_ExecuteMultiPR_IndependentPRs tests independent PR creation
func TestExecutor_ExecuteMultiPR_IndependentPRs(t *testing.T) {
	// Setup: Create a real temporary git repo
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

	// Provider returns files in different categories (enough to trigger split)
	mockProvider := &mockProvider{
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			// Simulate creating 10 files in different categories to trigger split
			files := []claude.FileChange{
				{Path: "internal/auth_test.go", Content: "package internal\n\nfunc TestAuth(t *testing.T) {}"},
				{Path: "internal/client_test.go", Content: "package internal\n\nfunc TestClient(t *testing.T) {}"},
				{Path: "internal/config_test.go", Content: "package internal\n\nfunc TestConfig(t *testing.T) {}"},
				{Path: "internal/auth.go", Content: "package internal\n\nfunc Auth() {}"},
				{Path: "internal/client.go", Content: "package internal\n\nfunc Client() {}"},
				{Path: "internal/config.go", Content: "package internal\n\nfunc Config() {}"},
				{Path: "README.md", Content: "# Updated docs"},
				{Path: "CHANGELOG.md", Content: "# Changelog"},
				{Path: "docs/guide.md", Content: "# Guide"},
			}

			return &claude.CodeResponse{
				Files:   files,
				Summary: "Added tests, implementation, and docs",
				CostUSD: 0.10,
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
		Repo:       "owner/repo",
		Number:     123,
		Branch:     "main",
		Prompt:     "Add tests and docs",
		IssueTitle: "Feature request",
		IssueBody:  "Please add",
		Username:   "testuser",
	}

	// Execute
	err := executor.Execute(context.Background(), task)

	// Push will fail (no remote), but we should get to Multi-PR workflow
	if err != nil && !strings.Contains(err.Error(), "push") && !strings.Contains(err.Error(), "remote") {
		t.Errorf("Execute() failed unexpectedly: %v", err)
	}

	// Verify UpdateComment was called multiple times (once per PR + final)
	if len(mockGH.UpdateCommentCalls) < 2 {
		t.Errorf("Expected at least 2 UpdateComment calls for multi-PR workflow, got %d", len(mockGH.UpdateCommentCalls))
	}

	for i, call := range mockGH.UpdateCommentCalls {
		t.Logf("UpdateComment #%d body:\n%s", i+1, call.Body)
	}

	// Verify split plan was mentioned in comments
	foundSplitPlan := false
	for _, call := range mockGH.UpdateCommentCalls {
		if strings.Contains(call.Body, "Split into Multiple PRs") || strings.Contains(call.Body, "🔀") {
			foundSplitPlan = true
			break
		}
	}

	if !foundSplitPlan {
		t.Error("Expected to find split plan in comment updates")
	}
}

// TestExecutor_ExecuteMultiPR_WithDependencies tests dependent PR handling
func TestExecutor_ExecuteMultiPR_WithDependencies(t *testing.T) {
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
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return nil
	}

	mockAuth := &mockAppAuth{}

	// Provider returns files that will trigger dependency relationships
	// tests (independent), internal (depends on tests), core (depends on internal)
	mockProvider := &mockProvider{
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			// Create 10 files to trigger split with dependencies
			files := []claude.FileChange{
				{Path: "internal/auth_test.go", Content: "package internal\n\nfunc TestAuth(t *testing.T) {}"},
				{Path: "internal/client_test.go", Content: "package internal\n\nfunc TestClient(t *testing.T) {}"},
				{Path: "internal/config_test.go", Content: "package internal\n\nfunc TestConfig(t *testing.T) {}"},
				{Path: "internal/auth.go", Content: "package internal\n\nfunc Auth() {}"},
				{Path: "internal/client.go", Content: "package internal\n\nfunc Client() {}"},
				{Path: "internal/config.go", Content: "package internal\n\nfunc Config() {}"},
				{Path: "pkg/core/handler.go", Content: "package core\n\nfunc Handle() {}"},
				{Path: "pkg/core/processor.go", Content: "package core\n\nfunc Process() {}"},
				{Path: "pkg/core/validator.go", Content: "package core\n\nfunc Validate() {}"},
			}

			return &claude.CodeResponse{
				Files:   files,
				Summary: "Added tests, internal, and core",
				CostUSD: 0.12,
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
		Number:   456,
		Branch:   "main",
		Prompt:   "Add infrastructure",
		Username: "testuser",
	}

	// Execute
	err := executor.Execute(context.Background(), task)

	// Push will fail, but Multi-PR workflow should run
	if err != nil && !strings.Contains(err.Error(), "push") && !strings.Contains(err.Error(), "remote") {
		t.Errorf("Execute() failed unexpectedly: %v", err)
	}

	// Verify comment updates mention dependencies
	foundWaitingDependencies := false
	for _, call := range mockGH.UpdateCommentCalls {
		if strings.Contains(call.Body, "waiting for dependencies") {
			foundWaitingDependencies = true
			break
		}
	}

	if !foundWaitingDependencies {
		t.Error("Expected to find 'waiting for dependencies' in comment updates")
	}
}

// TestExecutor_ExecuteMultiPR_PartialFailure tests partial PR creation failure
func TestExecutor_ExecuteMultiPR_PartialFailure(t *testing.T) {
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
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return nil
	}

	mockAuth := &mockAppAuth{}

	// Provider returns enough files to trigger split
	mockProvider := &mockProvider{
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			files := []claude.FileChange{
				{Path: "file1_test.go", Content: "package test"},
				{Path: "file2_test.go", Content: "package test"},
				{Path: "file3_test.go", Content: "package test"},
				{Path: "README.md", Content: "# Docs"},
				{Path: "pkg/core1.go", Content: "package core"},
				{Path: "pkg/core2.go", Content: "package core"},
				{Path: "pkg/core3.go", Content: "package core"},
				{Path: "pkg/core4.go", Content: "package core"},
				{Path: "pkg/core5.go", Content: "package core"},
			}

			return &claude.CodeResponse{
				Files:   files,
				Summary: "Added multiple files",
				CostUSD: 0.15,
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
		Number:   789,
		Branch:   "main",
		Prompt:   "Add many files",
		Username: "testuser",
	}

	// Execute
	err := executor.Execute(context.Background(), task)

	// Should complete even if some PRs fail to push
	// (push will fail due to no remote, but workflow should continue)
	if err != nil && !strings.Contains(err.Error(), "push") && !strings.Contains(err.Error(), "remote") {
		t.Errorf("Execute() failed unexpectedly: %v", err)
	}

	// Verify multiple update calls
	if len(mockGH.UpdateCommentCalls) < 2 {
		t.Errorf("Expected multiple UpdateComment calls, got %d", len(mockGH.UpdateCommentCalls))
	}
}

// TestExecutor_CommitSubPR_AtomicOperation tests commit sub PR git operations
func TestExecutor_CommitSubPR_AtomicOperation(t *testing.T) {
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
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	executor := &Executor{}

	// Create a SubPR with specific files
	subPR := github.SubPR{
		Index:       0,
		Name:        "Add test infrastructure",
		Description: "Test files",
		Files: []claude.FileChange{
			{Path: "test1.go", Content: "package test\n\nfunc Test1() {}"},
			{Path: "test2.go", Content: "package test\n\nfunc Test2() {}"},
		},
		Category: github.CategoryTests,
	}

	branchName := generateSubPRBranchName(123, string(github.CategoryTests))

	task := &webhook.Task{
		Number:   123,
		IsPR:     false,
		Username: "testuser",
	}

	// Execute commitSubPR
	err := executor.commitSubPR(tmpDir, "owner/repo", branchName, subPR, task, "")

	// Push will fail (no remote), but commit should succeed
	if err != nil && !strings.Contains(err.Error(), "push") && !strings.Contains(err.Error(), "remote") {
		t.Errorf("commitSubPR() failed before push: %v", err)
	}

	// Verify branch was created
	cmd = exec.Command("git", "branch", "--list", branchName)
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to list branches: %v", err)
	}

	if !strings.Contains(string(output), branchName) {
		t.Errorf("Branch %s was not created", branchName)
	}

	// Verify files exist in the branch
	for _, file := range subPR.Files {
		fullPath := filepath.Join(tmpDir, file.Path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("File %s was not created: %v", file.Path, err)
			continue
		}
		if string(content) != file.Content {
			t.Errorf("File %s content = %q, want %q", file.Path, string(content), file.Content)
		}
	}

	// Verify commit was created with correct message
	cmd = exec.Command("git", "log", "-1", "--pretty=%B")
	cmd.Dir = tmpDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to get commit message: %v", err)
	}

	commitMsg := string(output)
	if !strings.Contains(commitMsg, subPR.Name) {
		t.Errorf("Commit message should contain SubPR name, got: %s", commitMsg)
	}
}

// TestExecutor_CommitSubPR_OnlyCommitsSpecifiedFiles tests file isolation
func TestExecutor_CommitSubPR_OnlyCommitsSpecifiedFiles(t *testing.T) {
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

	// Create initial commit with multiple files
	for _, filename := range []string{"base1.go", "base2.go", "base3.go"} {
		if err := os.WriteFile(filepath.Join(tmpDir, filename), []byte("package base"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = tmpDir
	cmd.Run()

	executor := &Executor{}

	// Create SubPR with only 2 files (not all 3)
	subPR := github.SubPR{
		Index:       0,
		Name:        "Update only 2 files",
		Description: "Selective update",
		Files: []claude.FileChange{
			{Path: "base1.go", Content: "package base // modified"},
			{Path: "base2.go", Content: "package base // modified"},
			// Note: base3.go is NOT included
		},
		Category: github.CategoryCore,
	}

	branchName := generateSubPRBranchName(789, string(github.CategoryCore))

	task := &webhook.Task{
		Number:   789,
		IsPR:     false,
		Username: "testuser",
	}

	// Execute commitSubPR
	err := executor.commitSubPR(tmpDir, "owner/repo", branchName, subPR, task, "")

	// Push will fail, but reset/clean/apply should work
	if err != nil && !strings.Contains(err.Error(), "push") && !strings.Contains(err.Error(), "remote") {
		t.Errorf("commitSubPR() failed unexpectedly: %v", err)
	}

	// Verify only the 2 specified files were modified
	content1, _ := os.ReadFile(filepath.Join(tmpDir, "base1.go"))
	content2, _ := os.ReadFile(filepath.Join(tmpDir, "base2.go"))
	content3, _ := os.ReadFile(filepath.Join(tmpDir, "base3.go"))

	if !strings.Contains(string(content1), "modified") {
		t.Error("base1.go should be modified")
	}
	if !strings.Contains(string(content2), "modified") {
		t.Error("base2.go should be modified")
	}
	if strings.Contains(string(content3), "modified") {
		t.Error("base3.go should NOT be modified")
	}
}

// TestExecutor_GetChangedFiles tests file change detection
func TestExecutor_GetChangedFiles(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(tmpDir, "existing.go"), []byte("package existing"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = tmpDir
	cmd.Run()

	// Modify file and add new file
	os.WriteFile(filepath.Join(tmpDir, "existing.go"), []byte("package existing // modified"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "new.go"), []byte("package new"), 0644)

	executor := &Executor{}
	changes, err := executor.getChangedFiles(tmpDir)

	if err != nil {
		t.Fatalf("getChangedFiles() error = %v", err)
	}

	if len(changes) != 2 {
		t.Errorf("getChangedFiles() returned %d files, want 2", len(changes))
	}

	// Verify we got both files
	foundExisting := false
	foundNew := false
	for _, change := range changes {
		if change.Path == "existing.go" {
			foundExisting = true
			if !strings.Contains(change.Content, "modified") {
				t.Error("existing.go content should contain 'modified'")
			}
		}
		if change.Path == "new.go" {
			foundNew = true
		}
	}

	if !foundExisting {
		t.Error("Should detect existing.go modification")
	}
	if !foundNew {
		t.Error("Should detect new.go addition")
	}
}
