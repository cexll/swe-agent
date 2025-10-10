package github

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Clone clones a GitHub repository to a temporary directory
// Returns: workdir path, cleanup function, error
func Clone(repo, branch string) (string, func(), error) {
	// Create temporary directory with timestamp
	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("pilot-%d", time.Now().Unix()))

	// Execute gh repo clone
	cmd := exec.Command("gh", "repo", "clone", repo, tmpDir, "--branch", branch)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", nil, fmt.Errorf("gh repo clone failed: %w\nOutput: %s", err, string(output))
	}

	// Cleanup function to remove temporary directory
	cleanup := func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			fmt.Printf("Warning: failed to cleanup %s: %v\n", tmpDir, err)
		}
	}

	return tmpDir, cleanup, nil
}
