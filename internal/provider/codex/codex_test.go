package codex

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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
	_, _ = provider.invokeCodex(ctx, "test prompt", "/tmp/test")

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
	_, err := provider.invokeCodex(ctx, "test prompt", "/tmp/test")
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

func TestBuildCodexMCPConfig_FullContext(t *testing.T) {
	cases := []struct {
		name      string
		ctx       map[string]string
		wantLines []string
	}{
		{
			name: "all context fields",
			ctx: map[string]string{
				"github_token": "tok_123",
				"comment_id":   "42",
				"repo_owner":   "linux",
				"repo_name":    "kernel",
				"event_name":   "pull_request",
			},
			wantLines: []string{
				"# Dynamically generated Codex configuration",
				"model = \"gpt-5-codex\"",
				"[mcp_servers.github]",
				"type = \"http\"",
				"url = \"https://api.githubcopilot.com/mcp\"",
				"[mcp_servers.github.headers]",
				"Authorization = \"Bearer tok_123\"",
				"[mcp_servers.git]",
				"command = \"uvx\"",
				"args = [\"mcp-server-git\"]",
				"[mcp_servers.comment_updater]",
				"[mcp_servers.comment_updater.env]",
				"GITHUB_TOKEN = \"tok_123\"",
				"REPO_OWNER = \"linux\"",
				"REPO_NAME = \"kernel\"",
				"CLAUDE_COMMENT_ID = \"42\"",
				"GITHUB_EVENT_NAME = \"pull_request\"",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			home := setupTempHome(t)
			ensureUVXAvailability(t, true)

			if err := buildCodexMCPConfig(tc.ctx); err != nil {
				t.Fatalf("buildCodexMCPConfig error: %v", err)
			}

			content := readConfigFile(t, home)

			for _, want := range tc.wantLines {
				if !strings.Contains(content, want) {
					t.Fatalf("config missing line %q\nconfig:\n%s", want, content)
				}
			}

			assertTOMLFormat(t, content)
		})
	}
}

func TestBuildCodexMCPConfig_FileWritten(t *testing.T) {
	cases := []struct {
		name       string
		precreate  bool
		wantMode   os.FileMode
		wantDirMod os.FileMode
	}{
		{
			name:       "creates directory and file",
			precreate:  false,
			wantMode:   0o600,
			wantDirMod: 0o700,
		},
		{
			name:       "reuses existing directory",
			precreate:  true,
			wantMode:   0o600,
			wantDirMod: 0o700,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			home := setupTempHome(t)
			configDir := filepath.Join(home, ".codex")
			configPath := filepath.Join(configDir, "config.toml")

			if tc.precreate {
				if err := os.MkdirAll(configDir, 0o755); err != nil {
					t.Fatalf("precreate dir: %v", err)
				}
			}

			if err := buildCodexMCPConfig(map[string]string{}); err != nil {
				t.Fatalf("buildCodexMCPConfig error: %v", err)
			}

			dirInfo, err := os.Stat(configDir)
			if err != nil {
				t.Fatalf("stat config dir: %v", err)
			}
			if !dirInfo.IsDir() {
				t.Fatalf("config path is not directory")
			}

			fileInfo, err := os.Stat(configPath)
			if err != nil {
				t.Fatalf("stat config file: %v", err)
			}

			if fileInfo.Mode().Perm() != tc.wantMode {
				t.Fatalf("config file permissions = %o, want %o", fileInfo.Mode().Perm(), tc.wantMode)
			}

			// Directory may already exist; ensure at least execute for user.
			if dirInfo.Mode()&0o700 != tc.wantDirMod {
				t.Fatalf("config dir permissions = %o, want prefix %o", dirInfo.Mode()&0o777, tc.wantDirMod)
			}
		})
	}
}

func TestBuildCodexMCPConfig_GitHubServer(t *testing.T) {
	cases := []struct {
		name          string
		ctx           map[string]string
		expectedLines []string
	}{
		{
			name: "token provided",
			ctx: map[string]string{
				"github_token": "ghp_abc",
			},
			expectedLines: []string{
				"[mcp_servers.github]",
				"type = \"http\"",
				"url = \"https://api.githubcopilot.com/mcp\"",
				"[mcp_servers.github.headers]",
				"Authorization = \"Bearer ghp_abc\"",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			home := setupTempHome(t)
			ensureUVXAvailability(t, false)

			if err := buildCodexMCPConfig(tc.ctx); err != nil {
				t.Fatalf("buildCodexMCPConfig error: %v", err)
			}

			content := readConfigFile(t, home)
			for _, line := range tc.expectedLines {
				if !strings.Contains(content, line) {
					t.Fatalf("expected config to contain %q\nconfig:\n%s", line, content)
				}
			}
		})
	}
}

