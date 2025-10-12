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

// CloneFunc is a function type for cloning repositories
type CloneFunc func(repo, branch string) (workdir string, cleanup func(), err error)

// Executor executes pilot tasks
type Executor struct {
	provider provider.Provider
	appAuth  github.AuthProvider
	ghClient github.GHClient
	cloneFn  CloneFunc
}

// New creates a new executor
func New(p provider.Provider, appAuth github.AuthProvider) *Executor {
	return &Executor{
		provider: p,
		appAuth:  appAuth,
		ghClient: github.NewRealGHClient(),
		cloneFn:  github.Clone,
	}
}

// NewWithClient creates a new executor with a custom gh client (useful for testing)
func NewWithClient(p provider.Provider, appAuth github.AuthProvider, ghClient github.GHClient) *Executor {
	return &Executor{
		provider: p,
		appAuth:  appAuth,
		ghClient: ghClient,
		cloneFn:  github.Clone,
	}
}

// Execute executes a pilot task
func (e *Executor) Execute(ctx context.Context, task *webhook.Task) error {
	log.Printf("Starting task execution for %s#%d", task.Repo, task.Number)

	// 0. Get GitHub App installation token
	log.Printf("Authenticating as GitHub App for %s", task.Repo)
	installToken, err := e.appAuth.GetInstallationToken(task.Repo)
	if err != nil {
		return fmt.Errorf("failed to authenticate: %v", err)
	}
	log.Printf("Successfully authenticated (token expires at %s)", installToken.ExpiresAt.Format(time.RFC3339))

	// 1. Create tracking comment
	tracker := github.NewCommentTrackerWithClient(task.Repo, task.Number, task.Username, e.ghClient)
	tracker.State.StartTime = time.Now()
	tracker.State.OriginalBody = task.Prompt

	if err := tracker.Create(installToken.Token); err != nil {
		log.Printf("Warning: Failed to create tracking comment: %v", err)
		// Continue execution even if comment creation fails
	} else {
		log.Printf("Created tracking comment (ID: %d)", tracker.CommentID)
	}

	// Add "swe" label to the issue for tracking
	if err := e.ghClient.AddLabel(task.Repo, task.Number, "swe", installToken.Token); err != nil {
		log.Printf("Warning: Failed to add label: %v", err)
	}

	// 2. Clone repository
	log.Printf("Cloning repository %s (branch: %s)", task.Repo, task.Branch)
	workdir, cleanup, err := e.cloneFn(task.Repo, task.Branch)
	if err != nil {
		return e.handleError(tracker, installToken.Token, fmt.Sprintf("Failed to clone repository: %v", err))
	}
	defer cleanup()
	log.Printf("Repository cloned to %s", workdir)

	// 3. Call AI provider to generate changes
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
		return e.handleError(tracker, installToken.Token, fmt.Sprintf("%s error: %v", e.provider.Name(), err))
	}

	log.Printf("%s completed (cost: $%.4f)", e.provider.Name(), result.CostUSD)

	// 4. Apply file changes if provider returned file list
	if len(result.Files) > 0 {
		log.Printf("%s returned %d file changes, applying them", e.provider.Name(), len(result.Files))
		if err := e.applyChanges(workdir, result.Files); err != nil {
			return e.handleError(tracker, installToken.Token, fmt.Sprintf("Failed to apply changes: %v", err))
		}
	} else {
		log.Printf("%s did not return file list, checking git status for direct modifications", e.provider.Name())
	}

	// 5. Detect actual file changes using git
	hasChanges, err := e.detectGitChanges(workdir)
	if err != nil {
		return e.handleError(tracker, installToken.Token, fmt.Sprintf("Failed to detect changes: %v", err))
	}

	if !hasChanges {
		// No actual file changes detected, just post the AI's response
		log.Printf("No file changes detected in working directory (analysis/answer only)")
		return e.handleResponseOnly(tracker, installToken.Token, result)
	}

	log.Printf("File changes detected in working directory, proceeding with commit")

	// 5.5. Get changed files from git status
	changedFiles, err := e.getChangedFiles(workdir)
	if err != nil {
		return e.handleError(tracker, installToken.Token, fmt.Sprintf("Failed to get changed files: %v", err))
	}
	log.Printf("Detected %d changed files", len(changedFiles))

	// 5.6. Analyze and decide if splitting is needed
	splitter := github.NewPRSplitter(8, 300)
	plan := splitter.Analyze(changedFiles, task.Prompt)

	log.Printf("Split analysis: %d sub-PRs planned", len(plan.SubPRs))

	// 5.7. Execute based on split plan
	if len(plan.SubPRs) > 1 {
		// Multiple PRs needed - use split workflow
		log.Printf("Using multi-PR workflow")
		return e.executeMultiPR(ctx, task, workdir, plan, result, tracker, installToken.Token)
	}

	// 5.8. Single PR workflow (original logic)
	log.Printf("Using single-PR workflow")

	// 6. Create branch and commit changes
	branchName := fmt.Sprintf("pilot/%d-%d", task.Number, time.Now().Unix())
	log.Printf("Creating branch %s and committing changes", branchName)
	if err := e.commitAndPush(workdir, branchName, result.Summary); err != nil {
		return e.handleError(tracker, installToken.Token, fmt.Sprintf("Failed to commit/push: %v", err))
	}

	// 7. Create PR link
	log.Printf("Creating PR from %s to %s", branchName, task.Branch)
	prURL, err := e.createPRLink(task.Repo, branchName, task.Branch, result.Summary)
	if err != nil {
		return e.handleError(tracker, installToken.Token, fmt.Sprintf("Failed to create PR: %v", err))
	}
	log.Printf("PR link created: %s", prURL)

	// 8. Build branch URL
	branchURL := fmt.Sprintf("https://github.com/%s/tree/%s", task.Repo, branchName)

	// 9. Update tracking comment with success
	tracker.MarkEnd()
	tracker.SetCompleted(result.Summary, e.extractFilePaths(result.Files), result.CostUSD)
	tracker.SetBranch(branchName, branchURL)
	tracker.SetPRURL(prURL)

	if err := tracker.Update(installToken.Token); err != nil {
		log.Printf("Warning: Failed to update tracking comment: %v", err)
	}

	log.Printf("Task completed successfully")
	return nil
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

