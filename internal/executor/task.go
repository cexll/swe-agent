package executor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/provider"
	"github.com/cexll/swe/internal/provider/claude"
	"github.com/cexll/swe/internal/taskstore"
	"github.com/cexll/swe/internal/webhook"
)

var execCommand = exec.Command

// StubExecCommandForTest replaces the command runner for the duration of a test.
// The returned function must be deferred to restore the default behaviour.
func StubExecCommandForTest(handler func(name string, args ...string) *exec.Cmd) func() {
	original := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return handler(name, args...)
	}
	return func() {
		execCommand = original
	}
}

// CloneFunc is a function type for cloning repositories
type CloneFunc func(repo, branch, token string) (workdir string, cleanup func(), err error)

// Executor executes pilot tasks
type Executor struct {
	provider        provider.Provider
	appAuth         github.AuthProvider
	ghClient        github.GHClient
	cloneFn         CloneFunc
	store           *taskstore.Store
	disallowedTools string // Tools that are not allowed to be used
}

// New creates a new executor
func New(p provider.Provider, appAuth github.AuthProvider) *Executor {
	return &Executor{
		provider:        p,
		appAuth:         appAuth,
		ghClient:        github.NewRealGHClient(),
		cloneFn:         github.Clone,
		disallowedTools: "", // Default: no restrictions
	}
}

// WithDisallowedTools sets the disallowed tools
func (e *Executor) WithDisallowedTools(tools string) *Executor {
	e.disallowedTools = tools
	return e
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

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// WithStore attaches a task store to the executor for tracking execution state
func (e *Executor) WithStore(store *taskstore.Store) *Executor {
	e.store = store
	return e
}

// WithCloneFunc allows tests to override the repository clone implementation.
// Passing nil restores the default GitHub-based clone.
func (e *Executor) WithCloneFunc(fn CloneFunc) *Executor {
	if fn == nil {
		e.cloneFn = github.Clone
		return e
	}
	e.cloneFn = fn
	return e
}

func (e *Executor) ensureAttempt(task *webhook.Task) int {
	attempt := task.Attempt
	if attempt <= 0 {
		attempt = 1
	}
	task.Attempt = attempt
	return attempt
}

func (e *Executor) buildExecutionContext(task *webhook.Task) map[string]string {
	contextMap := cloneStringMap(task.PromptContext)
	if contextMap == nil {
		contextMap = make(map[string]string)
	}
	if task.IssueTitle != "" {
		contextMap["issue_title"] = task.IssueTitle
	}
	if task.IssueBody != "" {
		contextMap["issue_body"] = task.IssueBody
	}
	if _, ok := contextMap["repository"]; !ok && task.Repo != "" {
		contextMap["repository"] = task.Repo
	}
	if _, ok := contextMap["base_branch"]; !ok && task.Branch != "" {
		contextMap["base_branch"] = task.Branch
	}
	if task.IsPR {
		contextMap["is_pr"] = "true"
		contextMap["pr_number"] = strconv.Itoa(task.Number)
	} else {
		contextMap["is_pr"] = "false"
		contextMap["issue_number"] = strconv.Itoa(task.Number)
	}
	if task.Username != "" {
		if _, ok := contextMap["trigger_username"]; !ok {
			contextMap["trigger_username"] = task.Username
		}
		if _, ok := contextMap["trigger_display_name"]; !ok {
			contextMap["trigger_display_name"] = task.Username
		}
	}
	if e.disallowedTools != "" {
		contextMap["disallowed_tools"] = e.disallowedTools
	}
	return contextMap
}

func (e *Executor) authenticateWithGitHub(task *webhook.Task) (*github.InstallationToken, error) {
	log.Printf("Authenticating as GitHub App for %s", task.Repo)
	e.addLog(task, "info", "Authenticating as GitHub App for %s", task.Repo)

	installToken, err := e.appAuth.GetInstallationToken(task.Repo)
	if err != nil {
		e.addLog(task, "error", "Failed to authenticate: %v", err)
		return nil, fmt.Errorf("failed to authenticate: %v", err)
	}

	log.Printf("Successfully authenticated (token expires at %s)", installToken.ExpiresAt.Format(time.RFC3339))
	e.addLog(task, "info", "Authenticated as GitHub App (expires %s)", installToken.ExpiresAt.Format(time.RFC3339))
	return installToken, nil
}

func (e *Executor) enrichPromptWithDiscussion(task *webhook.Task, token string) {
	if discussion := e.composeDiscussionSection(task, token); discussion != "" {
		task.Prompt = injectDiscussion(task.Prompt, discussion)
	}
}

func (e *Executor) initializeTracker(task *webhook.Task, contextMap map[string]string, token string) *github.CommentTracker {
	tracker := github.NewCommentTrackerWithClient(task.Repo, task.Number, task.Username, e.ghClient)
	tracker.SetQueued()
	if task.PromptSummary != "" {
		tracker.State.OriginalBody = task.PromptSummary
	} else {
		tracker.State.OriginalBody = task.Prompt
	}
	tracker.State.Context = cloneStringMap(contextMap)

	tracker.AddTask("Authenticate with GitHub")
	tracker.CompleteTask("Authenticate with GitHub")
	tracker.AddTask("Clone repository")
	tracker.AddTask("Generate code changes")
	tracker.AddTask("Commit and push changes")
	if !task.IsPR || task.PRState != "open" {
		tracker.AddTask("Create pull request")
	}

	if existingID := extractTrackerID(contextMap["claude_comment_id"]); existingID > 0 {
		tracker.CommentID = existingID
		if err := tracker.Update(token); err != nil {
			log.Printf("Warning: Failed to update existing tracking comment: %v", err)
			e.addLog(task, "error", "Failed to update tracking comment: %v", err)
		}
	} else {
		if err := tracker.Create(token); err != nil {
			log.Printf("Warning: Failed to create tracking comment: %v", err)
			e.addLog(task, "error", "Failed to create tracking comment: %v", err)
		} else {
			log.Printf("Created tracking comment (ID: %d)", tracker.CommentID)
			e.addLog(task, "info", "Created tracking comment (ID: %d)", tracker.CommentID)
		}
	}

	tracker.State.StartTime = time.Now()
	if tracker.CommentID > 0 {
		contextMap["claude_comment_id"] = strconv.Itoa(tracker.CommentID)
		tracker.State.Context = cloneStringMap(contextMap)
		tracker.SetWorking()
		if err := tracker.Update(token); err != nil {
			log.Printf("Warning: Failed to update tracking comment to working status: %v", err)
			e.addLog(task, "error", "Failed to set tracking comment to working: %v", err)
		}
	}

	return tracker
}

func extractTrackerID(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return -1
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return -1
	}
	return id
}

