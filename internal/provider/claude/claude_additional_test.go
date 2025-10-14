package claude

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cexll/swe/internal/provider/shared"
)

func writeExecutable(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
	return path
}

func withPatchedPATH(t *testing.T, dir string) func() {
	t.Helper()
	original := os.Getenv("PATH")
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+original); err != nil {
		t.Fatalf("failed to set PATH: %v", err)
	}
	return func() {
		os.Setenv("PATH", original)
	}
}

func TestGenerateCode_UsesClaudeCLI(t *testing.T) {
	repoDir := t.TempDir()
	// Populate repo with a file so ListRepoFiles sees something
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatalf("failed to write repo file: %v", err)
	}

	cliDir := t.TempDir()
	output := `{"result":"<file path=\"new.go\"><content>package main\n</content></file>\n<summary>done</summary>","isError":false,"costUSD":0.42}`
	script := "#!/bin/sh\ncat >/dev/null\ncat <<'JSON'\n" + output + "\nJSON\n"
	writeExecutable(t, cliDir, "claude", script)
	restorePath := withPatchedPATH(t, cliDir)
	t.Cleanup(restorePath)

	provider := NewProvider("fake", "claude-3")
	resp, err := provider.GenerateCode(context.Background(), &CodeRequest{
		Prompt:   "Add file",
		RepoPath: repoDir,
		Context:  map[string]string{"disallowed_tools": "git"},
	})
	if err != nil {
		t.Fatalf("GenerateCode returned error: %v", err)
	}
	if len(resp.Files) != 1 || resp.Files[0].Path != "new.go" {
		t.Fatalf("unexpected files: %+v", resp.Files)
	}
	if resp.CostUSD != 0.42 {
		t.Fatalf("CostUSD = %v, want 0.42", resp.CostUSD)
	}
	if resp.Summary != "done" {
		t.Fatalf("Summary = %q, want done", resp.Summary)
	}
}

func TestGenerateCode_CLIFailure(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("docs"), 0o644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}

	cliDir := t.TempDir()
	writeExecutable(t, cliDir, "claude", "#!/bin/sh\ncat >/dev/null\necho 'boom' >&2\nexit 1\n")
	restore := withPatchedPATH(t, cliDir)
	t.Cleanup(restore)

	provider := NewProvider("fake", "claude-3")
	_, err := provider.GenerateCode(context.Background(), &CodeRequest{
		Prompt:   "noop",
		RepoPath: repoDir,
	})
	if err == nil || !strings.Contains(err.Error(), "claude CLI execution failed") {
		t.Fatalf("expected CLI failure, got %v", err)
	}
}

func TestGenerateCode_CLIReportedError(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("docs"), 0o644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}

	cliDir := t.TempDir()
	output := `{"result":"Quota exceeded","isError":true,"costUSD":0}`
	script := "#!/bin/sh\ncat >/dev/null\ncat <<'JSON'\n" + output + "\nJSON\n"
	writeExecutable(t, cliDir, "claude", script)
	restore := withPatchedPATH(t, cliDir)
	t.Cleanup(restore)

	provider := NewProvider("fake", "claude-3")
	_, err := provider.GenerateCode(context.Background(), &CodeRequest{
		Prompt:   "noop",
		RepoPath: repoDir,
	})
	if err == nil || !strings.Contains(err.Error(), "Quota exceeded") {
		t.Fatalf("expected CLI error, got %v", err)
	}
}

func TestParseMarkdownCodeBlocksVariants(t *testing.T) {
	response := "```go handlers/login.go\npackage handlers\n```\n\n**docs/setup.md:**\n```md\n# Setup\n```"
	parsed, err := shared.ParseResponse("ClaudeTest", response)
	if err != nil {
		t.Fatalf("ParseResponse() error = %v", err)
	}
	if len(parsed.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(parsed.Files))
	}
	if parsed.Files[0].Path != "handlers/login.go" {
		t.Fatalf("first path = %q", parsed.Files[0].Path)
	}
	if parsed.Files[1].Path != "docs/setup.md" {
		t.Fatalf("second path = %q", parsed.Files[1].Path)
	}
}

func TestTruncateString(t *testing.T) {
	if got := truncateString("short", 10); got != "short" {
		t.Fatalf("truncate short string = %q", got)
	}
	if got := truncateString("very long string", 4); got != "very..." {
		t.Fatalf("truncate long string = %q", got)
	}
}
