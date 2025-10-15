package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cexll/swe/internal/provider/claude"
	"github.com/cexll/swe/internal/webhook"
)

// Additional comprehensive tests for executor package

func TestApplyChanges_ComprehensiveScenarios(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		name        string
		changes     []claude.FileChange
		setupDir    func(string) error
		verifyFiles func(string, *testing.T)
	}{
		{
			name: "create files in multiple directories",
			changes: []claude.FileChange{
				{Path: "pkg/api/handler.go", Content: "package api"},
				{Path: "pkg/db/conn.go", Content: "package db"},
				{Path: "cmd/server/main.go", Content: "package main"},
			},
			setupDir: func(dir string) error {
				return nil
			},
			verifyFiles: func(dir string, t *testing.T) {
				paths := []string{
					"pkg/api/handler.go",
					"pkg/db/conn.go",
					"cmd/server/main.go",
				}
				for _, p := range paths {
					fullPath := filepath.Join(dir, p)
					if _, err := os.Stat(fullPath); os.IsNotExist(err) {
						t.Errorf("File %s was not created", p)
					}
				}
			},
		},
		{
			name: "overwrite existing files preserving directory structure",
			changes: []claude.FileChange{
				{Path: "config/app.yaml", Content: "updated: true"},
			},
			setupDir: func(dir string) error {
				cfgDir := filepath.Join(dir, "config")
				if err := os.MkdirAll(cfgDir, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(cfgDir, "app.yaml"), []byte("updated: false"), 0644)
			},
			verifyFiles: func(dir string, t *testing.T) {
				content, err := os.ReadFile(filepath.Join(dir, "config/app.yaml"))
				if err != nil {
					t.Errorf("Failed to read file: %v", err)
					return
				}
				if string(content) != "updated: true" {
					t.Errorf("File content = %s, want 'updated: true'", string(content))
				}
			},
		},
		{
			name: "create files with various extensions",
			changes: []claude.FileChange{
				{Path: "README.md", Content: "# Project"},
				{Path: "config.json", Content: "{}"},
				{Path: "script.sh", Content: "#!/bin/bash"},
				{Path: "data.txt", Content: "data"},
			},
			setupDir: func(dir string) error {
				return nil
			},
			verifyFiles: func(dir string, t *testing.T) {
				files := []string{"README.md", "config.json", "script.sh", "data.txt"}
				for _, f := range files {
					if _, err := os.Stat(filepath.Join(dir, f)); os.IsNotExist(err) {
						t.Errorf("File %s was not created", f)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.setupDir != nil {
				if err := tt.setupDir(tmpDir); err != nil {
					t.Fatalf("Setup failed: %v", err)
				}
			}

			err := executor.applyChanges(tmpDir, tt.changes)
			if err != nil {
				t.Errorf("applyChanges() error = %v", err)
				return
			}

			if tt.verifyFiles != nil {
				tt.verifyFiles(tmpDir, t)
			}
		})
	}
}

func TestCreatePRLink_ComprehensiveURLs(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		name         string
		repo         string
		head         string
		base         string
		title        string
		validateURL  func(string) error
		wantContains []string
	}{
		{
			name:  "standard GitHub URL",
			repo:  "facebook/react",
			head:  "feature/hooks",
			base:  "main",
			title: "Add new hooks",
			wantContains: []string{
				"github.com/facebook/react",
				"compare",
				"main...feature%2Fhooks",
				"title=Add+new+hooks",
			},
		},
		{
			name:  "title with special URL characters",
			repo:  "user/project",
			head:  "fix",
			base:  "develop",
			title: "Fix bug #123 & improve performance",
			wantContains: []string{
				"github.com/user/project",
				"develop...fix",
				"title=Fix+bug+%23123+%26+improve+performance",
			},
		},
		{
			name:  "long branch names",
			repo:  "org/repo",
			head:  "feature/implement-advanced-caching-mechanism",
			base:  "main",
			title: "Implement advanced caching",
			wantContains: []string{
				"feature%2Fimplement-advanced-caching-mechanism",
				"title=Implement+advanced+caching",
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

			for _, want := range tt.wantContains {
				if !strings.Contains(url, want) {
					t.Errorf("URL = %s, should contain %s", url, want)
				}
			}

			// Verify URL structure
			if !strings.HasPrefix(url, "https://github.com/") {
				t.Errorf("URL should start with https://github.com/")
			}
			if !strings.Contains(url, "compare") {
				t.Errorf("URL should contain 'compare'")
			}
			if !strings.Contains(url, "expand=1") {
				t.Errorf("URL should contain 'expand=1'")
			}

			// New behavior: quick_pull and body params should be present
			if !strings.Contains(url, "quick_pull=1") {
				t.Errorf("URL should contain 'quick_pull=1'")
			}
			if !strings.Contains(url, "body=") {
				t.Errorf("URL should contain 'body=' parameter")
			}
		})
	}
}