// handleError updates the tracking comment with error details and returns the error
func (e *Executor) handleError(tracker *github.CommentTracker, token, errorMsg string) error {
	tracker.MarkEnd()
	tracker.SetFailed(errorMsg)

	if err := tracker.Update(token); err != nil {
		log.Printf("Warning: Failed to update tracking comment with error: %v", err)
	}

	return fmt.Errorf("%s", errorMsg)
}

// handleResponseOnly updates the tracking comment with AI response (no code changes)
func (e *Executor) handleResponseOnly(tracker *github.CommentTracker, token string, result *claude.CodeResponse) error {
	tracker.MarkEnd()
	tracker.SetCompleted(result.Summary, nil, result.CostUSD)

	if err := tracker.Update(token); err != nil {
		log.Printf("Warning: Failed to update tracking comment: %v", err)
	}

	log.Printf("Task completed (response only, no code changes)")
	return nil
}

// extractFilePaths extracts file paths from FileChange array
func (e *Executor) extractFilePaths(files []claude.FileChange) []string {
	paths := make([]string, len(files))
	for i, file := range files {
		paths[i] = file.Path
	}
	return paths
}

// getChangedFiles gets list of changed files from git status
func (e *Executor) getChangedFiles(workdir string) ([]claude.FileChange, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w", err)
	}

	var changes []claude.FileChange
	// Don't TrimSpace the whole output - it will corrupt the first line!
	// Just split by newline and handle empty lines
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		// Skip empty lines
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}

		// git status --porcelain format: "XY filename"
		// We just need the filename (skip first 3 characters: status + space)
		if len(line) < 4 {
			continue
		}

		filePath := strings.TrimSpace(line[3:])

		// Skip directories (git shows untracked directories with trailing slash)
		if strings.HasSuffix(filePath, "/") {
			// This is a directory, need to list files recursively
			dirPath := filepath.Join(workdir, filePath)
			err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}

				relPath, err := filepath.Rel(workdir, path)
				if err != nil {
					return err
				}

				content, err := os.ReadFile(path)
				if err != nil {
					log.Printf("Warning: Could not read %s: %v", relPath, err)
					return nil
				}

				changes = append(changes, claude.FileChange{
					Path:    relPath,
					Content: string(content),
				})
				return nil
			})
			if err != nil {
				log.Printf("Warning: Could not walk directory %s: %v", filePath, err)
			}
			continue
		}

		// Read file content
		fullPath := filepath.Join(workdir, filePath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			// File might be deleted, skip
			log.Printf("Warning: Could not read %s: %v", filePath, err)
			continue
		}

		changes = append(changes, claude.FileChange{
			Path:    filePath,
			Content: string(content),
		})
	}

	return changes, nil
}