func (e *Executor) ensureTrackingLabel(task *webhook.Task, tracker *github.CommentTracker, token string) {
	if err := e.ghClient.AddLabel(task.Repo, task.Number, "swe", token); err != nil {
		log.Printf("Warning: Failed to add label: %v", err)
		e.addLog(task, "error", "Failed to add swe label: %v", err)
	}
}

func (e *Executor) cloneAndPrepareWorkspace(
	task *webhook.Task,
	tracker *github.CommentTracker,
	token string,
	contextMap map[string]string,
) (workdir string, cleanup func(), branchName string, isNewBranch bool, err error) {
	tracker.StartTask("Clone repository")
	if err := tracker.Update(token); err != nil {
		log.Printf("Warning: Failed to update progress: %v", err)
	}

	log.Printf("Cloning repository %s (branch: %s)", task.Repo, task.Branch)
	e.addLog(task, "info", "Cloning repository %s (branch %s)", task.Repo, task.Branch)

	workdir, cleanup, err = e.cloneFn(task.Repo, task.Branch, token)
	if err != nil {
		tracker.FailTask("Clone repository")
		return "", nil, "", false, e.handleError(task, tracker, token, fmt.Sprintf("Failed to clone repository: %v", err))
	}

	branchName, isNewBranch, err = e.prepareBranch(workdir, task)
	if err != nil {
		tracker.FailTask("Clone repository")
		cleanup()
		return "", nil, "", false, e.handleError(task, tracker, token, fmt.Sprintf("Failed to prepare branch: %v", err))
	}

	contextMap["claude_branch"] = branchName
	tracker.State.Context = cloneStringMap(contextMap)

	tracker.CompleteTask("Clone repository")
	log.Printf("Repository cloned to %s", workdir)
	e.addLog(task, "info", "Repository cloned to %s", workdir)

	return workdir, cleanup, branchName, isNewBranch, nil
}

func (e *Executor) generateCodeChanges(
	ctx context.Context,
	task *webhook.Task,
	workdir string,
	contextMap map[string]string,
	tracker *github.CommentTracker,
	token string,
) (*claude.CodeResponse, error) {
	tracker.StartTask("Generate code changes")
	if err := tracker.Update(token); err != nil {
		log.Printf("Warning: Failed to update progress: %v", err)
	}

	log.Printf("Calling %s provider (prompt length: %d chars)", e.provider.Name(), len(task.Prompt))
	e.addLog(task, "info", "Calling %s provider", e.provider.Name())

	preStatus := captureGitStatus(workdir)

	result, err := e.provider.GenerateCode(ctx, &claude.CodeRequest{
		Prompt:   task.Prompt,
		RepoPath: workdir,
		Context:  cloneStringMap(contextMap),
	})
	if err != nil {
		tracker.FailTask("Generate code changes")
		return nil, e.handleError(task, tracker, token, fmt.Sprintf("%s error: %v", e.provider.Name(), err))
	}

	tracker.CompleteTask("Generate code changes")

	log.Printf("%s completed (cost: $%.4f)", e.provider.Name(), result.CostUSD)
	e.addLog(task, "info", "%s completed (cost: $%.4f)", e.provider.Name(), result.CostUSD)

	compareGitStatus(workdir, preStatus)

	return result, nil
}

func captureGitStatus(workdir string) string {
	if os.Getenv("DEBUG_GIT_DETECTION") != "true" {
		return ""
	}

	cmd := execCommand("git", "status", "--porcelain", "--untracked-files=all")
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	status := strings.TrimSpace(string(output))
	if status != "" {
		log.Printf("[Pre-Claude] Git status:\n%s", status)
	} else {
		log.Printf("[Pre-Claude] Git status: clean")
	}
	return status
}

func compareGitStatus(workdir, before string) {
	if os.Getenv("DEBUG_GIT_DETECTION") != "true" {
		return
	}

	cmd := execCommand("git", "status", "--porcelain", "--untracked-files=all")
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return
	}
	after := strings.TrimSpace(string(output))
	log.Printf("[Post-Claude] Git status:\n%s", after)
	switch {
	case before != after:
		log.Printf("[Git Comparison] Status changed during provider execution")
	default:
		log.Printf("[Git Comparison] No status changes from provider execution")
	}
}

