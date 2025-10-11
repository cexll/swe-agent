package github

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// CreateComment creates a comment on a GitHub issue or PR using GitHub App authentication
func CreateComment(repo string, number int, body string, token string) error {
	cmd := exec.Command("gh", "issue", "comment", strconv.Itoa(number),
		"--repo", repo,
		"--body", body)

	// Set GitHub token as environment variable for gh CLI
	// This makes gh CLI use the GitHub App's identity
	cmd.Env = append(os.Environ(), "GITHUB_TOKEN="+token)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gh issue comment failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}