// executeMultiPR executes multi-PR workflow
func (e *Executor) executeMultiPR(
	ctx context.Context,
	task *webhook.Task,
	workdir string,
	plan *github.SplitPlan,
	result *claude.CodeResponse,
	tracker *github.CommentTracker,
	token string,
) error {
	log.Printf("Executing multi-PR workflow with %d sub-PRs", len(plan.SubPRs))

	// Store AI-generated summary in tracker state (will be displayed in split plan section)
	tracker.State.Summary = result.Summary

	// Update tracker to show split plan
	tracker.SetSplitPlan(plan)
	if err := tracker.Update(token); err != nil {
		log.Printf("Warning: Failed to update comment with split plan: %v", err)
	}

	createdPRs := []github.CreatedPR{}

	// Create PRs in order (independent ones first)
	for _, idx := range plan.CreationOrder {
		subPR := plan.SubPRs[idx]

		log.Printf("Creating sub-PR #%d: %s (%d files)", idx, subPR.Name, len(subPR.Files))

		// Check if dependencies are satisfied
		// For now, we only create independent PRs (no dependencies)
		if len(subPR.DependsOn) > 0 {
			log.Printf("Sub-PR #%d has dependencies, skipping for now", idx)
			createdPR := github.CreatedPR{
				Index:    idx,
				Name:     subPR.Name,
				Status:   "pending",
				Category: subPR.Category,
			}
			createdPRs = append(createdPRs, createdPR)
			tracker.AddCreatedPR(createdPR)
			continue
		}

		// Create branch for this sub-PR
		branchName := fmt.Sprintf("pilot/%d-%s-%d", task.Number, subPR.Category, time.Now().Unix())

		// Commit only files from this sub-PR
		if err := e.commitSubPR(workdir, branchName, subPR); err != nil {
			log.Printf("Warning: Failed to create sub-PR #%d: %v", idx, err)
			// Continue with other PRs
			continue
		}

		// Generate PR URL
		prURL, _ := e.createPRLink(task.Repo, branchName, task.Branch, subPR.Name)
		branchURL := fmt.Sprintf("https://github.com/%s/tree/%s", task.Repo, branchName)

		// Record created PR
		createdPR := github.CreatedPR{
			Index:      idx,
			Name:       subPR.Name,
			BranchName: branchName,
			URL:        prURL,
			BranchURL:  branchURL,
			Status:     "created",
			Category:   subPR.Category,
		}
		createdPRs = append(createdPRs, createdPR)
		tracker.AddCreatedPR(createdPR)

		// Update comment with progress
		if err := tracker.Update(token); err != nil {
			log.Printf("Warning: Failed to update comment: %v", err)
		}

		log.Printf("Created sub-PR #%d: %s", idx, prURL)
	}

	// Mark task as completed
	tracker.MarkEnd()
	tracker.SetCompletedWithSplit(plan, createdPRs, result.CostUSD)

	if err := tracker.Update(token); err != nil {
		log.Printf("Warning: Failed to update final comment: %v", err)
	}

	log.Printf("Multi-PR workflow completed: %d PRs created", len(createdPRs))
	return nil
}

// commitSubPR commits only the files from a specific sub-PR
func (e *Executor) commitSubPR(workdir, branchName string, subPR github.SubPR) error {
	// Reset to base branch first
	resetCmd := exec.Command("git", "reset", "--hard", "HEAD")
	resetCmd.Dir = workdir
	if output, err := resetCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset failed: %w\nOutput: %s", err, string(output))
	}

	// Clean untracked files
	cleanCmd := exec.Command("git", "clean", "-fd")
	cleanCmd.Dir = workdir
	if output, err := cleanCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clean failed: %w\nOutput: %s", err, string(output))
	}

	// Reapply only files from this sub-PR
	for _, file := range subPR.Files {
		filePath := filepath.Join(workdir, file.Path)

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", file.Path, err)
		}

		// Write file
		if err := os.WriteFile(filePath, []byte(file.Content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", file.Path, err)
		}
	}

	// Create branch and commit
	commands := [][]string{
		{"git", "checkout", "-b", branchName},
		{"git", "add", "."},
		{"git", "commit", "-m", subPR.Name + "\n\n" + subPR.Description},
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
