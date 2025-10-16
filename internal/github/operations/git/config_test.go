package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// createGitShim creates a temporary directory with a fake `git` executable
// that records config writes and serves --get lookups from a state file.
func createGitShim(t *testing.T) (shimDir string) {
	t.Helper()
	dir := t.TempDir()
	state := filepath.Join(dir, "git_state")

	// POSIX shell script only (Unix-like test environments)
	script := "#!/usr/bin/env bash\nset -euo pipefail\nSTATE=\"" + state + "\"\nif [[ \"${1:-}\" == config ]]; then\n  shift\n  if [[ \"${1:-}\" == --global ]]; then shift; fi\n  if [[ \"${1:-}\" == --get ]]; then\n    key=\"${2:-}\"\n    if [[ -f \"$STATE\" ]]; then\n      val=$(grep -E \"^${key}=\" \"$STATE\" | tail -n1 | sed -E 's/^([^=]+)=//') || true\n      if [[ -n \"$val\" ]]; then echo -n \"$val\"; exit 0; fi\n    fi\n    exit 1\n  else\n    key=\"${1:-}\"\n    val=\"${2:-}\"\n    echo \"${key}=${val}\" >> \"$STATE\"\n    exit 0\n  fi\nfi\nexit 0\n"

	shim := filepath.Join(dir, "git")
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write git shim: %v", err)
	}
	_ = os.Chmod(shim, 0o755)
    return dir
}

func prependPath(t *testing.T, dir string) func() {
	t.Helper()
	old := os.Getenv("PATH")
	sep := string(os.PathListSeparator)
	if err := os.Setenv("PATH", dir+sep+old); err != nil {
		t.Fatalf("failed setting PATH: %v", err)
	}
	return func() { _ = os.Setenv("PATH", old) }
}

func TestConfigureGitAndGet(t *testing.T) {
    shimDir := createGitShim(t)
	restore := prependPath(t, shimDir)
	defer restore()

	if err := ConfigureGit("bot-name", "bot@example.com"); err != nil {
		t.Fatalf("ConfigureGit error: %v", err)
	}

	// Verify the values can be read back via GetGitConfig using the shim
	if v, err := GetGitConfig("user.name"); err != nil || strings.TrimSpace(v) != "bot-name" {
		t.Fatalf("GetGitConfig user.name = %q, err=%v", v, err)
	}
	if v, err := GetGitConfig("user.email"); err != nil || strings.TrimSpace(v) != "bot@example.com" {
		t.Fatalf("GetGitConfig user.email = %q, err=%v", v, err)
	}
}

func TestConfigureGitForApp(t *testing.T) {
    shimDir := createGitShim(t)
	restore := prependPath(t, shimDir)
	defer restore()

	if err := ConfigureGitForApp(12345, "swe-agent"); err != nil {
		t.Fatalf("ConfigureGitForApp error: %v", err)
	}
	name, _ := GetGitConfig("user.name")
	email, _ := GetGitConfig("user.email")
	if !strings.Contains(name, "swe-agent[bot]") {
		t.Fatalf("user.name = %q, want contains swe-agent[bot]", name)
	}
	if !strings.Contains(email, "12345+swe-agent[bot]@users.noreply.github.com") {
		t.Fatalf("user.email = %q, want app pattern", email)
	}
}

func TestConfigureGitForApp_DefaultName(t *testing.T) {
    shimDir := createGitShim(t)
	restore := prependPath(t, shimDir)
	defer restore()

	if err := ConfigureGitForApp(777, ""); err != nil {
		t.Fatalf("ConfigureGitForApp (default) error: %v", err)
	}
	name, _ := GetGitConfig("user.name")
	email, _ := GetGitConfig("user.email")
	if !strings.Contains(name, "swe-agent[bot]") || !strings.Contains(email, "777+swe-agent[bot]@users.noreply.github.com") {
		t.Fatalf("unexpected defaults: %q / %q", name, email)
	}
}