func (e *Executor) prepareChangePlan(
	task *webhook.Task,
	workdir string,
	result *claude.CodeResponse,
	tracker *github.CommentTracker,
	token string,
) (*github.SplitPlan, []claude.FileChange, bool, error) {
	hasChanges, err := e.detectGitChanges(workdir)
	if err != nil {
		return nil, nil, false, e.handleError(task, tracker, token, fmt.Sprintf("Failed to detect changes: %v", err))
	}

	if !hasChanges && len(result.Files) > 0 {
		log.Printf("Git detected no changes but we have %d file responses - performing manual verification", len(result.Files))
		e.addLog(task, "info", "Performing manual file verification")

		manualChanges := e.detectManualChanges(workdir, result.Files)
		if len(manualChanges) > 0 {
			log.Printf("Manual verification found %d actual file changes", len(manualChanges))
			hasChanges = true
		} else {
			log.Printf("Manual verification confirmed no actual changes were made")
		}
	}

	if !hasChanges {
		log.Printf("No file changes detected in working directory (analysis/answer only)")
		e.addLog(task, "info", "No file changes detected (response only)")
		if err := e.handleResponseOnly(task, tracker, token, result); err != nil {
			return nil, nil, true, err
		}
		return nil, nil, true, nil
	}

	log.Printf("File changes detected in working directory, proceeding with commit")
	e.addLog(task, "info", "File changes detected, preparing commit")

	changedFiles, err := e.getChangedFiles(workdir)
	if err != nil {
		return nil, nil, false, e.handleError(task, tracker, token, fmt.Sprintf("Failed to get changed files: %v", err))
	}

	log.Printf("Detected %d changed files", len(changedFiles))
	e.addLog(task, "info", "Detected %d changed files", len(changedFiles))

	splitter := github.NewPRSplitter(8, 300)
	plan := splitter.Analyze(changedFiles, task.Prompt)

	log.Printf("Split analysis: %d sub-PRs planned", len(plan.SubPRs))
	e.addLog(task, "info", "Split analysis planned %d sub-PRs", len(plan.SubPRs))

	return plan, changedFiles, false, nil
}

func (e *Executor) executeSinglePRWorkflow(
	task *webhook.Task,
	tracker *github.CommentTracker,
	token string,
	result *claude.CodeResponse,
	workdir string,
	branchName string,
	isNewBranch bool,
) error {
	log.Printf("Using single-PR workflow")
	e.addLog(task, "info", "Using single-PR workflow")

	if strings.TrimSpace(branchName) == "" {
		tracker.FailTask("Commit and push changes")
		return e.handleError(task, tracker, token, "Failed to determine branch: branch name is empty")
	}

	log.Printf("Committing changes to branch %s", branchName)
	e.addLog(task, "info", "Committing changes to branch %s", branchName)

	tracker.StartTask("Commit and push changes")
	if err := tracker.Update(token); err != nil {
		log.Printf("Warning: Failed to update progress: %v", err)
	}

	commitMsg := e.formatCommitMessage(result.Summary, task)
	if err := e.commitAndPush(workdir, task.Repo, branchName, commitMsg, isNewBranch, token); err != nil {
		tracker.FailTask("Commit and push changes")
		return e.handleError(task, tracker, token, fmt.Sprintf("Failed to commit/push: %v", err))
	}
	tracker.CompleteTask("Commit and push changes")

	if !task.IsPR || task.PRState != "open" {
		tracker.StartTask("Create pull request")
		if err := tracker.Update(token); err != nil {
			log.Printf("Warning: Failed to update progress: %v", err)
		}
	}

	log.Printf("Creating PR from %s to %s", branchName, task.Branch)
	e.addLog(task, "info", "Creating PR from %s to %s", branchName, task.Branch)

    prURL, err := e.createPRLink(task.Repo, branchName, task.Branch, result.Summary)
	if err != nil {
		if !task.IsPR || task.PRState != "open" {
			tracker.FailTask("Create pull request")
		}
		return e.handleError(task, tracker, token, fmt.Sprintf("Failed to create PR: %v", err))
	}
	if !task.IsPR || task.PRState != "open" {
		tracker.CompleteTask("Create pull request")
	}

	log.Printf("PR link created: %s", prURL)
	e.addLog(task, "info", "PR link created: %s", prURL)

	branchURL := fmt.Sprintf("https://github.com/%s/tree/%s", task.Repo, url.PathEscape(branchName))

	tracker.MarkEnd()
	tracker.SetCompleted(result.Summary, e.extractFilePaths(result.Files), result.CostUSD)
	tracker.SetBranch(branchName, branchURL)
	tracker.SetPRURL(prURL)

	if err := tracker.Update(token); err != nil {
		log.Printf("Warning: Failed to update tracking comment: %v", err)
		e.addLog(task, "error", "Failed to update tracking comment: %v", err)
	}

	log.Printf("Task completed successfully")
	e.addLog(task, "success", "Task completed successfully")
	e.updateStatus(task, taskstore.StatusCompleted)
	return nil
}

// Execute executes a pilot task
func (e *Executor) Execute(ctx context.Context, task *webhook.Task) error {
	attempt := e.ensureAttempt(task)

	e.updateStatus(task, taskstore.StatusRunning)
	e.addLog(task, "info", "Starting task execution for %s#%d (attempt %d)", task.Repo, task.Number, attempt)
	log.Printf("Starting task execution for %s#%d (attempt %d)", task.Repo, task.Number, attempt)

	contextMap := e.buildExecutionContext(task)

	installToken, err := e.authenticateWithGitHub(task)
	if err != nil {
		return err
	}

	e.enrichPromptWithDiscussion(task, installToken.Token)

	tracker := e.initializeTracker(task, contextMap, installToken.Token)

	e.ensureTrackingLabel(task, tracker, installToken.Token)

	workdir, cleanup, branchName, isNewBranch, err := e.cloneAndPrepareWorkspace(task, tracker, installToken.Token, contextMap)
	if err != nil {
		return err
	}
	defer cleanup()

	result, err := e.generateCodeChanges(ctx, task, workdir, contextMap, tracker, installToken.Token)
	if err != nil {
		return err
	}

	if len(result.Files) > 0 {
		log.Printf("%s returned %d file changes, applying them", e.provider.Name(), len(result.Files))
		e.addLog(task, "info", "%s returned %d file changes, applying them", e.provider.Name(), len(result.Files))
		if err := e.applyChanges(workdir, result.Files); err != nil {
			return e.handleError(task, tracker, installToken.Token, fmt.Sprintf("Failed to apply changes: %v", err))
		}
	} else {
		log.Printf("%s did not return file list, checking git status for direct modifications", e.provider.Name())
		e.addLog(task, "info", "%s did not return file list, checking git status", e.provider.Name())
	}

	plan, _, handled, err := e.prepareChangePlan(task, workdir, result, tracker, installToken.Token)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	if len(plan.SubPRs) > 1 {
		log.Printf("Using multi-PR workflow")
		e.addLog(task, "info", "Using multi-PR workflow")
		return e.executeMultiPR(ctx, task, workdir, plan, result, tracker, installToken.Token)
	}

	return e.executeSinglePRWorkflow(task, tracker, installToken.Token, result, workdir, branchName, isNewBranch)
}

