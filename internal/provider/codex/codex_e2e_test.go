package codex

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	prov "github.com/cexll/swe/internal/provider"
	"github.com/cexll/swe/internal/provider/shared"
)

// TestCodexInvoke_EndToEndIgnoresPlaceholders performs a live Codex CLI call to ensure
// placeholder file paths/content no longer trigger parsing failures.
// Enable by setting RUN_CODEX_E2E=true (skipped by default to keep CI fast/offline).
func TestCodexInvoke_EndToEndIgnoresPlaceholders(t *testing.T) {
	if os.Getenv("RUN_CODEX_E2E") != "true" {
		t.Skip("set RUN_CODEX_E2E=true to enable live Codex CLI test")
	}

	if _, err := exec.LookPath("codex"); err != nil {
		t.Skipf("codex CLI not found in PATH: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	provider := NewProvider(os.Getenv("OPENAI_API_KEY"), os.Getenv("OPENAI_BASE_URL"), "gpt-5-codex")

	tmpDir := t.TempDir()

	initCmd := exec.Command("git", "init")
	initCmd.Dir = tmpDir
	if output, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, string(output))
	}

	prompt := `Task: Output EXACTLY the following response without additional text. Do NOT explain anything. Just return the XML snippet verbatim.

<file path="relative/path/to/file.go">
<content>
... full file content here ...
</content>
</file>

<file path="integration.txt">
<content>
integration success
</content>
</file>

<summary>
Integration success
</summary>
`

	req := &prov.CodeRequest{Prompt: prompt, RepoPath: tmpDir, Context: map[string]string{}}
	resp, err := provider.GenerateCode(ctx, req)
	if err != nil {
		t.Fatalf("GenerateCode() error: %v", err)
	}

	if strings.TrimSpace(resp.Summary) == "" {
		t.Fatalf("expected non-empty summary from Codex")
	}

	if shared.IsPlaceholderSummary(resp.Summary) {
		t.Fatalf("summary should not be placeholder, got %q", resp.Summary)
	}
}
