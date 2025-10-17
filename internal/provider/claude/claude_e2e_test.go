package claude

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	prov "github.com/cexll/swe/internal/provider"
)

// TestClaudeGenerate_EndToEnd verifies that the Claude provider can execute a live CLI
// call and return structured file changes. Enabled only when RUN_CLAUDE_E2E=true and
// a valid ANTHROPIC_API_KEY is present to avoid accidental live calls in CI.
func TestClaudeGenerate_EndToEnd(t *testing.T) {
	if os.Getenv("RUN_CLAUDE_E2E") != "true" {
		t.Skip("set RUN_CLAUDE_E2E=true to enable live Claude CLI test")
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set; skipping live Claude test")
	}

	if _, err := exec.LookPath("claude"); err != nil {
		t.Skipf("claude CLI not found in PATH: %v", err)
	}

	tmpDir := t.TempDir()

	model := os.Getenv("CLAUDE_MODEL")
	if model == "" {
		model = "claude-sonnet-4-5-20250929"
	}

	provider := NewProvider(apiKey, model)

	req := &prov.CodeRequest{
		Prompt:   "Create a file integration_claude.txt containing the text 'claude e2e success'.",
		RepoPath: tmpDir,
		Context:  map[string]string{"repository": "integration-test"},
	}

	resp, err := provider.GenerateCode(context.Background(), req)
	if err != nil {
		t.Fatalf("GenerateCode() error: %v", err)
	}

	if strings.TrimSpace(resp.Summary) == "" {
		t.Fatal("expected non-empty summary from Claude response")
	}
}