// applyChanges writes file changes to disk with enhanced validation and logging
func (e *Executor) applyChanges(workdir string, changes []claude.FileChange) error {
	log.Printf("Applying %d file changes to %s", len(changes), workdir)

	successCount := 0
	for i, change := range changes {
		if change.Path == "" {
			log.Printf("Warning: Skipping empty file path at index %d", i)
			continue
		}

		cleanPath := filepath.Clean(change.Path)

		if filepath.IsAbs(cleanPath) {
			return fmt.Errorf("absolute paths are not allowed: %s", change.Path)
		}

		if cleanPath == "." || cleanPath == ".." {
			return fmt.Errorf("invalid file path %s: resolves outside workdir", change.Path)
		}

		filePath := filepath.Join(workdir, cleanPath)

		relative, err := filepath.Rel(workdir, filePath)
		if err != nil {
			return fmt.Errorf("failed to resolve file path %s: %w", change.Path, err)
		}

		if relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("path traversal detected for %s", change.Path)
		}

		// Debug logging
		if os.Getenv("DEBUG_GIT_DETECTION") == "true" || os.Getenv("DEBUG_CLAUDE_PARSING") == "true" {
			log.Printf("[File Write %d/%d] Processing: %s", i+1, len(changes), change.Path)
			log.Printf("[File Write %d/%d] Content length: %d chars", i+1, len(changes), len(change.Content))
		}

		// Check if file already exists
		existsBefore := false
		if _, err := os.Stat(filePath); err == nil {
			existsBefore = true
		}

		// Ensure directory exists
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", change.Path, err)
		}

		// Write file
		if err := os.WriteFile(filePath, []byte(change.Content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", change.Path, err)
		}

		// Verify file was written correctly
		if writtenContent, err := os.ReadFile(filePath); err != nil {
			return fmt.Errorf("failed to verify written file %s: %w", change.Path, err)
		} else if string(writtenContent) != change.Content {
			return fmt.Errorf("file content mismatch for %s: expected %d chars, got %d chars",
				change.Path, len(change.Content), len(writtenContent))
		}

		// Log successful write
		action := "modified"
		if !existsBefore {
			action = "created"
		}
		log.Printf("Successfully %s %s (%d bytes)", action, change.Path, len(change.Content))
		successCount++
	}

	log.Printf("File changes applied: %d successful out of %d requested", successCount, len(changes))
	return nil
}

// detectGitChanges checks if there are any uncommitted changes in the working directory
// Enhanced with untracked files detection and debugging
func (e *Executor) detectGitChanges(workdir string) (bool, error) {
	// Use --untracked-files=all to include new files
	cmd := execCommand("git", "status", "--porcelain", "--untracked-files=all")
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git status failed: %w\nOutput: %s", err, string(output))
	}

	outputStr := strings.TrimSpace(string(output))
	hasChanges := len(outputStr) > 0

	// Debug logging if enabled
	if os.Getenv("DEBUG_GIT_DETECTION") == "true" {
		log.Printf("[Git Detection] Working directory: %s", workdir)
		log.Printf("[Git Detection] Command: git status --porcelain --untracked-files=all")
		log.Printf("[Git Detection] Output length: %d", len(outputStr))
		if hasChanges {
			log.Printf("[Git Detection] Changes detected:\n%s", outputStr)
		} else {
			log.Printf("[Git Detection] No changes detected")
		}
	}

	return hasChanges, nil
}

// detectManualChanges manually verifies if parsed files actually have changes
// This is used as a fallback when git detection fails
func (e *Executor) detectManualChanges(workdir string, parsedFiles []claude.FileChange) []claude.FileChange {
	var actualChanges []claude.FileChange

	for _, file := range parsedFiles {
		filePath := filepath.Join(workdir, file.Path)

		// Check if file exists and compare content
		if existingContent, err := os.ReadFile(filePath); err != nil {
			if os.IsNotExist(err) {
				// New file - this is a change
				if strings.TrimSpace(file.Content) != "" {
					actualChanges = append(actualChanges, file)
					log.Printf("[Manual Detection] New file: %s", file.Path)
				}
			} else {
				log.Printf("[Manual Detection] Error reading %s: %v", file.Path, err)
			}
		} else {
			// File exists - check if content is different
			if string(existingContent) != file.Content {
				actualChanges = append(actualChanges, file)
				log.Printf("[Manual Detection] Modified file: %s", file.Path)
			} else {
				log.Printf("[Manual Detection] No changes in: %s", file.Path)
			}
		}
	}

	return actualChanges
}

