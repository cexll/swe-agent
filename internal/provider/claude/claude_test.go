package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewProvider(t *testing.T) {
	apiKey := "sk-ant-test-key"
	model := "claude-3-opus-20240229"

	provider := NewProvider(apiKey, model)

	if provider == nil {
		t.Fatal("NewProvider() returned nil")
	}

	if provider.Name() != "claude" {
		t.Errorf("Name() = %s, want claude", provider.Name())
	}

	if provider.model != model {
		t.Errorf("model = %s, want %s", provider.model, model)
	}

	// Check that API key was set in environment
	envKey := os.Getenv("ANTHROPIC_API_KEY")
	if envKey != apiKey {
		t.Errorf("ANTHROPIC_API_KEY = %s, want %s", envKey, apiKey)
	}
}

func TestParseCodeResponse(t *testing.T) {
	tests := []struct {
		name        string
		response    string
		wantErr     bool
		wantFiles   int
		wantSummary string
	}{
		{
			name: "single file change",
			response: `<file path="main.go">
<content>
package main

func main() {
    println("Hello")
}
</content>
</file>

<summary>
Added hello world program
</summary>`,
			wantErr:     false,
			wantFiles:   1,
			wantSummary: "Added hello world program",
		},
		{
			name: "multiple file changes",
			response: `<file path="main.go">
<content>
package main
</content>
</file>

<file path="utils.go">
<content>
package utils
</content>
</file>

<summary>
Created main and utils files
</summary>`,
			wantErr:     false,
			wantFiles:   2,
			wantSummary: "Created main and utils files",
		},
		{
			name: "no summary tag (should use default)",
			response: `<file path="test.go">
<content>
package test
</content>
</file>`,
			wantErr:     false,
			wantFiles:   1,
			wantSummary: "Code changes applied",
		},
		{
			name:     "no file changes",
			response: `<summary>Nothing to do</summary>`,
			wantErr:  true,
		},
		{
			name:     "empty response",
			response: "",
			wantErr:  true,
		},
		{
			name: "malformed file tag",
			response: `<file path="test.go">
package test
</file>`,
			wantErr: true,
		},
		{
			name: "file with special characters in path",
			response: `<file path="src/utils/helper-functions.go">
<content>
package utils
</content>
</file>

<summary>
Added helper functions
</summary>`,
			wantErr:     false,
			wantFiles:   1,
			wantSummary: "Added helper functions",
		},
		{
			name: "multiline content with formatting",
			response: `<file path="main.go">
<content>
package main

import (
    "fmt"
)

func main() {
    fmt.Println("Hello, World!")
}
</content>
</file>

<summary>
Implemented main function with proper formatting
</summary>`,
			wantErr:     false,
			wantFiles:   1,
			wantSummary: "Implemented main function with proper formatting",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseCodeResponse(tt.response)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseCodeResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if len(result.Files) != tt.wantFiles {
				t.Errorf("Files count = %d, want %d", len(result.Files), tt.wantFiles)
			}

			if result.Summary != tt.wantSummary {
				t.Errorf("Summary = %q, want %q", result.Summary, tt.wantSummary)
			}

			// Verify file contents are not empty
			for i, file := range result.Files {
				if file.Path == "" {
					t.Errorf("File[%d].Path is empty", i)
				}
				if file.Content == "" {
					t.Errorf("File[%d].Content is empty", i)
				}
			}
		})
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	files := []string{
		"main.go",
		"utils/helper.go",
		"README.md",
	}

	context := map[string]string{
		"issue_title": "Fix bug in login",
		"issue_body":  "The login function crashes",
	}

	prompt := buildSystemPrompt(files, context)

	// Check that files are included
	for _, file := range files {
		if !strings.Contains(prompt, file) {
			t.Errorf("buildSystemPrompt() does not contain file %s", file)
		}
	}

	// Check that context is included
	if !strings.Contains(prompt, "Fix bug in login") {
		t.Error("buildSystemPrompt() does not contain issue title")
	}
	if !strings.Contains(prompt, "The login function crashes") {
		t.Error("buildSystemPrompt() does not contain issue body")
	}

	// Check for key instructions
	expectedInstructions := []string{
		"code modification assistant",
		"Repository structure",
		"complete file content",
	}

	for _, instruction := range expectedInstructions {
		if !strings.Contains(prompt, instruction) {
			t.Errorf("buildSystemPrompt() does not contain instruction: %s", instruction)
		}
	}
}

func TestBuildSystemPrompt_EmptyContext(t *testing.T) {
	files := []string{"main.go"}
	context := map[string]string{}

	prompt := buildSystemPrompt(files, context)

	if !strings.Contains(prompt, "main.go") {
		t.Error("buildSystemPrompt() does not contain file")
	}

	// Should not contain "Additional Context:" section
	if strings.Contains(prompt, "Additional Context:") {
		t.Error("buildSystemPrompt() should not contain Additional Context section when context is empty")
	}
}

func TestBuildSystemPrompt_PartialContext(t *testing.T) {
	files := []string{"main.go"}
	context := map[string]string{
		"issue_title": "Test issue",
		"issue_body":  "", // Empty value should be skipped
	}

	prompt := buildSystemPrompt(files, context)

	if !strings.Contains(prompt, "Test issue") {
		t.Error("buildSystemPrompt() does not contain non-empty context value")
	}

	// Should contain "Additional Context:" but not the empty issue_body
	lines := strings.Split(prompt, "\n")
	bodyLineCount := 0
	for _, line := range lines {
		if strings.Contains(line, "issue_body") {
			bodyLineCount++
		}
	}
	if bodyLineCount > 0 {
		t.Error("buildSystemPrompt() should not include empty context values")
	}
}

