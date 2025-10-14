package codex

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/cexll/swe/internal/prompt"
	"github.com/cexll/swe/internal/provider/claude"
)

func TestNewProvider_Name(t *testing.T) {
	provider := NewProvider("", "", "gpt-5-codex")
	if provider.Name() != "codex" {
		t.Fatalf("Name() = %s, want codex", provider.Name())
	}
}

func TestNewProvider_APIKey(t *testing.T) {
	// Test that API key is set in environment
	testKey := "test-api-key"
	testBaseURL := "https://api.example.com"
	provider := NewProvider(testKey, testBaseURL, "gpt-5-codex")

	if provider.apiKey != testKey {
		t.Errorf("apiKey = %s, want %s", provider.apiKey, testKey)
	}

	if provider.baseURL != testBaseURL {
		t.Errorf("baseURL = %s, want %s", provider.baseURL, testBaseURL)
	}

	if provider.model != "gpt-5-codex" {
		t.Errorf("model = %s, want gpt-5-codex", provider.model)
	}
}

func TestListRepoFiles(t *testing.T) {
	// Test list repo files functionality (no need to test CLI execution)
	// This is a unit test for the helper function
	manager := prompt.NewManager()
	files, err := manager.ListRepoFiles(".")
	if err != nil {
		t.Fatalf("listRepoFiles() error = %v", err)
	}

	// Should find at least the test file itself
	found := false
	for _, f := range files {
		if f == "codex_test.go" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("listRepoFiles() should find codex_test.go, got: %v", files)
	}
}

// TestInvokeCodex_CommandConstruction tests that the command is constructed correctly
func TestInvokeCodex_CommandConstruction(t *testing.T) {
	provider := NewProvider("test-key", "https://api.test.com", "gpt-5-codex")

	// Mock the exec.CommandContext to capture arguments
	originalExec := execCommandContext
	defer func() { execCommandContext = originalExec }()

	var capturedArgs []string
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		// Return a command that will fail quickly
		return exec.Command("false")
	}

	// Call invokeCodex
	ctx := context.Background()
	_, _, _ = provider.invokeCodex(ctx, "test prompt", "/tmp/test")

	// Verify command structure
	expectedArgs := []string{
		"codex",
		"exec",
		"-m", "gpt-5-codex",
		"-c", `model_reasoning_effort="high"`,
		"--dangerously-bypass-approvals-and-sandbox",
		"-C", "/tmp/test",
		"test prompt",
	}

	if len(capturedArgs) != len(expectedArgs) {
		t.Errorf("Command args length = %d, want %d", len(capturedArgs), len(expectedArgs))
	}

	for i, arg := range expectedArgs {
		if i < len(capturedArgs) && capturedArgs[i] != arg {
			t.Errorf("Arg[%d] = %s, want %s", i, capturedArgs[i], arg)
		}
	}
}

// TestInvokeCodex_Timeout tests that timeout is enforced
func TestInvokeCodex_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	provider := NewProvider("", "", "gpt-5-codex")

	// Mock the exec.CommandContext to return a long-running command
	originalExec := execCommandContext
	defer func() { execCommandContext = originalExec }()

	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Return a command that sleeps
		return exec.CommandContext(ctx, "sleep", "60")
	}

	// Set a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, _, err := provider.invokeCodex(ctx, "test prompt", "/tmp/test")
	duration := time.Since(start)

	if err == nil {
		t.Error("invokeCodex should return error on timeout")
	}

	if duration > 2*time.Second {
		t.Errorf("Timeout took too long: %v", duration)
	}

	if !contains(err.Error(), "timeout") && ctx.Err() != context.DeadlineExceeded {
		t.Errorf("Error should mention timeout or context should be canceled, got: %v", err)
	}
}

// TestParseCodeResponse tests the response parsing logic
func TestParseCodeResponse(t *testing.T) {
	tests := []struct {
		name        string
		response    string
		wantFiles   int
		wantSummary string
		wantErr     bool
	}{
		{
			name: "valid response with files",
			response: `<file path="main.go"><content>package main

func main() {
}
</content></file>

<summary>Created main.go</summary>`,
			wantFiles:   1,
			wantSummary: "Created main.go",
			wantErr:     false,
		},
		{
			name: "multiple files",
			response: `<file path="main.go"><content>package main</content></file>
<file path="utils.go"><content>package utils</content></file>
<summary>Created multiple files</summary>`,
			wantFiles:   2,
			wantSummary: "Created multiple files",
			wantErr:     false,
		},
		{
			name:        "analysis only (no files)",
			response:    `<summary>This is an analysis of the code.</summary>`,
			wantFiles:   0,
			wantSummary: "This is an analysis of the code.",
			wantErr:     false,
		},
		{
			name:        "raw text (no tags)",
			response:    `This is a raw response without tags.`,
			wantFiles:   0,
			wantSummary: "This is a raw response without tags.",
			wantErr:     false,
		},
		{
			name:        "empty response",
			response:    "",
			wantFiles:   0,
			wantSummary: "",
			wantErr:     true,
		},
		{
			name: "placeholder template response",
			response: `<file path="path/to/file.ext">
<content>
... full file content here ...
</content>
</file>

<summary>
Brief description of changes made
</summary>`,
			wantFiles:   0,
			wantSummary: "",
			wantErr:     true,
		},
		{
			name: "placeholder summary with real file path",
			response: `<file path="main.go"><content>package main</content></file>
<summary>
Brief description of changes made
</summary>`,
			wantFiles:   0,
			wantSummary: "",
			wantErr:     true,
		},
		{
			name: "relative placeholder path and content",
			response: `<file path="relative/path/to/file.go"><content>
package example

// entire updated file content here
</content></file>
<summary>
Add user authentication to handler.go
</summary>`,
			wantFiles:   0,
			wantSummary: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseCodeResponse(tt.response)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseCodeResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			if len(result.Files) != tt.wantFiles {
				t.Errorf("Files count = %d, want %d", len(result.Files), tt.wantFiles)
			}

			if result.Summary != tt.wantSummary {
				t.Errorf("Summary = %q, want %q", result.Summary, tt.wantSummary)
			}
		})
	}
}

// TestGenerateCode_Integration tests the full GenerateCode flow (without actual codex execution)
func TestGenerateCode_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	provider := NewProvider("", "", "gpt-5-codex")

	// Mock the exec.CommandContext to return a valid response
	originalExec := execCommandContext
	defer func() { execCommandContext = originalExec }()

	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Return a command that outputs a valid response
		return exec.Command("echo", "<summary>Test response</summary>")
	}

	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create a test file in the temp directory
	testFile := tmpDir + "/test.go"
	if err := os.WriteFile(testFile, []byte("package test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	ctx := context.Background()
	req := &claude.CodeRequest{
		Prompt:   "Test prompt",
		RepoPath: tmpDir,
		Context:  map[string]string{"test": "context"},
	}

	result, err := provider.GenerateCode(ctx, req)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}

	if result.Summary == "" {
		t.Error("GenerateCode() should return a summary")
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
