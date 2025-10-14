package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cexll/swe/internal/prompt"
)

var testPromptManager = prompt.NewManager()

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
			name:        "no file changes (text response only)",
			response:    `<summary>Nothing to do</summary>`,
			wantErr:     false,
			wantFiles:   0,
			wantSummary: "Nothing to do",
		},
		{
			name:        "plain text response without tags",
			response:    `This is a plain text analysis without any XML tags. The issue is caused by X and can be fixed by doing Y.`,
			wantErr:     false,
			wantFiles:   0,
			wantSummary: "This is a plain text analysis without any XML tags. The issue is caused by X and can be fixed by doing Y.",
		},
		{
			name:     "empty response",
			response: "",
			wantErr:  true,
		},
		{
			name: "malformed file tag (missing content tag, uses raw as summary)",
			response: `<file path="test.go">
package test
</file>`,
			wantErr:     false,
			wantFiles:   0,
			wantSummary: "<file path=\"test.go\">\npackage test\n</file>",
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

func TestBuildDefaultSystemPrompt(t *testing.T) {
	files := []string{
		"main.go",
		"utils/helper.go",
		"README.md",
	}

	context := map[string]string{
		"issue_title": "Fix bug in login",
		"issue_body":  "The login function crashes",
		"priority":    "P1",
	}

	prompt := testPromptManager.BuildDefaultSystemPrompt(files, context)

	// Check that files are included
	for _, file := range files {
		if !strings.Contains(prompt, file) {
			t.Errorf("testPromptManager.BuildDefaultSystemPrompt() does not contain file %s", file)
		}
	}

	// Issue title/body should be omitted from additional context (already present in main prompt)
	if strings.Contains(prompt, "Fix bug in login") {
		t.Error("testPromptManager.BuildDefaultSystemPrompt() should not duplicate issue title in system prompt")
	}

	if !strings.Contains(prompt, "<pr_or_issue_body>\nThe login function crashes\n</pr_or_issue_body>") {
		t.Error("testPromptManager.BuildDefaultSystemPrompt() should include issue body within <pr_or_issue_body> section")
	}

	// Custom context should appear
	if !strings.Contains(prompt, "- priority: P1") {
		t.Error("testPromptManager.BuildDefaultSystemPrompt() should include non-issue context entries")
	}

	// Check for key instructions
	expectedInstructions := []string{
		"You are an AI assistant designed to help with GitHub issues and pull requests",
		"Repository structure",
		"conduct your analysis inside <analysis> tags",
	}

	for _, instruction := range expectedInstructions {
		if !strings.Contains(prompt, instruction) {
			t.Errorf("testPromptManager.BuildDefaultSystemPrompt() does not contain instruction: %s", instruction)
		}
	}

	// Ensure XML format instructions are NOT present (removed to match claude-code-action)
	if strings.Contains(prompt, "<file path=\"path/to/file\">") {
		t.Error("testPromptManager.BuildDefaultSystemPrompt() should NOT contain XML format instructions")
	}
}

func TestBuildDefaultSystemPrompt_EmptyContext(t *testing.T) {
	files := []string{"main.go"}
	context := map[string]string{}

	prompt := testPromptManager.BuildDefaultSystemPrompt(files, context)

	if !strings.Contains(prompt, "main.go") {
		t.Error("testPromptManager.BuildDefaultSystemPrompt() does not contain file")
	}

	// Should not contain "Additional context:" section
	if strings.Contains(prompt, "Additional context:") {
		t.Error("testPromptManager.BuildDefaultSystemPrompt() should not contain Additional context section when context is empty")
	}
}

func TestBuildDefaultSystemPrompt_PartialContext(t *testing.T) {
	files := []string{"main.go"}
	context := map[string]string{
		"issue_title": "Test issue",
		"issue_body":  "", // Empty value should be skipped
		"environment": "staging",
		"notes":       "", // Empty value should be skipped
	}

	prompt := testPromptManager.BuildDefaultSystemPrompt(files, context)

	if !strings.Contains(prompt, "- environment: staging") {
		t.Error("testPromptManager.BuildDefaultSystemPrompt() should include non-issue context")
	}

	if strings.Contains(prompt, "Test issue") {
		t.Error("testPromptManager.BuildDefaultSystemPrompt() should not include issue title in additional context")
	}

	if strings.Contains(prompt, "notes") {
		t.Error("testPromptManager.BuildDefaultSystemPrompt() should skip empty context values")
	}
}

func TestBuildCommitPrompt(t *testing.T) {
	files := []string{"main.go"}
	context := map[string]string{
		"issue_body":       "Fix login bug",
		"repository":       "owner/repo",
		"event_type":       "PULL_REQUEST",
		"trigger_context":  "pull request opened",
		"is_pr":            "true",
		"pr_number":        "123",
		"trigger_username": "octocat",
		"trigger_phrase":   "@claude",
		"event_name":       "issue_comment",
		"trigger_comment":  "@claude please finalize the commit",
	}

	prompt := testPromptManager.BuildCommitPrompt(files, context)

	if !strings.Contains(prompt, "You are an AI assistant responsible for finalizing Git commits") {
		t.Error("BuildCommitPrompt() should describe commit responsibilities")
	}

	if !strings.Contains(prompt, "<commit_message>") {
		t.Error("BuildCommitPrompt() should request commit_message section")
	}

	if !strings.Contains(prompt, "<commit_body>") {
		t.Error("BuildCommitPrompt() should request commit_body section")
	}

	if !strings.Contains(prompt, "<testing>") {
		t.Error("BuildCommitPrompt() should request testing section")
	}

	if !strings.Contains(prompt, "<follow_up>") {
		t.Error("BuildCommitPrompt() should request follow_up section")
	}

	// When commit signing is disabled (default), instructions should include git commands
	if !strings.Contains(prompt, "Bash(git add <files>)") {
		t.Error("BuildCommitPrompt() should include git commit instructions when commit signing is disabled")
	}

	if !strings.Contains(prompt, "<repository>owner/repo</repository>") {
		t.Error("BuildCommitPrompt() should include repository metadata")
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
	files, err := testPromptManager.ListRepoFiles(tmpDir)
	if err != nil {
		t.Fatalf("testPromptManager.ListRepoFiles() error = %v", err)
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
			t.Errorf("testPromptManager.ListRepoFiles() missing expected file: %s", expectedFile)
		}
	}

	// Check that .git files are not included
	for _, file := range files {
		if strings.HasPrefix(file, ".git") {
			t.Errorf("testPromptManager.ListRepoFiles() should not include .git files, found: %s", file)
		}
		if strings.HasPrefix(filepath.Base(file), ".") && file != ".gitignore" {
			t.Errorf("testPromptManager.ListRepoFiles() should not include hidden files, found: %s", file)
		}
	}

	// Check count
	if len(files) != len(testFiles) {
		t.Errorf("testPromptManager.ListRepoFiles() returned %d files, want %d", len(files), len(testFiles))
	}
}

func TestListRepoFiles_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	files, err := testPromptManager.ListRepoFiles(tmpDir)
	if err != nil {
		t.Fatalf("testPromptManager.ListRepoFiles() error = %v", err)
	}

	if len(files) != 0 {
		t.Errorf("testPromptManager.ListRepoFiles() returned %d files for empty directory, want 0", len(files))
	}
}