func TestListRepoFiles(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()

	// Create test files
	testFiles := []string{
		"main.go",
		"utils/helper.go",
		"utils/test.go",
		"README.md",
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(tmpDir, file)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// Create .git directory (should be ignored)
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("git config"), 0644); err != nil {
		t.Fatalf("Failed to create .git/config: %v", err)
	}

	// Create hidden file (should be ignored)
	hiddenFile := filepath.Join(tmpDir, ".hidden")
	if err := os.WriteFile(hiddenFile, []byte("hidden"), 0644); err != nil {
		t.Fatalf("Failed to create hidden file: %v", err)
	}

	// List files
	files, err := listRepoFiles(tmpDir)
	if err != nil {
		t.Fatalf("listRepoFiles() error = %v", err)
	}

	// Check that test files are included
	for _, expectedFile := range testFiles {
		found := false
		for _, file := range files {
			if file == expectedFile {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("listRepoFiles() missing expected file: %s", expectedFile)
		}
	}

	// Check that .git files are not included
	for _, file := range files {
		if strings.HasPrefix(file, ".git") {
			t.Errorf("listRepoFiles() should not include .git files, found: %s", file)
		}
		if strings.HasPrefix(filepath.Base(file), ".") && file != ".gitignore" {
			t.Errorf("listRepoFiles() should not include hidden files, found: %s", file)
		}
	}

	// Check count
	if len(files) != len(testFiles) {
		t.Errorf("listRepoFiles() returned %d files, want %d", len(files), len(testFiles))
	}
}

func TestListRepoFiles_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	files, err := listRepoFiles(tmpDir)
	if err != nil {
		t.Fatalf("listRepoFiles() error = %v", err)
	}

	if len(files) != 0 {
		t.Errorf("listRepoFiles() returned %d files for empty directory, want 0", len(files))
	}
}

func TestListRepoFiles_NonexistentDirectory(t *testing.T) {
	_, err := listRepoFiles("/nonexistent/directory")
	if err == nil {
		t.Error("listRepoFiles() should return error for nonexistent directory")
	}
}

func TestListRepoFiles_NestedGitDirectory(t *testing.T) {
	// Create temporary directory with nested .git
	tmpDir := t.TempDir()

	// Create nested structure
	nestedDir := filepath.Join(tmpDir, "subdir", ".git", "nested")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create nested .git directory: %v", err)
	}

	// Create file in .git (should be ignored)
	gitFile := filepath.Join(tmpDir, "subdir", ".git", "file.txt")
	if err := os.WriteFile(gitFile, []byte("git file"), 0644); err != nil {
		t.Fatalf("Failed to create file in .git: %v", err)
	}

	// Create normal file
	normalFile := filepath.Join(tmpDir, "subdir", "normal.txt")
	if err := os.WriteFile(normalFile, []byte("normal file"), 0644); err != nil {
		t.Fatalf("Failed to create normal file: %v", err)
	}

	// List files
	files, err := listRepoFiles(tmpDir)
	if err != nil {
		t.Fatalf("listRepoFiles() error = %v", err)
	}

	// Should only contain normal file
	if len(files) != 1 {
		t.Errorf("listRepoFiles() returned %d files, want 1", len(files))
	}

	// Should not contain .git files
	for _, file := range files {
		if strings.Contains(file, ".git") {
			t.Errorf("listRepoFiles() should not include .git files, found: %s", file)
		}
	}
}

func TestCodeRequest_Validation(t *testing.T) {
	// Test CodeRequest structure
	req := &CodeRequest{
		Prompt:   "test prompt",
		RepoPath: "/tmp/repo",
		Context: map[string]string{
			"issue_title": "Test issue",
		},
	}

	if req.Prompt == "" {
		t.Error("Prompt should not be empty")
	}
	if req.RepoPath == "" {
		t.Error("RepoPath should not be empty")
	}
	if len(req.Context) == 0 {
		t.Error("Context should not be empty")
	}
}

func TestCodeResponse_Validation(t *testing.T) {
	// Test CodeResponse structure
	resp := &CodeResponse{
		Files: []FileChange{
			{Path: "test.go", Content: "package test"},
		},
		Summary: "Test summary",
		CostUSD: 0.05,
	}

	if len(resp.Files) == 0 {
		t.Error("Files should not be empty")
	}
	if resp.Summary == "" {
		t.Error("Summary should not be empty")
	}
	if resp.CostUSD < 0 {
		t.Error("CostUSD should not be negative")
	}
}

func TestFileChange_Validation(t *testing.T) {
	// Test FileChange structure
	change := FileChange{
		Path:    "main.go",
		Content: "package main",
	}

	if change.Path == "" {
		t.Error("Path should not be empty")
	}
	if change.Content == "" {
		t.Error("Content should not be empty")
	}
}

func TestParseCodeResponse_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantErr  bool
	}{
		{
			name: "file with nested XML-like tags",
			response: `<file path="test.go">
<content>
package test

// <example>
// This is not a real tag
// </example>
</content>
</file>

<summary>Added test file</summary>`,
			wantErr: false,
		},
		{
			name: "summary with special characters",
			response: `<file path="test.go">
<content>
package test
</content>
</file>

<summary>Fixed bug #123 & added feature</summary>`,
			wantErr: false,
		},
		{
			name: "multiple summaries (should use first)",
			response: `<file path="test.go">
<content>
package test
</content>
</file>

<summary>First summary</summary>
<summary>Second summary</summary>`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseCodeResponse(tt.response)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseCodeResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(result.Files) == 0 {
					t.Error("parseCodeResponse() should return at least one file")
				}
				if result.Summary == "" {
					t.Error("parseCodeResponse() should return a summary")
				}
			}
		})
	}
}
