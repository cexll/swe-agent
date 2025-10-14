package codex

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

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

	raw, _, err := provider.invokeCodex(ctx, prompt, tmpDir)
	if err != nil {
		t.Fatalf("invokeCodex() error: %v", err)
	}

	if !strings.Contains(raw, `<file path="relative/path/to/file.go">`) {
		t.Fatalf("live output missing placeholder path; raw response:\n%s", raw)
	}

	response, err := parseCodeResponse(raw)
	if err != nil {
		t.Fatalf("parseCodeResponse() error: %v", err)
	}

	var integrationMatches int
	for _, file := range response.Files {
		if strings.Contains(strings.ToLower(file.Path), "relative/path/to/file.go") {
			t.Fatalf("placeholder file path still present in parsed files: %s", file.Path)
		}
		if file.Path == "integration.txt" && strings.Contains(file.Content, "integration success") {
			integrationMatches++
		}
	}

	if integrationMatches == 0 {
		t.Fatalf("missing integration.txt output; files: %+v", response.Files)
	}

	if shared.IsPlaceholderSummary(response.Summary) {
		t.Fatalf("summary should not be placeholder, got %q", response.Summary)
	}
}
