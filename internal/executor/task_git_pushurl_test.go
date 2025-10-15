package executor

import (
	"os/exec"
	"testing"
)

// Test that when origin is a GitHub SSH remote, resolvePushURL returns a
// tokenized HTTPS pushurl for the given owner/repo.
func TestResolvePushURL_GitHubSSHRemoteGeneratesTokenizedHTTPS(t *testing.T) {
	workdir := t.TempDir()
	token := "token-xyz"
	repo := "owner/repo"

	restore := StubExecCommandForTest(func(name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) == 3 && args[0] == "remote" && args[1] == "get-url" && args[2] == "origin" {
			return exec.Command("bash", "-lc", "printf 'git@github.com:owner/repo.git'")
		}
		// No other git commands are expected in this test path
		return exec.Command("bash", "-lc", "printf ''")
	})
	defer restore()

	got, err := resolvePushURL(workdir, repo, token)
	if err != nil {
		t.Fatalf("resolvePushURL error = %v", err)
	}

	want := "https://x-access-token:" + token + "@github.com/" + repo
	if got != want {
		t.Fatalf("resolvePushURL = %q, want %q", got, want)
	}
}
