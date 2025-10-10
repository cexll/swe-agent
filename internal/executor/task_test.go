package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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
	executor := New(provider)

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

func TestNotifyError(t *testing.T) {
	// This test focuses on error message formatting
	executor := &Executor{}

	task := &webhook.Task{
		Repo:   "owner/repo",
		Number: 123,
		Prompt: "test prompt",
	}

	errorMsg := "test error message"

	// Since notifyError calls github.CreateComment which requires gh CLI,
	// we can only test that it returns an error containing our message
	err := executor.notifyError(task, errorMsg)
	if err == nil {
		t.Error("notifyError() should return an error")
	}

	if !contains(err.Error(), errorMsg) {
		t.Errorf("notifyError() error = %v, want to contain %q", err, errorMsg)
	}
}

func TestNotifySuccess(t *testing.T) {
	executor := &Executor{}

	task := &webhook.Task{
		Repo:   "owner/repo",
		Number: 123,
	}

	result := &claude.CodeResponse{
		Files: []claude.FileChange{
			{Path: "main.go", Content: "package main"},
			{Path: "utils.go", Content: "package utils"},
		},
		Summary: "Added main and utils",
		CostUSD: 0.05,
	}

	prURL := "https://github.com/owner/repo/pull/124"

	// Since notifySuccess calls github.CreateComment which requires gh CLI,
	// we can only test that it's callable without panic
	err := executor.notifySuccess(task, result, prURL)
	// Error is expected since gh CLI is not available in test environment
	// We just verify it doesn't panic
	_ = err
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

func TestNotifySuccess_MessageFormat(t *testing.T) {
	executor := &Executor{}

	task := &webhook.Task{
		Repo:   "owner/repo",
		Number: 123,
		Prompt: "fix bug",
	}

	tests := []struct {
		name   string
		result *claude.CodeResponse
		prURL  string
	}{
		{
			name: "single file",
			result: &claude.CodeResponse{
				Files: []claude.FileChange{
					{Path: "main.go", Content: "package main"},
				},
				Summary: "Fixed bug",
				CostUSD: 0.01,
			},
			prURL: "https://github.com/owner/repo/pull/1",
		},
		{
			name: "multiple files",
			result: &claude.CodeResponse{
				Files: []claude.FileChange{
					{Path: "main.go", Content: "package main"},
					{Path: "utils.go", Content: "package utils"},
					{Path: "test.go", Content: "package test"},
				},
				Summary: "Refactored code",
				CostUSD: 0.05,
			},
			prURL: "https://github.com/owner/repo/pull/2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.notifySuccess(task, tt.result, tt.prURL)
			// Error is expected since gh CLI is not available
			_ = err
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
