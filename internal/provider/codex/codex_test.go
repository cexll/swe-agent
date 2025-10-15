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

	"github.com/cexll/swe/internal/prompt"
	prov "github.com/cexll/swe/internal/provider"
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

// parseCodeResponse was removed; tests relying on it are no longer applicable.

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
	req := &prov.CodeRequest{
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
	req := &prov.CodeRequest{
		Prompt:   "Test prompt",
		RepoPath: tmpDir,
		Context:  map[string]string{"test": "value"},
	}

	result, err := provider.GenerateCode(ctx, req)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}

	if !strings.Contains(result.Summary, "JSON summary") {
		t.Fatalf("Summary should contain %q, got %q", "JSON summary", result.Summary)
	}

	// Files removed from response; only Summary is validated.

	// Comment tracker integration removed; only validate summary content.
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