func TestListRepoFiles_NonexistentDirectory(t *testing.T) {
	_, err := testPromptManager.ListRepoFiles("/nonexistent/directory")
	if err == nil {
		t.Error("testPromptManager.ListRepoFiles() should return error for nonexistent directory")
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
	files, err := testPromptManager.ListRepoFiles(tmpDir)
	if err != nil {
		t.Fatalf("testPromptManager.ListRepoFiles() error = %v", err)
	}

	// Should only contain normal file
	if len(files) != 1 {
		t.Errorf("testPromptManager.ListRepoFiles() returned %d files, want 1", len(files))
	}

	// Should not contain .git files
	for _, file := range files {
		if strings.Contains(file, ".git") {
			t.Errorf("testPromptManager.ListRepoFiles() should not include .git files, found: %s", file)
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

func TestListRepoFiles_ErrorConditions(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() (string, error)
		cleanup func(string)
		wantErr bool
	}{
		{
			name: "symlink in directory",
			setup: func() (string, error) {
				tmpDir := t.TempDir()
				// Create a file
				file := filepath.Join(tmpDir, "target.txt")
				if err := os.WriteFile(file, []byte("target"), 0644); err != nil {
					return "", err
				}
				// Create symlink
				link := filepath.Join(tmpDir, "link.txt")
				if err := os.Symlink(file, link); err != nil {
					return "", err
				}
				return tmpDir, nil
			},
			wantErr: false, // Should handle symlinks gracefully
		},
		{
			name: "directory with permission issues",
			setup: func() (string, error) {
				// This test is platform-dependent and may not work on all systems
				tmpDir := t.TempDir()
				// Create subdirectory
				subdir := filepath.Join(tmpDir, "restricted")
				if err := os.Mkdir(subdir, 0755); err != nil {
					return "", err
				}
				// Create file in subdirectory
				file := filepath.Join(subdir, "file.txt")
				if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
					return "", err
				}
				// Try to restrict permissions (may not work on all platforms)
				if err := os.Chmod(subdir, 0000); err != nil {
					return "", err
				}
				return tmpDir, nil
			},
			cleanup: func(dir string) {
				// Restore permissions for cleanup
				subdir := filepath.Join(dir, "restricted")
				os.Chmod(subdir, 0755)
			},
			wantErr: true, // Should error on permission denied
		},
		{
			name: "empty directory with .git only",
			setup: func() (string, error) {
				tmpDir := t.TempDir()
				gitDir := filepath.Join(tmpDir, ".git")
				if err := os.Mkdir(gitDir, 0755); err != nil {
					return "", err
				}
				// Create files in .git (should be ignored)
				if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("config"), 0644); err != nil {
					return "", err
				}
				return tmpDir, nil
			},
			wantErr: false, // Should return empty list
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := tt.setup()
			if err != nil {
				t.Skipf("Setup failed: %v", err)
			}

			if tt.cleanup != nil {
				defer tt.cleanup(dir)
			}

			files, err := testPromptManager.ListRepoFiles(dir)

			if tt.wantErr {
				if err == nil {
					t.Error("testPromptManager.ListRepoFiles() should return error")
				}
			} else {
				if err != nil {
					t.Errorf("testPromptManager.ListRepoFiles() unexpected error: %v", err)
				}
				// Verify no .git files are included
				for _, file := range files {
					if strings.Contains(file, ".git") {
						t.Errorf("testPromptManager.ListRepoFiles() should not include .git files, found: %s", file)
					}
				}
			}
		})
	}
}