// formatCommitMessage formats a commit message with tool signature and co-authorship
func (e *Executor) formatCommitMessage(summary string, task *webhook.Task) string {
	var parts []string

	// Add main summary
	parts = append(parts, summary)

	// Add issue reference for issues (not PRs)
	if task.Number > 0 && !task.IsPR {
		parts = append(parts, fmt.Sprintf("Fixes #%d", task.Number))
	}

	// Add tool signature
	parts = append(parts, "Generated with [SWE Agent](https://github.com/cexll/swe-agent)")

	// Add Co-authored-by if username is available
	if task.Username != "" && task.Username != "Unknown" {
		email := fmt.Sprintf("%s@users.noreply.github.com", task.Username)
		parts = append(parts, fmt.Sprintf("Co-authored-by: %s <%s>", task.Username, email))
	}

	return strings.Join(parts, "\n\n")
}

func (e *Executor) prepareBranch(workdir string, task *webhook.Task) (string, bool, error) {
	if task.IsPR && task.PRState == "open" && task.PRBranch != "" {
		branchName := task.PRBranch
		log.Printf("PR #%d is open, using existing branch: %s", task.Number, branchName)
		e.addLog(task, "info", "PR is open, using existing branch: %s", branchName)

		commands := [][]string{
			{"git", "fetch", "origin", branchName},
			{"git", "checkout", branchName},
		}

		for _, args := range commands {
			cmd := execCommand(args[0], args[1:]...)
			cmd.Dir = workdir
			if output, err := cmd.CombinedOutput(); err != nil {
				return "", false, fmt.Errorf("%s failed: %w\nOutput: %s", strings.Join(args, " "), err, string(output))
			}
		}

		return branchName, false, nil
	}

	branchName := e.generateWorkingBranchName(task)

	switch {
	case task.IsPR && task.PRState == "closed":
		log.Printf("PR #%d is closed, creating new branch: %s", task.Number, branchName)
		e.addLog(task, "info", "PR is closed, creating new branch: %s", branchName)
	case task.IsPR:
		log.Printf("PR #%d requires a new branch: %s", task.Number, branchName)
		e.addLog(task, "info", "Creating new working branch for PR: %s", branchName)
	default:
		log.Printf("Issue #%d, creating new branch: %s", task.Number, branchName)
		e.addLog(task, "info", "Creating new branch for issue: %s", branchName)
	}

	cmd := execCommand("git", "checkout", "-b", branchName)
	cmd.Dir = workdir
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", false, fmt.Errorf("git checkout -b %s failed: %w\nOutput: %s", branchName, err, string(output))
	}

	return branchName, true, nil
}

func (e *Executor) generateWorkingBranchName(task *webhook.Task) string {
	entity := "issue"
	if task.IsPR {
		entity = "pr"
	}
	number := task.Number
	if number <= 0 {
		number = int(time.Now().Unix())
	}
	return fmt.Sprintf("swe/%s-%d-%d", entity, number, time.Now().Unix())
}

func generateSubPRBranchName(issueNumber int, category string) string {
	segment := sanitizeBranchSegment(category)
	if segment == "" {
		segment = "change"
	}
	if issueNumber <= 0 {
		issueNumber = int(time.Now().Unix())
	}
	return fmt.Sprintf("swe/%s-%d-%d", segment, issueNumber, time.Now().Unix())
}

func sanitizeBranchSegment(segment string) string {
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return ""
	}
	segment = strings.ToLower(segment)
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", "_", "-")
	segment = replacer.Replace(segment)
	segment = strings.Trim(segment, "-")
	return segment
}

// commitAndPush commits changes and pushes to remote
func (e *Executor) commitAndPush(workdir, repo, branchName, commitMessage string, isNewBranch bool, token string) error {
	name, email := resolveGitIdentity()

	setupCommands := [][]string{
		{"git", "config", "user.name", name},
		{"git", "config", "user.email", email},
	}

	for _, args := range setupCommands {
		if err := runGitCommand(workdir, args, false); err != nil {
			return err
		}
	}

	commands := [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", commitMessage},
	}

	for _, args := range commands {
		if err := runGitCommand(workdir, args, false); err != nil {
			return err
		}
	}

	cleanup := func() {}
	if token != "" && repo != "" {
		var err error
		if cleanup, err = configurePushURL(workdir, repo, token); err != nil {
			return err
		}
		defer cleanup()
	}

    pushArgs := []string{"git", "push", "origin", branchName}
    if isNewBranch {
        pushArgs = []string{"git", "push", "-u", "origin", branchName}
    }

    // Push with small exponential backoff for transient network issues (KISS)
    var lastErr error
    for attempt := 0; attempt < 3; attempt++ {
        if attempt > 0 {
            // 1s, 2s backoff
            time.Sleep(time.Duration(attempt) * time.Second)
        }
        if err := runGitCommand(workdir, pushArgs, false); err != nil {
            lastErr = err
            if shouldRetryPush(err) {
                continue
            }
            return err
        }
        lastErr = nil
        break
    }
    if lastErr != nil {
        return lastErr
    }

	return nil
}

// shouldRetryPush returns true for common transient network errors on git push
func shouldRetryPush(err error) bool {
    if err == nil {
        return false
    }
    s := strings.ToLower(err.Error())
    patterns := []string{
        "timeout",
        "connection refused",
        "temporary failure",
        "connection reset",
        "broken pipe",
        "no such host",
        "network is unreachable",
        "eof",
    }
    for _, p := range patterns {
        if strings.Contains(s, p) {
            return true
        }
    }
    return false
}

func resolveGitIdentity() (string, string) {
	name := os.Getenv("SWE_AGENT_GIT_NAME")
	if strings.TrimSpace(name) == "" {
		name = "SWE Agent[bot]"
	}

	email := os.Getenv("SWE_AGENT_GIT_EMAIL")
	if strings.TrimSpace(email) == "" {
		email = "swe-agent[bot]@users.noreply.github.com"
	}

	return name, email
}