func TestExecutor_Initialization(t *testing.T) {
	provider := &mockProvider{name: "test"}

	executor := New(provider, nil)

	if executor == nil {
		t.Fatal("New() returned nil")
	}

	if executor.provider == nil {
		t.Error("Provider not set in executor")
	}

	if executor.provider.Name() != "test" {
		t.Errorf("Provider name = %s, want test", executor.provider.Name())
	}
}

func TestMockProvider_Behavior(t *testing.T) {
	callCount := 0
	provider := &mockProvider{
		generateFunc: func(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
			callCount++
			return &claude.CodeResponse{
				Files: []claude.FileChange{
					{Path: "test.go", Content: "package test"},
				},
				Summary: "Generated code",
				CostUSD: 0.02,
			}, nil
		},
	}

	ctx := context.Background()
	req := &claude.CodeRequest{
		Prompt:   "test prompt",
		RepoPath: "/tmp/test",
	}

	// First call
	resp1, err := provider.GenerateCode(ctx, req)
	if err != nil {
		t.Errorf("GenerateCode() error = %v", err)
	}
	if resp1 == nil {
		t.Error("GenerateCode() returned nil")
	}
	if callCount != 1 {
		t.Errorf("Call count = %d, want 1", callCount)
	}

	// Second call
	_, _ = provider.GenerateCode(ctx, req)
	if callCount != 2 {
		t.Errorf("Call count = %d, want 2", callCount)
	}
}

func TestTask_CompleteWorkflow(t *testing.T) {
	// This test verifies the complete structure of Task
	task := &webhook.Task{
		Repo:       "owner/repo",
		Number:     456,
		Branch:     "develop",
		Prompt:     "implement feature",
		IssueTitle: "Feature Request",
		IssueBody:  "Please implement this feature",
		IsPR:       true,
	}

	// Verify all fields are set
	if task.Repo == "" {
		t.Error("Repo should be set")
	}
	if task.Number == 0 {
		t.Error("Number should be non-zero")
	}
	if task.Branch == "" {
		t.Error("Branch should be set")
	}
	if task.Prompt == "" {
		t.Error("Prompt should be set")
	}
	if !task.IsPR {
		t.Error("IsPR should be true")
	}

	// Test task with minimum required fields
	minTask := &webhook.Task{
		Repo:   "test/repo",
		Number: 1,
		Branch: "main",
		Prompt: "fix",
	}

	if minTask.Repo == "" || minTask.Number == 0 || minTask.Branch == "" || minTask.Prompt == "" {
		t.Error("Minimum required fields should be set")
	}
}

func TestCodeResponse_Structure(t *testing.T) {
	// Test various CodeResponse structures
	tests := []struct {
		name     string
		response *claude.CodeResponse
		valid    bool
	}{
		{
			name: "complete response",
			response: &claude.CodeResponse{
				Files: []claude.FileChange{
					{Path: "file1.go", Content: "content1"},
					{Path: "file2.go", Content: "content2"},
				},
				Summary: "Changes summary",
				CostUSD: 0.05,
			},
			valid: true,
		},
		{
			name: "single file response",
			response: &claude.CodeResponse{
				Files: []claude.FileChange{
					{Path: "main.go", Content: "package main"},
				},
				Summary: "Added main",
				CostUSD: 0.01,
			},
			valid: true,
		},
		{
			name: "zero cost response",
			response: &claude.CodeResponse{
				Files: []claude.FileChange{
					{Path: "test.go", Content: "test"},
				},
				Summary: "Test",
				CostUSD: 0.0,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				if len(tt.response.Files) == 0 {
					t.Error("Valid response should have files")
				}
				if tt.response.Summary == "" {
					t.Error("Valid response should have summary")
				}
				if tt.response.CostUSD < 0 {
					t.Error("Cost should not be negative")
				}
			}
		})
	}
}
