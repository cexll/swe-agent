package github

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var runRepoClone = func(repo, branch, token, dest string) error {
	// Pass through to underlying git clone with shallow/single-branch options for stability/perf
	cmd := exec.Command("gh", "repo", "clone", repo, dest, "--", "-b", branch, "--depth=1", "--single-branch")
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

var (
	nowFunc            = time.Now
	issueNumberPattern = regexp.MustCompile(`(?i)issue[-_/](\d+)`)
	nonAlphanumeric    = regexp.MustCompile(`[^a-z0-9]+`)
)

func sanitizeToken(token string) string {
	token = strings.ToLower(token)
	token = nonAlphanumeric.ReplaceAllString(token, "-")
	token = strings.Trim(token, "-")
	if token == "" {
		return "unknown"
	}
	return token
}

func extractBranchContext(branch string) (string, string) {
	if match := issueNumberPattern.FindStringSubmatch(branch); len(match) == 2 {
		return "issue", match[1]
	}

	sanitized := sanitizeToken(branch)
	if sanitized == "" {
		return "branch", "unknown"
	}
	return "branch", sanitized
}

func buildCloneWorkdir(repo, branch string, ts time.Time) string {
	ownerSegment := "unknown"
	repoSegment := "repo"

	if parts := strings.Split(repo, "/"); len(parts) == 2 {
		ownerSegment = sanitizeToken(parts[0])
		repoSegment = sanitizeToken(parts[1])
	} else {
		ownerSegment = sanitizeToken(repo)
	}

	context, detail := extractBranchContext(branch)

	dirName := fmt.Sprintf("%s-%s-%s-%s-%d", ownerSegment, repoSegment, context, detail, ts.UnixNano())
	return filepath.Join(os.TempDir(), dirName)
}

// Clone clones a GitHub repository to a temporary directory with retry logic.
// Returns: workdir path, cleanup function, error.
func Clone(repo, branch, token string) (string, func(), error) {
	// Create temporary directory name that avoids collisions across concurrent clones.
	tmpDir := buildCloneWorkdir(repo, branch, nowFunc())

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