func TestListRepoFiles_LargeDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create many files
	numFiles := 100
	for i := 0; i < numFiles; i++ {
		filename := filepath.Join(tmpDir, fmt.Sprintf("file%03d.txt", i))
		if err := os.WriteFile(filename, []byte(fmt.Sprintf("content %d", i)), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	files, err := testPromptManager.ListRepoFiles(tmpDir)
	if err != nil {
		t.Fatalf("testPromptManager.ListRepoFiles() error: %v", err)
	}

	if len(files) != numFiles {
		t.Errorf("testPromptManager.ListRepoFiles() returned %d files, want %d", len(files), numFiles)
	}
}

func TestListRepoFiles_MixedContent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mix of files and directories
	structure := map[string]string{
		"README.md":            "readme",
		"src/main.go":          "package main",
		"src/utils/util.go":    "package utils",
		"docs/guide.md":        "guide",
		".gitignore":           "*.log",    // Should be ignored (hidden)
		".github/workflow.yml": "workflow", // Should be ignored
	}

	for path, content := range structure {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	files, err := testPromptManager.ListRepoFiles(tmpDir)
	if err != nil {
		t.Fatalf("testPromptManager.ListRepoFiles() error: %v", err)
	}

	// Should include non-hidden files (including files in hidden directories like .github)
	// Note: listRepoFiles skips hidden files but not files inside hidden directories
	expectedFiles := []string{"README.md", "src/main.go", "src/utils/util.go", "docs/guide.md", ".github/workflow.yml"}
	if len(files) != len(expectedFiles) {
		t.Errorf("testPromptManager.ListRepoFiles() returned %d files, want %d files", len(files), len(expectedFiles))
		t.Logf("Got files: %v", files)
	}

	// Verify no hidden files
	for _, file := range files {
		if strings.HasPrefix(filepath.Base(file), ".") {
			t.Errorf("testPromptManager.ListRepoFiles() should not include hidden files, found: %s", file)
		}
	}
}
