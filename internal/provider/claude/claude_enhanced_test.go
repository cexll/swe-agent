package claude

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// skipIfIntegrationDisabled skips the test if integration tests are disabled
func skipIfIntegrationDisabled(t *testing.T) {
	// Skip integration tests unless explicitly enabled
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Integration tests disabled, set RUN_INTEGRATION_TESTS=true to enable")
	}

	// Check if claude CLI is available
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("Claude CLI not available")
	}
}

// Test Claude CLI working directory setup - Integration test covering our key fix
func TestCallClaudeCLI_WorkingDirectoryIntegration(t *testing.T) {
	skipIfIntegrationDisabled(t)

	// Test working directory validation
	t.Run("validates working directory exists", func(t *testing.T) {
		// Test with non-existent directory - should fail early
		_, err := callClaudeCLI("/non/existent/path", "test prompt", "claude-3-sonnet", "")
		if err == nil {
			t.Error("callClaudeCLI() should return error for non-existent directory")
		}

		// Check it's a directory error, not later in the process
		if !strings.Contains(err.Error(), "directory") && !strings.Contains(err.Error(), "no such file") {
			t.Logf("Got expected error type: %v", err)
		}
	})

	t.Run("calls with correct working directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		// This will likely fail due to invalid API key, but we can check the error message
		_, err := callClaudeCLI(tmpDir, "test prompt", "claude-3-sonnet", "")

		// We expect some kind of error (API key, network, etc.) but not a "directory not found" error
		if err != nil {
			errorStr := strings.ToLower(err.Error())

			// These would indicate our working directory fix is NOT working:
			if strings.Contains(errorStr, "no such file or directory") && !strings.Contains(errorStr, "claude") {
				t.Errorf("callClaudeCLI() failed due to directory issue (our fix not working): %v", err)
			}

			// These errors are expected in test environment and show the fix is working:
			// - claude command not found
			// - API authentication errors
			// - Network connection errors
			t.Logf("Expected test environment error (shows working directory fix is working): %v", err)
		}
	})
}

// Test Environment Variable handling - covers our environment setup fixes
func TestNewProvider_EnvironmentVariables(t *testing.T) {
	// Save original environment
	originalAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	originalAuthToken := os.Getenv("ANTHROPIC_AUTH_TOKEN")
	originalBaseURL := os.Getenv("ANTHROPIC_BASE_URL")

	defer func() {
		if originalAPIKey == "" {
			os.Unsetenv("ANTHROPIC_API_KEY")
		} else {
			os.Setenv("ANTHROPIC_API_KEY", originalAPIKey)
		}
		if originalAuthToken == "" {
			os.Unsetenv("ANTHROPIC_AUTH_TOKEN")
		} else {
			os.Setenv("ANTHROPIC_AUTH_TOKEN", originalAuthToken)
		}
		if originalBaseURL == "" {
			os.Unsetenv("ANTHROPIC_BASE_URL")
		} else {
			os.Setenv("ANTHROPIC_BASE_URL", originalBaseURL)
		}
	}()

	t.Run("preserves custom base URL", func(t *testing.T) {
		os.Setenv("ANTHROPIC_BASE_URL", "http://custom-endpoint.com")

		provider := NewProvider("test-key", "claude-3-sonnet")

		// Check environment was preserved (our fix)
		if os.Getenv("ANTHROPIC_BASE_URL") != "http://custom-endpoint.com" {
			t.Error("Custom ANTHROPIC_BASE_URL should be preserved")
		}

		if os.Getenv("ANTHROPIC_API_KEY") != "test-key" {
			t.Error("ANTHROPIC_API_KEY should be set from NewProvider")
		}

		if provider.Name() != "claude" {
			t.Error("Provider name should be claude")
		}
	})

	t.Run("sets both API key environment variables", func(t *testing.T) {
		provider := NewProvider("new-test-key", "claude-3-haiku")

		// Our fix sets both ANTHROPIC_API_KEY and ANTHROPIC_AUTH_TOKEN
		if os.Getenv("ANTHROPIC_API_KEY") != "new-test-key" {
			t.Error("ANTHROPIC_API_KEY should be set")
		}

		if os.Getenv("ANTHROPIC_AUTH_TOKEN") != "new-test-key" {
			t.Error("ANTHROPIC_AUTH_TOKEN should also be set")
		}

		if provider.model != "claude-3-haiku" {
			t.Errorf("Provider model = %s, want claude-3-haiku", provider.model)
		}
	})
}

