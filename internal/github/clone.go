package github

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var runRepoClone = func(repo, branch, token, dest string) error {
    cmd := exec.Command("gh", "repo", "clone", repo, dest, "--", "-b", branch)
    if token != "" {
        // Set both GITHUB_TOKEN and GH_TOKEN for maximum compatibility with gh CLI
        cmd.Env = append(os.Environ(),
            fmt.Sprintf("GITHUB_TOKEN=%s", token),
            fmt.Sprintf("GH_TOKEN=%s", token),
        )
    }

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gh repo clone failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// Clone clones a GitHub repository to a temporary directory with retry logic.
// Returns: workdir path, cleanup function, error.
func Clone(repo, branch, token string) (string, func(), error) {
	// Create temporary directory with timestamp
	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("pilot-%d", time.Now().Unix()))

	// Execute gh repo clone with retry for transient failures
	// Note: git flags must be passed after '--' separator
	err := retryWithBackoff(func() error {
		return runRepoClone(repo, branch, token, tmpDir)
	})

	if err != nil {
		return "", nil, err
	}

	// Cleanup function to remove temporary directory
	cleanup := func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			fmt.Printf("Warning: failed to cleanup %s: %v\n", tmpDir, err)
		}
	}

	return tmpDir, cleanup, nil
}
