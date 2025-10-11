package executor

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/provider"
	"github.com/cexll/swe/internal/provider/claude"
	"github.com/cexll/swe/internal/webhook"
)

// Executor executes pilot tasks
type Executor struct {
	provider provider.Provider
	appAuth  *github.AppAuth
}

// New creates a new executor
func New(p provider.Provider, appAuth *github.AppAuth) *Executor {
	return &Executor{
		provider: p,
		appAuth:  appAuth,
	}
}

// Execute executes a pilot task
func (e *Executor) Execute(ctx context.Context, task *webhook.Task) error {
	log.Printf("Starting task execution for %s#%d", task.Repo, task.Number)

	// 0. Get GitHub App installation token
	log.Printf("Authenticating as GitHub App for %s", task.Repo)
	installToken, err := e.appAuth.GetInstallationToken(task.Repo)
	if err != nil {
		return e.notifyError(task, "", fmt.Sprintf("Failed to authenticate: %v", err))
	}
	log.Printf("Successfully authenticated (token expires at %s)", installToken.ExpiresAt.Format(time.RFC3339))

	// 1. Clone repository
	log.Printf("Cloning repository %s (branch: %s)", task.Repo, task.Branch)
	workdir, cleanup, err := github.Clone(task.Repo, task.Branch)
	if err != nil {
		return e.notifyError(task, installToken.Token, fmt.Sprintf("Failed to clone repository: %v", err))
	}
	defer cleanup()
	log.Printf("Repository cloned to %s", workdir)

	// 2. Call AI provider to generate changes
	log.Printf("Calling %s provider with prompt: %s", e.provider.Name(), task.Prompt)

	// Build context
	context := map[string]string{
		"issue_title": task.IssueTitle,
		"issue_body":  task.IssueBody,
	}

	result, err := e.provider.GenerateCode(ctx, &claude.CodeRequest{
		Prompt:   task.Prompt,
		RepoPath: workdir,
		Context:  context,
	})
	if err != nil {
		return e.notifyError(task, installToken.Token, fmt.Sprintf("%s error: %v", e.provider.Name(), err))
	}
	log.Printf("%s generated %d file changes (cost: $%.4f)", e.provider.Name(), len(result.Files), result.CostUSD)

	// 3. Apply file changes
	log.Printf("Applying file changes")
	if err := e.applyChanges(workdir, result.Files); err != nil {
		return e.notifyError(task, installToken.Token, fmt.Sprintf("Failed to apply changes: %v", err))
	}

	// 4. Create branch and commit changes
	branchName := fmt.Sprintf("pilot/%d-%d", task.Number, time.Now().Unix())
	log.Printf("Creating branch %s and committing changes", branchName)
	if err := e.commitAndPush(workdir, branchName, result.Summary); err != nil {
		return e.notifyError(task, installToken.Token, fmt.Sprintf("Failed to commit/push: %v", err))
	}

	// 5. Create PR (don't actually create it, just return a create PR link)
	log.Printf("Creating PR from %s to %s", branchName, task.Branch)
	prURL, err := e.createPRLink(task.Repo, branchName, task.Branch, result.Summary)
	if err != nil {
		return e.notifyError(task, installToken.Token, fmt.Sprintf("Failed to create PR: %v", err))
	}
	log.Printf("PR link created: %s", prURL)

	// 6. Post success comment
	return e.notifySuccess(task, installToken.Token, result, prURL)
}

// applyChanges writes file changes to disk
func (e *Executor) applyChanges(workdir string, changes []claude.FileChange) error {
	for _, change := range changes {
		filePath := filepath.Join(workdir, change.Path)

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", change.Path, err)
		}

		// Write file
		if err := os.WriteFile(filePath, []byte(change.Content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", change.Path, err)
		}

		log.Printf("Applied changes to %s", change.Path)
	}
	return nil
}

// commitAndPush commits changes and pushes to remote
func (e *Executor) commitAndPush(workdir, branchName, commitMessage string) error {
	commands := [][]string{
		{"git", "config", "user.name", "Pilot Bot"},
		{"git", "config", "user.email", "pilot@github.com"},
		{"git", "checkout", "-b", branchName},
		{"git", "add", "."},
		{"git", "commit", "-m", commitMessage},
		{"git", "push", "-u", "origin", branchName},
	}

	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = workdir
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s failed: %w\nOutput: %s", strings.Join(args, " "), err, string(output))
		}
	}

	return nil
}

// createPRLink generates a GitHub URL for creating a PR
func (e *Executor) createPRLink(repo, head, base, title string) (string, error) {
	// Generate GitHub compare URL that allows user to create PR
	// Format: https://github.com/owner/repo/compare/base...head?expand=1
	prURL := fmt.Sprintf("https://github.com/%s/compare/%s...%s?expand=1&title=%s",
		repo, base, head, strings.ReplaceAll(title, " ", "+"))
	return prURL, nil
}

// notifySuccess posts a success comment to the issue/PR
func (e *Executor) notifySuccess(task *webhook.Task, token string, result *claude.CodeResponse, prURL string) error {
	// Format file list
	fileList := make([]string, len(result.Files))
	for i, file := range result.Files {
		fileList[i] = fmt.Sprintf("- `%s`", file.Path)
	}

	comment := fmt.Sprintf(`### ✅ Task Completed Successfully

**Summary:** %s

**Modified Files:** (%d)
%s

**Next Step:**
[🚀 Click here to create Pull Request](%s)

---
*Generated by Pilot SWE*`, result.Summary, len(result.Files), strings.Join(fileList, "\n"), prURL)

	log.Printf("Posting success comment to %s#%d", task.Repo, task.Number)
	return github.CreateComment(task.Repo, task.Number, comment, token)
}

// notifyError posts an error comment to the issue/PR
func (e *Executor) notifyError(task *webhook.Task, token string, errorMsg string) error {
	comment := fmt.Sprintf(`### ❌ Task Failed

**Error:** %s

Please check the error message and try again.

---
*Generated by Pilot SWE*`, errorMsg)

	log.Printf("Posting error comment to %s#%d: %s", task.Repo, task.Number, errorMsg)
	if err := github.CreateComment(task.Repo, task.Number, comment, token); err != nil {
		log.Printf("Failed to post error comment: %v", err)
		return fmt.Errorf("%s (also failed to post comment: %w)", errorMsg, err)
	}

	return fmt.Errorf("%s", errorMsg)
}