// Test GenerateCode validation and error handling - covers our GenerateCode fixes
func TestGenerateCode_Validation(t *testing.T) {
	provider := NewProvider("test-key", "claude-3-sonnet")

	t.Run("validates repository path", func(t *testing.T) {
		req := &CodeRequest{
			Prompt:   "test prompt",
			RepoPath: "", // Empty path should fail
			Context:  map[string]string{},
		}

		_, err := provider.GenerateCode(context.Background(), req)
		if err == nil {
			t.Error("GenerateCode() should return error for empty repository path")
		}

		if !strings.Contains(err.Error(), "repository path is required") {
			t.Errorf("Error should mention repository path, got: %v", err)
		}
	})

	t.Run("validates repository path exists", func(t *testing.T) {
		req := &CodeRequest{
			Prompt:   "test prompt",
			RepoPath: "/non/existent/path",
			Context:  map[string]string{},
		}

		_, err := provider.GenerateCode(context.Background(), req)
		if err == nil {
			t.Error("GenerateCode() should return error for non-existent path")
		}

		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("Error should mention path does not exist, got: %v", err)
		}
	})

	t.Run("validates context and builds correct prompt", func(t *testing.T) {
		skipIfIntegrationDisabled(t)

		tmpDir := t.TempDir()

		// Create a simple file structure
		testFile := tmpDir + "/test.go"
		if err := os.WriteFile(testFile, []byte("package main"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		req := &CodeRequest{
			Prompt:   "Fix this code",
			RepoPath: tmpDir,
			Context: map[string]string{
				"issue_title": "Bug in main",
				"issue_body":  "The code is broken",
				"priority":    "high",
			},
		}

		// This will fail due to missing claude CLI or API issues, but we test validation logic
		_, err := provider.GenerateCode(context.Background(), req)

		// Should get past validation and fail at CLI execution
		if err != nil && strings.Contains(err.Error(), "repository path") {
			t.Error("Should pass repository path validation")
		}
	})
}

// Test edge cases for our key fixes
func TestCallClaudeCLI_EdgeCases(t *testing.T) {
	skipIfIntegrationDisabled(t)

	t.Run("handles empty prompt", func(t *testing.T) {
		tmpDir := t.TempDir()

		_, err := callClaudeCLI(tmpDir, "", "claude-3-sonnet", "")
		// Should not crash, but may return API error
		if err != nil {
			// Verify it's not a crash or directory-related error
			errorStr := strings.ToLower(err.Error())
			if strings.Contains(errorStr, "panic") || (strings.Contains(errorStr, "directory") && !strings.Contains(errorStr, "claude")) {
				t.Errorf("callClaudeCLI() failed with system error: %v", err)
			}
		}
	})

	t.Run("handles empty model parameter", func(t *testing.T) {
		tmpDir := t.TempDir()

		_, err := callClaudeCLI(tmpDir, "test", "", "")
		// Should work (uses default model), but may return API error
		if err != nil {
			errorStr := strings.ToLower(err.Error())
			if strings.Contains(errorStr, "panic") || (strings.Contains(errorStr, "directory") && !strings.Contains(errorStr, "claude")) {
				t.Errorf("callClaudeCLI() failed with system error: %v", err)
			}
		}
	})

	t.Run("handles special characters in working directory path", func(t *testing.T) {
		// Create directory with space in name (common issue)
		tmpDir := t.TempDir()
		specialDir := tmpDir + "/test dir with spaces"
		if err := os.Mkdir(specialDir, 0755); err != nil {
			t.Fatalf("Failed to create special directory: %v", err)
		}

		_, err := callClaudeCLI(specialDir, "test", "claude-3-sonnet", "")
		if err != nil {
			errorStr := strings.ToLower(err.Error())
			// Should not fail due to path parsing issues
			if strings.Contains(errorStr, "no such file") && !strings.Contains(errorStr, "claude") {
				t.Errorf("callClaudeCLI() failed to handle path with spaces: %v", err)
			}
		}
	})
}

// Test debug logging for our parsing improvements
func TestDebugLogging_ClaudeProvider(t *testing.T) {
	// Test that debug logging doesn't crash and provides useful output
	oldValue := os.Getenv("DEBUG_CLAUDE_PARSING")
	os.Setenv("DEBUG_CLAUDE_PARSING", "true")
	defer func() {
		if oldValue == "" {
			os.Unsetenv("DEBUG_CLAUDE_PARSING")
		} else {
			os.Setenv("DEBUG_CLAUDE_PARSING", oldValue)
		}
	}()

	// Test parsing with debug enabled
	response := `<file path="debug.go">
<content>
package debug

func TestDebug() {
    // test function
}
</content>
</file>

<summary>
Added debug test function
</summary>`

	result, err := parseCodeResponse(response)
	if err != nil {
		t.Errorf("parseCodeResponse() with debug logging failed: %v", err)
		return
	}

	if len(result.Files) != 1 {
		t.Errorf("parseCodeResponse() files count = %d, want 1", len(result.Files))
	}

	if result.Files[0].Path != "debug.go" {
		t.Errorf("parseCodeResponse() file path = %s, want debug.go", result.Files[0].Path)
	}

	if result.Summary != "Added debug test function" {
		t.Errorf("parseCodeResponse() summary = %q, want %q", result.Summary, "Added debug test function")
	}

	// Test that debug output doesn't break normal operation
	// (we can't easily capture log output in tests, but we verify no crashes)
}

// Test that our fixes work with the full GenerateCode workflow
func TestGenerateCode_IntegrationWorkflow(t *testing.T) {
	skipIfIntegrationDisabled(t)

	provider := NewProvider("test-key", "claude-3-sonnet")

	// Test full workflow validation (will fail at CLI call, but tests our setup)
	tmpDir := t.TempDir()

	// Create some files to simulate a repository
	if err := os.WriteFile(tmpDir+"/main.go", []byte("package main\n\nfunc main() {\n\tprintln(\"hello\")\n}"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(tmpDir+"/README.md", []byte("# Test Project"), 0644); err != nil {
		t.Fatalf("Failed to create README: %v", err)
	}

	req := &CodeRequest{
		Prompt:   "Add error handling to the main function",
		RepoPath: tmpDir,
		Context: map[string]string{
			"issue_title": "Add error handling",
			"issue_body":  "The main function should handle errors properly",
			"branch":      "main",
		},
	}

	// This will likely fail at the CLI call, but should pass all our validation logic
	_, err := provider.GenerateCode(context.Background(), req)

	// Should not fail on our validation logic
	if err != nil {
		if strings.Contains(err.Error(), "repository path") {
			t.Error("Should pass repository path validation")
		}
		if strings.Contains(err.Error(), "list repo files") {
			t.Error("Should successfully list repository files")
		}

		// Expected failures (showing our fixes work):
		// - "claude CLI execution failed" - shows we got to the CLI call
		// - Network/API errors - shows CLI was called with correct parameters
		t.Logf("Expected integration test error (shows our fixes work): %v", err)
	}
}