func configurePushURL(workdir, repo, token string) (func(), error) {
	if strings.TrimSpace(token) == "" {
		return func() {}, nil
	}

	pushURL, err := resolvePushURL(workdir, repo, token)
	if err != nil || pushURL == "" {
		return func() {}, nil
	}

	if err := runGitCommand(workdir, []string{"git", "config", "remote.origin.pushurl", pushURL}, true); err != nil {
		return nil, err
	}

	cleanup := func() {
		if err := runGitCommand(workdir, []string{"git", "config", "--unset", "remote.origin.pushurl"}, false); err != nil {
			if !strings.Contains(err.Error(), "No such section or key") {
				log.Printf("Warning: cleanup of remote.origin.pushurl failed: %v", err)
			}
		}
	}

	return cleanup, nil
}

func resolvePushURL(workdir, repo, token string) (string, error) {
	remoteURL, err := getRemoteOriginURL(workdir)
	if err == nil {
		// Prefer using existing HTTPS GitHub remote when possible
		if url := injectToken(remoteURL, token); url != "" {
			return url, nil
		}
		// If the existing remote is a GitHub SSH URL, configure a separate
		// pushurl that uses HTTPS + token so we can push without SSH keys.
		if looksLikeGitHubSSH(remoteURL) && strings.TrimSpace(repo) != "" {
			fallback := fmt.Sprintf("https://github.com/%s", strings.TrimSpace(repo))
			if url := injectToken(fallback, token); url != "" {
				return url, nil
			}
		}
		// Could not construct a tokenized push URL; return empty to skip override.
		return "", nil
	}

	if strings.TrimSpace(repo) == "" {
		return "", nil
	}

	// Fallback to GitHub HTTPS URL when remote isn't available.
	fallback := fmt.Sprintf("https://github.com/%s", strings.TrimSpace(repo))
	return injectToken(fallback, token), nil
}

func looksLikeGitHubSSH(remote string) bool {
	s := strings.TrimSpace(remote)
	if s == "" {
		return false
	}
	// Common SSH URL patterns for GitHub
	if strings.HasPrefix(s, "git@github.com:") {
		return true
	}
	if strings.HasPrefix(strings.ToLower(s), "ssh://git@github.com/") {
		return true
	}
	return false
}

func getRemoteOriginURL(workdir string) (string, error) {
	cmd := execCommand("git", "remote", "get-url", "origin")
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git remote get-url origin failed: %w\nOutput: %s", err, strings.TrimSpace(string(output)))
	}

	return strings.TrimSpace(string(output)), nil
}

func injectToken(rawURL, token string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	if parsed.Scheme != "https" {
		return ""
	}

	host := strings.ToLower(parsed.Host)
	if host == "" || !strings.Contains(host, "github.com") {
		return ""
	}

	parsed.User = url.UserPassword("x-access-token", token)
	return parsed.String()
}

func runGitCommand(workdir string, args []string, sensitive bool) error {
	if len(args) == 0 {
		return fmt.Errorf("git command is empty")
	}

	cmd := execCommand(args[0], args[1:]...)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := cmd.CombinedOutput()
	if err != nil {
		commandLabel := strings.Join(args, " ")
		outputStr := string(output)
		if sensitive {
			commandLabel = fmt.Sprintf("%s (arguments redacted)", args[0])
			outputStr = "[redacted]"
		}
		return fmt.Errorf("%s failed: %w\nOutput: %s", commandLabel, err, outputStr)
	}

	return nil
}

// createPRLink generates a GitHub URL for creating a PR
func (e *Executor) createPRLink(repo, head, base, title string) (string, error) {
    // Generate GitHub compare URL that allows user to create PR with prefilled title/body
    // Format: https://github.com/owner/repo/compare/base...head?expand=1&quick_pull=1&title=...&body=...
    t := url.QueryEscape(strings.TrimSpace(title))
    // KISS: reuse title as body when a separate body isn't provided by caller
    b := t
    prURL := fmt.Sprintf("https://github.com/%s/compare/%s...%s?expand=1&quick_pull=1&title=%s",
        repo, url.PathEscape(base), url.PathEscape(head), t)
    prURL = prURL + "&body=" + b
    if _, err := url.Parse(prURL); err != nil {
        return "", fmt.Errorf("invalid PR URL: %w", err)
    }
    return prURL, nil
}

// handleError updates the tracking comment with error details and returns the error
func (e *Executor) handleError(task *webhook.Task, tracker *github.CommentTracker, token, errorMsg string) error {
    tracker.MarkEnd()
    tracker.SetFailed(errorMsg)
    e.updateStatus(task, taskstore.StatusFailed)
    e.addLog(task, "error", "%s", errorMsg)

    // Provide actionable hints without altering the primary error message
    for _, hint := range deriveHelpfulHints(errorMsg, task) {
        e.addLog(task, "hint", "%s", hint)
    }

	if err := tracker.Update(token); err != nil {
		log.Printf("Warning: Failed to update tracking comment with error: %v", err)
		e.addLog(task, "error", "Failed to update tracking comment with error: %v", err)
	}

	if isNonRetryableTaskError(errorMsg) {
		return &NonRetryableError{msg: errorMsg}
	}

	return fmt.Errorf("%s", errorMsg)
}

func isNonRetryableTaskError(msg string) bool {
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "api error: 401"):
		return true
	case strings.Contains(lower, "invalid token"):
		return true
	case strings.Contains(msg, "无效的令牌"):
		return true
	case strings.Contains(lower, "please run /login"):
		return true
	default:
		return false
	}
}