func TestBuildCodexMCPConfig_GitServer(t *testing.T) {
	cases := []struct {
		name          string
		uvxAvailable  bool
		expectedLines []string
		rejectLines   []string
	}{
		{
			name:         "uvx present",
			uvxAvailable: true,
			expectedLines: []string{
				"[mcp_servers.git]",
				"command = \"uvx\"",
				"args = [\"mcp-server-git\"]",
			},
		},
		{
			name:         "uvx missing",
			uvxAvailable: false,
			rejectLines: []string{
				"[mcp_servers.git]",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			home := setupTempHome(t)
			ensureUVXAvailability(t, tc.uvxAvailable)

			if err := buildCodexMCPConfig(map[string]string{}); err != nil {
				t.Fatalf("buildCodexMCPConfig error: %v", err)
			}

			content := readConfigFile(t, home)

			for _, want := range tc.expectedLines {
				if !strings.Contains(content, want) {
					t.Fatalf("expected config to contain %q\nconfig:\n%s", want, content)
				}
			}

			for _, reject := range tc.rejectLines {
				if strings.Contains(content, reject) {
					t.Fatalf("config should not contain %q\nconfig:\n%s", reject, content)
				}
			}
		})
	}
}

func TestBuildCodexMCPConfig_CommentUpdater(t *testing.T) {
	cases := []struct {
		name          string
		ctx           map[string]string
		expectPresent bool
		wantLines     []string
	}{
		{
			name: "full comment context",
			ctx: map[string]string{
				"github_token": "ghp_full",
				"comment_id":   "24",
				"repo_owner":   "torvalds",
				"repo_name":    "linux",
				"event_name":   "issue_comment",
			},
			expectPresent: true,
			wantLines: []string{
				"[mcp_servers.comment_updater]",
				"[mcp_servers.comment_updater.env]",
				"GITHUB_TOKEN = \"ghp_full\"",
				"REPO_OWNER = \"torvalds\"",
				"REPO_NAME = \"linux\"",
				"CLAUDE_COMMENT_ID = \"24\"",
				"GITHUB_EVENT_NAME = \"issue_comment\"",
			},
		},
		{
			name: "missing repo owner",
			ctx: map[string]string{
				"github_token": "ghp_partial",
				"comment_id":   "25",
				"repo_name":    "linux",
			},
			expectPresent: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			home := setupTempHome(t)
			ensureUVXAvailability(t, false)

			if err := buildCodexMCPConfig(tc.ctx); err != nil {
				t.Fatalf("buildCodexMCPConfig error: %v", err)
			}

			content := readConfigFile(t, home)
			hasSection := strings.Contains(content, "[mcp_servers.comment_updater]")

			if tc.expectPresent != hasSection {
				t.Fatalf("comment_updater presence = %t, want %t\nconfig:\n%s", hasSection, tc.expectPresent, content)
			}

			if tc.expectPresent {
				for _, line := range tc.wantLines {
					if !strings.Contains(content, line) {
						t.Fatalf("expected config to contain %q\nconfig:\n%s", line, content)
					}
				}
			}
		})
	}
}

func TestBuildCodexMCPConfig_PartialContext(t *testing.T) {
	cases := []struct {
		name        string
		ctx         map[string]string
		wantPresent []string
		wantAbsent  []string
	}{
		{
			name: "token without comment context",
			ctx: map[string]string{
				"github_token": "ghp_partial",
			},
			wantPresent: []string{
				"[mcp_servers.github]",
				"Authorization = \"Bearer ghp_partial\"",
			},
			wantAbsent: []string{
				"[mcp_servers.comment_updater]",
			},
		},
		{
			name: "comment id missing owner",
			ctx: map[string]string{
				"github_token": "ghp_partial",
				"comment_id":   "55",
				"repo_name":    "linux",
			},
			wantPresent: []string{
				"[mcp_servers.github]",
			},
			wantAbsent: []string{
				"[mcp_servers.comment_updater]",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			home := setupTempHome(t)
			ensureUVXAvailability(t, false)

			if err := buildCodexMCPConfig(tc.ctx); err != nil {
				t.Fatalf("buildCodexMCPConfig error: %v", err)
			}

			content := readConfigFile(t, home)
			for _, line := range tc.wantPresent {
				if !strings.Contains(content, line) {
					t.Fatalf("expected config to contain %q\nconfig:\n%s", line, content)
				}
			}
			for _, line := range tc.wantAbsent {
				if strings.Contains(content, line) {
					t.Fatalf("config should not contain %q\nconfig:\n%s", line, content)
				}
			}
		})
	}
}

