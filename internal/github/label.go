package github

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// AddLabel adds a label to a GitHub issue or PR using GitHub App authentication with retry logic
// If the label doesn't exist in the repository, it will be created automatically
func AddLabel(repo string, number int, label string, token string) error {
	// First, ensure the label exists in the repository (create if missing)
	err := retryWithBackoff(func() error {
		createCmd := exec.Command("gh", "label", "create", label,
			"--repo", repo,
			"--force",           // Don't fail if label already exists
			"--color", "0366d6", // GitHub blue
			"--description", "Managed by Pilot SWE")

		createCmd.Env = append(os.Environ(), "GITHUB_TOKEN="+token)

		// Ignore errors from label creation (might already exist)
		if output, err := createCmd.CombinedOutput(); err != nil {
			// Log but don't fail - the label might already exist in a different form
			// We'll let the next command (add-label) be the source of truth
			_ = output // Ignore output
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Now add the label to the issue/PR with retry
	return retryWithBackoff(func() error {
		addCmd := exec.Command("gh", "issue", "edit", strconv.Itoa(number),
			"--repo", repo,
			"--add-label", label)

		addCmd.Env = append(os.Environ(), "GITHUB_TOKEN="+token)

		if output, err := addCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("gh issue edit --add-label failed: %w\nOutput: %s", err, string(output))
		}

		return nil
	})
}