// deriveHelpfulHints produces non-intrusive, actionable hints for common failures.
// It does not modify user-visible primary error text to keep tests stable.
func deriveHelpfulHints(msg string, task *webhook.Task) []string {
    s := strings.ToLower(strings.TrimSpace(msg))
    if s == "" {
        return nil
    }

    var hints []string

    // Authentication / token problems
    if strings.Contains(s, "api error: 401") || strings.Contains(s, "invalid token") || strings.Contains(s, "authentication failed") {
        hints = append(hints, "Check GitHub App credentials and installation: GITHUB_APP_ID, GITHUB_PRIVATE_KEY, GITHUB_WEBHOOK_SECRET.")
        hints = append(hints, "Ensure the App is installed on the target repository with Contents: Read & Write permissions.")
    }

    // Permission denied on push/clone
    if strings.Contains(s, "permission denied") || strings.Contains(s, "http 403") || strings.Contains(s, "403 forbidden") || strings.Contains(s, "remote: permission to") {
        hints = append(hints, "Push permission denied: verify the App installation on this repo and branch protections.")
        hints = append(hints, "If this is a PR from a fork, ensure the workflow/app has permission to push to the branch.")
    }

    // No credential prompt allowed (common when token injection failed)
    if strings.Contains(s, "could not read username for 'https://github.com': terminal prompts disabled") {
        hints = append(hints, "Token not injected into remote.pushurl. Confirm repository string is correct and pushurl configuration succeeded.")
    }

    // Branch/ref issues
    if strings.Contains(s, "couldn't find remote ref") || strings.Contains(s, "src refspec") {
        hints = append(hints, "Remote branch not found. Verify branch creation and that the correct head/base branches are used.")
    }

    // Network/transient errors
    if shouldRetryPush(errors.New(s)) || strings.Contains(s, "no such host") || strings.Contains(s, "network is unreachable") {
        hints = append(hints, "Transient network issue detected. A quick retry may succeed; check network connectivity.")
    }

    // Generic guidance
    if len(hints) == 0 {
        hints = append(hints, "Review logs above for the failing step and verify GitHub permissions and branch setup.")
    }

    return hints
}

// handleResponseOnly updates the tracking comment with AI response (no code changes)
func (e *Executor) handleResponseOnly(task *webhook.Task, tracker *github.CommentTracker, token string, result *claude.CodeResponse) error {
	tracker.MarkEnd()
	tracker.SetCompleted(result.Summary, nil, result.CostUSD)
	e.updateStatus(task, taskstore.StatusCompleted)
	e.addLog(task, "success", "Task completed with response only")

	if err := tracker.Update(token); err != nil {
		log.Printf("Warning: Failed to update tracking comment: %v", err)
		e.addLog(task, "error", "Failed to update tracking comment: %v", err)
	}

	log.Printf("Task completed (response only, no code changes)")
	e.addLog(task, "success", "Task completed (response only, no code changes)")
	return nil
}

func (e *Executor) composeDiscussionSection(task *webhook.Task, token string) string {
	issueComments, err := e.ghClient.ListIssueComments(task.Repo, task.Number, token)
	if err != nil {
		log.Printf("Warning: Failed to fetch issue comments: %v", err)
		issueComments = nil
	}

	var reviewComments []github.ReviewComment
	if task.IsPR {
		reviewComments, err = e.ghClient.ListReviewComments(task.Repo, task.Number, token)
		if err != nil {
			log.Printf("Warning: Failed to fetch review comments: %v", err)
			reviewComments = nil
		}
	}

	return formatDiscussion(issueComments, reviewComments)
}

type discussionEntry struct {
	author    string
	body      string
	metadata  string
	createdAt time.Time
}

func formatDiscussion(issueComments []github.IssueComment, reviewComments []github.ReviewComment) string {
	total := len(issueComments) + len(reviewComments)
	if total == 0 {
		return ""
	}

	entries := make([]discussionEntry, 0, total)

	for _, c := range issueComments {
		body := strings.TrimSpace(c.Body)
		if body == "" {
			continue
		}
		author := strings.TrimSpace(c.Author)
		if author == "" {
			author = "unknown"
		}
		entries = append(entries, discussionEntry{
			author:    author,
			body:      body,
			createdAt: c.CreatedAt,
		})
	}

	for _, c := range reviewComments {
		body := strings.TrimSpace(c.Body)
		if body == "" {
			continue
		}
		author := strings.TrimSpace(c.Author)
		if author == "" {
			author = "unknown"
		}
		metadata := ""
		if path := strings.TrimSpace(c.Path); path != "" {
			metadata = fmt.Sprintf("_File: %s_", path)
			if diff := strings.TrimSpace(c.DiffHunk); diff != "" {
				metadata += "\n```diff\n" + diff + "\n```"
			}
		}

		entries = append(entries, discussionEntry{
			author:    author,
			body:      body,
			metadata:  metadata,
			createdAt: c.CreatedAt,
		})
	}

	if len(entries) == 0 {
		return ""
	}

	sort.SliceStable(entries, func(i, j int) bool {
		aZero := entries[i].createdAt.IsZero()
		bZero := entries[j].createdAt.IsZero()
		switch {
		case aZero && bZero:
			return i < j
		case aZero:
			return true
		case bZero:
			return false
		default:
			return entries[i].createdAt.Before(entries[j].createdAt)
		}
	})

	var builder strings.Builder
	builder.WriteString("## Discussion\n")

	for _, entry := range entries {
		timestamp := "unknown"
		if !entry.createdAt.IsZero() {
			timestamp = entry.createdAt.UTC().Format(time.RFC3339)
		}

		builder.WriteString("\n@")
		builder.WriteString(entry.author)
		builder.WriteString(" (")
		builder.WriteString(timestamp)
		builder.WriteString("):\n")

		if entry.metadata != "" {
			builder.WriteString(entry.metadata)
			builder.WriteString("\n")
		}

		builder.WriteString(entry.body)
		builder.WriteString("\n")
	}

	return strings.TrimRight(builder.String(), "\n")
}

