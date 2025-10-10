package github

import (
	"fmt"
	"os/exec"
	"strings"
)

// CreatePR creates a pull request and returns the PR URL
func CreatePR(workdir, repo, head, base, title, body string) (string, error) {
	cmd := exec.Command("gh", "pr", "create",
		"--repo", repo,
		"--head", head,
		"--base", base,
		"--title", title,
		"--body", body)

	cmd.Dir = workdir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr create failed: %w\nOutput: %s", err, string(output))
	}

	// gh pr create returns the PR URL
	prURL := strings.TrimSpace(string(output))
	return prURL, nil
}
