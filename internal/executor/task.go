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

	"github.com/cexll/swe/internal/concurrency"
	"github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/provider"
	"github.com/cexll/swe/internal/provider/claude"
	"github.com/cexll/swe/internal/webhook"
)

// Executor executes pilot tasks
type Executor struct {
	provider  provider.Provider
	appAuth   *github.AppAuth
	lockMgr   *concurrency.Manager
}

// New creates a new executor
func New(p provider.Provider, appAuth *github.AppAuth, lockMgr *concurrency.Manager) *Executor {
	return &Executor{
		provider: p,
		appAuth:  appAuth,
		lockMgr:  lockMgr,
	}
}

// Execute executes a pilot task
func (e *Executor) Execute(ctx context.Context, task *webhook.Task) error {
	// Generate lock key: "owner/repo#number"
	lockKey := fmt.Sprintf("%s#%d", task.Repo, task.Number)
	
	log.Printf("Attempting to acquire lock for %s", lockKey)
	
	// Try to acquire lock - fast-fail if already locked
	if !e.lockMgr.TryAcquire(lockKey) {
		errMsg := fmt.Sprintf("Another task is already running for %s. Please wait for it to complete.", lockKey)
		log.Printf("Lock acquisition failed for %s", lockKey)
		
		// Try to get token to post busy notification
		installToken, err := e.appAuth.GetInstallationToken(task.Repo)
		if err != nil {
			// If we can't even get a token, just log and return
			log.Printf("Failed to get token for busy notification: %v", err)
			return fmt.Errorf("%s", errMsg)
		}
		
		return e.notifyBusy(task, installToken.Token)
	}
	
	// Ensure lock is always released
	defer func() {
		e.lockMgr.Release(lockKey)
		log.Printf("Released lock for %s", lockKey)
	}()
	
	log.Printf("Lock acquired for %s, starting task execution", lockKey)
	log.Printf("Starting task execution for %s#%d", task.Repo, task.Number)

	// 0. Get GitHub App installation token
	log.Printf("Authenticating as GitHub App for %s", task.Repo)
	installToken, err := e.appAuth.GetInstallationToken(task.Repo)
	if err != nil {
		return e.notifyError(task, "", fmt.Sprintf("Failed to authenticate: %v", err))
	}
	log.Printf("Successfully authenticated (token expires at %s)", installToken.ExpiresAt.Format(time.RFC3339))

	// Notify user that task has started
	if err := e.notifyStart(task, installToken.Token); err != nil {
		log.Printf("Warning: Failed to post start notification: %v", err)
	}

	// Add "swe" label to the issue for tracking
	if err := github.AddLabel(task.Repo, task.Number, "swe", installToken.Token); err != nil {
		log.Printf("Warning: Failed to add label: %v", err)
	}

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

	log.Printf("%s completed (cost: $%.4f)", e.provider.Name(), result.CostUSD)

	// 3. Apply file changes if provider returned file list
	if len(result.Files) > 0 {
		log.Printf("%s returned %d file changes, applying them", e.provider.Name(), len(result.Files))
		if err := e.applyChanges(workdir, result.Files); err != nil {
			return e.notifyError(task, installToken.Token, fmt.Sprintf("Failed to apply changes: %v", err))
		}
	} else {
		log.Printf("%s did not return file list, checking git status for direct modifications", e.provider.Name())
	}

	// 3.5. Detect actual file changes using git (works for both direct modifications and applied changes)
	hasChanges, err := e.detectGitChanges(workdir)
	if err != nil {
		return e.notifyError(task, installToken.Token, fmt.Sprintf("Failed to detect changes: %v", err))
	}

	if !hasChanges {
		// No actual file changes detected, just post the AI's response as a comment
		log.Printf("No file changes detected in working directory (analysis/answer only)")
		return e.notifyResponse(task, installToken.Token, result)
	}

	log.Printf("File changes detected in working directory, proceeding with commit")

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

// detectGitChanges checks if there are any uncommitted changes in the working directory
func (e *Executor) detectGitChanges(workdir string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git status failed: %w\nOutput: %s", err, string(output))
	}

	// If output is empty, no changes detected
	hasChanges := len(strings.TrimSpace(string(output))) > 0
	return hasChanges, nil
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

// notifyResponse posts an AI response without file changes (analysis/answer/recommendations)
func (e *Executor) notifyResponse(task *webhook.Task, token string, result *claude.CodeResponse) error {
	comment := fmt.Sprintf(`### 💬 AI Response

%s

---
*Generated by Pilot SWE • Cost: $%.4f*`, result.Summary, result.CostUSD)

	log.Printf("Posting AI response to %s#%d", task.Repo, task.Number)
	return github.CreateComment(task.Repo, task.Number, comment, token)
}

// notifyStart posts a comment indicating the task has started
func (e *Executor) notifyStart(task *webhook.Task, token string) error {
	comment := `### 🤖 Task Started

Pilot is now processing your request...

---
*Generated by Pilot SWE*`

	log.Printf("Posting start notification to %s#%d", task.Repo, task.Number)
	return github.CreateComment(task.Repo, task.Number, comment, token)
}

// notifyBusy posts a comment indicating the task is busy
func (e *Executor) notifyBusy(task *webhook.Task, token string) error {
	comment := `### ⏳ Task Queue Busy

Another task is currently running for this issue/PR. Please wait for it to complete before issuing a new command.

---
*Generated by Pilot SWE*`

	log.Printf("Posting busy notification to %s#%d", task.Repo, task.Number)
	if err := github.CreateComment(task.Repo, task.Number, comment, token); err != nil {
		log.Printf("Failed to post busy comment: %v", err)
		return fmt.Errorf("task is busy and failed to post notification: %w", err)
	}
	
	return fmt.Errorf("task is busy: another command is already running for %s#%d", task.Repo, task.Number)
}