func TestGenerateCode_MCPConfigCalled(t *testing.T) {
	cases := []struct {
		name           string
		prepare        func(t *testing.T, home string)
		wantConfig     bool
		wantWarnLogged bool
	}{
		{
			name: "config succeeds",
			prepare: func(t *testing.T, home string) {
				t.Helper()
			},
			wantConfig:     true,
			wantWarnLogged: false,
		},
		{
			name: "config failure logs warning",
			prepare: func(t *testing.T, home string) {
				t.Helper()
				badPath := filepath.Join(home, ".codex")
				if err := os.WriteFile(badPath, []byte("not a dir"), 0o600); err != nil {
					t.Fatalf("write blocking file: %v", err)
				}
			},
			wantConfig:     false,
			wantWarnLogged: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			provider := NewProvider("", "", "gpt-5-codex")

			home := setupTempHome(t)
			tc.prepare(t, home)

			jsonOutput := `{"type":"item.completed","item":{"type":"agent_message","text":"<summary>OK</summary>"}}`
			originalExec := execCommandContext
			defer func() { execCommandContext = originalExec }()
			execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
				return exec.Command("echo", jsonOutput)
			}

			var logBuf bytes.Buffer
			originalLogger := log.Writer()
			log.SetOutput(&logBuf)
			defer log.SetOutput(originalLogger)

			req := &prov.CodeRequest{
				Prompt:   "Test prompt",
				RepoPath: t.TempDir(),
				Context: map[string]string{
					"github_token": "tok",
				},
			}

			resp, err := provider.GenerateCode(context.Background(), req)
			if err != nil {
				t.Fatalf("GenerateCode error: %v", err)
			}
			if resp.Summary == "" {
				t.Fatalf("expected summary to be populated")
			}

			configPath := filepath.Join(home, ".codex", "config.toml")
			_, statErr := os.Stat(configPath)
			if tc.wantConfig && statErr != nil {
				t.Fatalf("expected config file: %v", statErr)
			}
			if !tc.wantConfig && statErr == nil {
				t.Fatalf("config file should not exist at %s", configPath)
			}

			warnLogged := strings.Contains(logBuf.String(), "Warning: failed to build MCP config")
			if warnLogged != tc.wantWarnLogged {
				t.Fatalf("warning logged = %t, want %t\nlogs:\n%s", warnLogged, tc.wantWarnLogged, logBuf.String())
			}
		})
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

func setupTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func ensureUVXAvailability(t *testing.T, present bool) {
	t.Helper()
	dir := t.TempDir()
	if present {
		name := "uvx"
		content := "#!/bin/sh\nexit 0\n"
		mode := os.FileMode(0o755)
		if runtime.GOOS == "windows" {
			name = "uvx.bat"
			content = "@echo off\nexit /b 0\r\n"
			mode = 0o755
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			t.Fatalf("write mock uvx: %v", err)
		}
		// Ensure execution permission on non-windows systems
		if runtime.GOOS != "windows" {
			if err := os.Chmod(path, 0o755); err != nil {
				t.Fatalf("chmod mock uvx: %v", err)
			}
		}
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		return
	}
	t.Setenv("PATH", dir)
}

func readConfigFile(t *testing.T, home string) string {
	t.Helper()
	configPath := filepath.Join(home, ".codex", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	return string(data)
}

func assertTOMLFormat(t *testing.T, content string) {
	t.Helper()
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				t.Fatalf("invalid TOML section header: %q", line)
			}
			continue
		}
		if !strings.Contains(line, "=") {
			t.Fatalf("invalid TOML key-value: %q", line)
		}
		parts := strings.SplitN(line, "=", 2)
		if len(strings.TrimSpace(parts[0])) == 0 || len(strings.TrimSpace(parts[1])) == 0 {
			t.Fatalf("invalid TOML key-value: %q", line)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan error: %v", err)
	}
}
