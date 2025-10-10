package github

import (
	"fmt"
	"os/exec"
	"strconv"
)

// CreateComment creates a comment on a GitHub issue or PR
func CreateComment(repo string, number int, body string) error {
	cmd := exec.Command("gh", "issue", "comment", strconv.Itoa(number),
		"--repo", repo,
		"--body", body)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gh issue comment failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}
