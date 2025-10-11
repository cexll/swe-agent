package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cexll/swe/internal/provider/claude"
)

func TestNewProvider_Name(t *testing.T) {
	provider := NewProvider("", "gpt-5-codex")
	if provider.Name() != "codex" {
		t.Fatalf("Name() = %s, want codex", provider.Name())
	}
}

func TestProvider_GenerateCode(t *testing.T) {
	t.Helper()

	originalExec := execCommandContext
	defer func() {
		execCommandContext = originalExec
	}()

	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmdArgs := []string{"-test.run=TestCodexHelperProcess", "--", name}
		cmdArgs = append(cmdArgs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cmdArgs...)
		cmd.Env = append(os.Environ(), "GO_WANT_CODEX_HELPER=1")
		return cmd
	}

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc main(){}\n"), 0o644); err != nil {
		t.Fatalf("failed to seed repo: %v", err)
	}

	provider := NewProvider("", "test-model")

	req := &claude.CodeRequest{
		Prompt:   "Implement feature X",
		RepoPath: tmpDir,
		Context: map[string]string{
			"branch": "main",
		},
	}

	resp, err := provider.GenerateCode(context.Background(), req)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}

	if resp == nil {
		t.Fatalf("GenerateCode() returned nil response")
	}

	if len(resp.Files) != 1 {
		t.Fatalf("GenerateCode() files count = %d, want 1", len(resp.Files))
	}

	if resp.Files[0].Path != "output.txt" {
		t.Errorf("File path = %s, want output.txt", resp.Files[0].Path)
	}

	if resp.Summary != "Did something useful" {
		t.Errorf("Summary = %s, want Did something useful", resp.Summary)
	}

	if resp.CostUSD != 1.23 {
		t.Errorf("CostUSD = %.2f, want 1.23", resp.CostUSD)
	}
}

func TestCodexHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_CODEX_HELPER") != "1" {
		return
	}

	defer os.Exit(0)

	args := os.Args
	idx := -1
	for i, arg := range args {
		if arg == "--" {
			idx = i
			break
		}
	}
	if idx == -1 || idx+1 >= len(args) || args[idx+1] != codexCommand {
		fmt.Fprintf(os.Stderr, "unexpected command: %v", args)
		os.Exit(1)
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read stdin error: %v", err)
		os.Exit(1)
	}

	var payload codexRequest
	if err := json.Unmarshal(input, &payload); err != nil {
		fmt.Fprintf(os.Stderr, "unmarshal request error: %v", err)
		os.Exit(1)
	}

	if payload.Model != "test-model" {
		fmt.Fprintf(os.Stderr, "unexpected model: %s", payload.Model)
		os.Exit(1)
	}

	if payload.Sandbox != defaultSandbox {
		fmt.Fprintf(os.Stderr, "unexpected sandbox: %s", payload.Sandbox)
		os.Exit(1)
	}

	if payload.ApprovalPolicy != defaultPolicy {
		fmt.Fprintf(os.Stderr, "unexpected approval policy: %s", payload.ApprovalPolicy)
		os.Exit(1)
	}

	if payload.CWD == "" {
		fmt.Fprint(os.Stderr, "missing cwd")
		os.Exit(1)
	}

	if payload.Prompt == "" {
		fmt.Fprint(os.Stderr, "missing prompt")
		os.Exit(1)
	}

	response := codexResponse{
		Result: `<file path="output.txt">
<content>
hello world