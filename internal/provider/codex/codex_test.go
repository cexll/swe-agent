package codex

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
		"--json",
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
			name: "placeholder file ignored when other files valid",
			response: `<file path="path/to/file.ext"><content>
... full file content here ...
</content></file>
<file path="main.go"><content>package main</content></file>
<summary>Created main.go</summary>`,
			wantFiles:   1,
			wantSummary: "Created main.go",
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
			wantFiles:   1,
			wantSummary: "Code changes applied",
			wantErr:     false,
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
		{
			name: "permission request summary",
			response: `<summary>
The M1 implementation provides:
- WorkflowState data structure for tracking multi-stage workflow
- Clarify prompt generator for Stage 0

Next Steps:
- Create the files listed above
- Add unit tests for each component

Would you like me to proceed with creating these files if you grant the necessary permissions, or would you prefer to create them manually?
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

func TestTruncateLogStringKeepsTail(t *testing.T) {
	original := strings.Repeat("info line\n", 60) + "ERROR: final failure message"
	result := truncateLogString(original, 120)

	if !strings.Contains(result, "ERROR: final failure message") {
		t.Fatalf("truncateLogString should preserve the trailing error, got: %q", result)
	}

	if !strings.Contains(result, "(truncated)") {
		t.Fatalf("truncateLogString should annotate truncation, got: %q", result)
	}
}

func TestTruncateLogStringSmallLimit(t *testing.T) {
	original := "prefix logs\nERROR: boom"
	result := truncateLogString(original, 10)

	expectedSuffix := original[len(original)-len(result):]
	if result != expectedSuffix {
		t.Fatalf("Expected pure suffix truncation, got: %q (want suffix %q)", result, expectedSuffix)
	}
}

func TestTruncateLogStringZeroLimit(t *testing.T) {
	if got := truncateLogString("anything", 0); got != "" {
		t.Fatalf("Expected empty string for non-positive limit, got: %q", got)
	}
}

func TestAggregateCodexOutputJSON(t *testing.T) {
	jsonLines := strings.Join([]string{
		`{"type":"item.completed","item":{"id":"item_0","type":"reasoning","text":"thinking"}}`,
		`{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"final"}}`,
	}, "\n")

	result := aggregateCodexOutput(jsonLines)

	expected := "thinking\n\nfinal"
	if result != expected {
		t.Fatalf("Unexpected aggregation result: %q (want %q)", result, expected)
	}
}

func TestAggregateCodexOutputErrorLine(t *testing.T) {
	jsonLine := `{"type":"error","message":"bad stuff"}`

	result := aggregateCodexOutput(jsonLine)
	if result != "bad stuff" {
		t.Fatalf("Expected error message extraction, got: %q", result)
	}
}

func TestAggregateCodexOutputFallback(t *testing.T) {
	raw := "plain text line"
	if result := aggregateCodexOutput(raw); result != raw {
		t.Fatalf("Expected passthrough for non-JSON, got: %q", result)
	}
}

func TestGenerateCode_JSONOutputFeedsComment(t *testing.T) {
	provider := NewProvider("", "", "gpt-5-codex")

	reasoningLine := `{"type":"item.completed","item":{"type":"reasoning","text":"Analyzing repository files"}}`
	agentLine := `{"type":"item.completed","item":{"type":"agent_message","text":"<file path=\"main.go\"><content>package main\n</content></file>\n<summary>JSON summary</summary>"}}`
	jsonOutput := strings.Join([]string{reasoningLine, agentLine}, "\n")

	originalExec := execCommandContext
	defer func() { execCommandContext = originalExec }()

	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		script := fmt.Sprintf("cat <<'EOF'\n%s\nEOF", jsonOutput)
		return exec.Command("bash", "-lc", script)
	}

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "placeholder.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write placeholder file: %v", err)
	}

	ctx := context.Background()
	req := &claude.CodeRequest{
		Prompt:   "Test prompt",
		RepoPath: tmpDir,
		Context:  map[string]string{"test": "value"},
	}

	result, err := provider.GenerateCode(ctx, req)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}

	if result.Summary != "JSON summary" {
		t.Fatalf("Summary = %q, want %q", result.Summary, "JSON summary")
	}

	if len(result.Files) != 1 {
		t.Fatalf("Files count = %d, want 1", len(result.Files))
	}

	if result.Files[0].Path != "main.go" {
		t.Fatalf("File path = %q, want %q", result.Files[0].Path, "main.go")
	}

	if !strings.Contains(result.Files[0].Content, "package main") {
		t.Fatalf("File content = %q, want to contain %q", result.Files[0].Content, "package main")
	}

	mockGH := github.NewMockGHClient()
	tracker := github.NewCommentTrackerWithClient("owner/repo", 42, "tester", mockGH)
	tracker.CommentID = 100

	tracker.MarkEnd()
	tracker.SetCompleted(result.Summary, nil, result.CostUSD)

	if err := tracker.Update("token"); err != nil {
		t.Fatalf("tracker.Update() error = %v", err)
	}

	if len(mockGH.UpdateCommentCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(mockGH.UpdateCommentCalls))
	}

	if body := mockGH.UpdateCommentCalls[0].Body; !strings.Contains(body, result.Summary) {
		t.Fatalf("comment body %q should contain summary %q", body, result.Summary)
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