func injectDiscussion(basePrompt, discussion string) string {
	discussion = strings.TrimSpace(discussion)
	if discussion == "" {
		return basePrompt
	}

	discussionBlock := discussion
	if !strings.HasSuffix(discussionBlock, "\n") {
		discussionBlock += "\n"
	}

	separator := "\n\n---\n\n"
	if idx := strings.Index(basePrompt, separator); idx != -1 {
		prefix := strings.TrimRight(basePrompt[:idx], "\n")
		suffix := strings.TrimLeft(basePrompt[idx:], "\n")

		var builder strings.Builder
		builder.WriteString(prefix)
		builder.WriteString("\n\n")
		builder.WriteString(discussionBlock)
		builder.WriteString("\n\n")
		builder.WriteString(suffix)
		return builder.String()
	}

	trimmed := strings.TrimRight(basePrompt, "\n")
	if trimmed == "" {
		return discussionBlock
	}

	return trimmed + "\n\n" + discussionBlock
}

// extractFilePaths extracts file paths from FileChange array
func (e *Executor) extractFilePaths(files []claude.FileChange) []string {
	paths := make([]string, len(files))
	for i, file := range files {
		paths[i] = file.Path
	}
	return paths
}

func (e *Executor) updateStatus(task *webhook.Task, status taskstore.TaskStatus) {
	if e.store == nil || task == nil || task.ID == "" {
		return
	}
	e.store.UpdateStatus(task.ID, status)
}

func (e *Executor) addLog(task *webhook.Task, level, format string, args ...interface{}) {
	if e.store == nil || task == nil || task.ID == "" {
		return
	}
	e.store.AddLog(task.ID, level, fmt.Sprintf(format, args...))
}

// getChangedFiles gets list of changed files from git status
func (e *Executor) getChangedFiles(workdir string) ([]claude.FileChange, error) {
	cmd := execCommand("git", "status", "--porcelain")
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
	e.addLog(task, "info", "Executing multi-PR workflow with %d sub-PRs", len(plan.SubPRs))

	// Store AI-generated summary in tracker state (will be displayed in split plan section)
	tracker.State.Summary = result.Summary

	// Update tracker to show split plan
	tracker.SetSplitPlan(plan)
	if err := tracker.Update(token); err != nil {
		log.Printf("Warning: Failed to update comment with split plan: %v", err)
		e.addLog(task, "error", "Failed to update comment with split plan: %v", err)
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
		branchName := generateSubPRBranchName(task.Number, string(subPR.Category))

		// Commit only files from this sub-PR
		if err := e.commitSubPR(workdir, task.Repo, branchName, subPR, task, token); err != nil {
			log.Printf("Warning: Failed to create sub-PR #%d: %v", idx, err)
			// Continue with other PRs
			continue
		}

		// Generate PR URL
        prURL, _ := e.createPRLink(task.Repo, branchName, task.Branch, subPR.Name)
		branchURL := fmt.Sprintf("https://github.com/%s/tree/%s", task.Repo, url.PathEscape(branchName))

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
			e.addLog(task, "error", "Failed to update comment during multi-PR: %v", err)
		}

		log.Printf("Created sub-PR #%d: %s", idx, prURL)
		e.addLog(task, "info", "Created sub-PR #%d: %s", idx, prURL)
	}

	// Mark task as completed
	tracker.MarkEnd()
	tracker.SetCompletedWithSplit(plan, createdPRs, result.CostUSD)

	if err := tracker.Update(token); err != nil {
		log.Printf("Warning: Failed to update final comment: %v", err)
		e.addLog(task, "error", "Failed to update final comment for multi-PR: %v", err)
	}

	log.Printf("Multi-PR workflow completed: %d PRs created", len(createdPRs))
	e.addLog(task, "success", "Multi-PR workflow completed: %d PRs created", len(createdPRs))
	e.updateStatus(task, taskstore.StatusCompleted)
	return nil
}

// commitSubPR commits only the files from a specific sub-PR
func (e *Executor) commitSubPR(workdir, repo, branchName string, subPR github.SubPR, task *webhook.Task, token string) error {
	// Reset to base branch first
	resetCmd := execCommand("git", "reset", "--hard", "HEAD")
	resetCmd.Dir = workdir
	if output, err := resetCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset failed: %w\nOutput: %s", err, string(output))
	}

	// Clean untracked files
	cleanCmd := execCommand("git", "clean", "-fd")
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
	commitMsg := e.formatCommitMessage(subPR.Name+"\n\n"+subPR.Description, task)
	name, email := resolveGitIdentity()
	if err := runGitCommand(workdir, []string{"git", "config", "user.name", name}, false); err != nil {
		return err
	}
	if err := runGitCommand(workdir, []string{"git", "config", "user.email", email}, false); err != nil {
		return err
	}

	if err := runGitCommand(workdir, []string{"git", "checkout", "-b", branchName}, false); err != nil {
		return err
	}
	if err := runGitCommand(workdir, []string{"git", "add", "."}, false); err != nil {
		return err
	}
	if err := runGitCommand(workdir, []string{"git", "commit", "-m", commitMsg}, false); err != nil {
		return err
	}

	cleanup := func() {}
	if token != "" && repo != "" {
		var err error
		if cleanup, err = configurePushURL(workdir, repo, token); err != nil {
			return err
		}
		defer cleanup()
	}

    // Push with small exponential backoff for transient network issues
    var pushErr error
    for attempt := 0; attempt < 3; attempt++ {
        if attempt > 0 {
            time.Sleep(time.Duration(attempt) * time.Second)
        }
        if err := runGitCommand(workdir, []string{"git", "push", "-u", "origin", branchName}, false); err != nil {
            pushErr = err
            if shouldRetryPush(err) {
                continue
            }
            return err
        }
        pushErr = nil
        break
    }
    if pushErr != nil {
        return pushErr
    }

	return nil
}
