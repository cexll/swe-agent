package github

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// CommentTracker manages a single GitHub comment throughout task execution
// This eliminates the special cases of multiple notification functions
// by unifying all comment updates into a single state-driven approach
type CommentTracker struct {
	Repo      string
	Number    int
	CommentID int
	State     *CommentState
	ghClient  GHClient
}

// NewCommentTracker creates a new comment tracker
func NewCommentTracker(repo string, number int, username string) *CommentTracker {
	return &CommentTracker{
		Repo:      repo,
		Number:    number,
		CommentID: -1, // Not yet created
		State: &CommentState{
			Status:   StatusWorking,
			Username: username,
			Context:  make(map[string]string),
		},
		ghClient: defaultGHClient,
	}
}

// NewCommentTrackerWithClient creates a new comment tracker with a custom gh client
func NewCommentTrackerWithClient(repo string, number int, username string, ghClient GHClient) *CommentTracker {
	return &CommentTracker{
		Repo:      repo,
		Number:    number,
		CommentID: -1,
		State: &CommentState{
			Status:   StatusWorking,
			Username: username,
			Context:  make(map[string]string),
		},
		ghClient: ghClient,
	}
}

// Create creates the initial tracking comment
func (t *CommentTracker) Create(token string) error {
	body := t.renderBody()
	commentID, err := t.ghClient.CreateComment(t.Repo, t.Number, body, token)
	if err != nil {
		return fmt.Errorf("failed to create tracking comment: %w", err)
	}
	t.CommentID = commentID
	return nil
}

// Update updates the existing tracking comment
func (t *CommentTracker) Update(token string) error {
	if t.CommentID <= 0 {
		return fmt.Errorf("cannot update comment: not yet created")
	}

	body := t.renderBody()
	return t.ghClient.UpdateComment(t.Repo, t.CommentID, body, token)
}

// renderBody renders the comment body based on current state
// Single function handles all states - no special cases
func (t *CommentTracker) renderBody() string {
	state := t.State

	// Minimal view for queued/working statuses
	if state.Status == StatusWorking || state.Status == StatusQueued {
		username := state.Username
		if username == "" {
			username = "user"
		}

		message := ""
		if state.Status == StatusWorking {
			message = fmt.Sprintf("SWE Agent is working on @%s's task", username)
		} else {
			message = fmt.Sprintf("SWE Agent is queued for @%s's task", username)
		}

		return message + " <img src=\"https://github.githubassets.com/images/spinners/octocat-spinner-32.gif\" width=\"20\" height=\"20\" alt=\"loading\" />"
	}

	header := t.buildHeader()
	links := t.buildLinks()

	headerLine := header
	if links != "" {
		headerLine = headerLine + " " + links
	}

	var sections []string
	sections = append(sections, headerLine)

	switch {
	case state.IsCompleted():
		if state.Summary != "" {
			sections = append(sections, "", state.Summary)
		}
		if len(state.ModifiedFiles) > 0 {
			sections = append(sections, "", t.buildModifiedFilesList())
		}
		if state.SplitPlan != nil {
			if splitSection := t.buildSplitPlanSection(); splitSection != "" {
				sections = append(sections, "", splitSection)
			}
		}
	case state.IsFailed():
		if state.ErrorDetails != "" {
			sections = append(sections, "", "```", state.ErrorDetails, "```")
		}
	default:
		if jobLink := t.buildFooter(); jobLink != "" {
			sections = append(sections, "", jobLink)
			return strings.Join(sections, "\n")
		}
	}

	sections = append(sections, "", t.buildFooter())
	return strings.Join(sections, "\n")
}

// buildHeader builds the status header
func (t *CommentTracker) buildHeader() string {
	state := t.State
	username := state.Username
	if username == "" {
		username = "user"
	}

	switch state.Status {
	case StatusQueued:
		return fmt.Sprintf("**SWE Agent is queued for @%s's task**", username)
	case StatusWorking:
		return fmt.Sprintf("**SWE Agent is working on @%s's task**", username)

	case StatusCompleted:
		duration := state.Duration()
		if duration != "" {
			return fmt.Sprintf("**SWE Agent finished @%s's task in %s**", username, duration)
		}
		return fmt.Sprintf("**SWE Agent finished @%s's task**", username)

	case StatusFailed:
		duration := state.Duration()
		if duration != "" {
			return fmt.Sprintf("**SWE Agent encountered an error after %s**", duration)
		}
		return "**SWE Agent encountered an error**"

	default:
		return "**SWE Agent Task Status**"
	}
}

// buildLinks builds the links section (job, branch, PR)
func (t *CommentTracker) buildLinks() string {
	state := t.State
	var links []string

	// Add job link first (matches claude-code-action)
	if state.JobURL != "" {
		links = append(links, fmt.Sprintf("[View job](%s)", state.JobURL))
	}

	// Add branch link second
	if state.BranchName != "" {
		if state.BranchURL != "" {
			links = append(links, fmt.Sprintf("[`%s`](%s)", state.BranchName, state.BranchURL))
		} else {
			links = append(links, fmt.Sprintf("`%s`", state.BranchName))
		}
	}

	// Add PR link last
	if state.PRURL != "" {
		links = append(links, fmt.Sprintf("[Create PR ➔](%s)", state.PRURL))
	}

	if len(state.CreatedPRs) > 0 {
		seen := make(map[string]struct{})
		for _, pr := range state.CreatedPRs {
			if pr.URL == "" {
				continue
			}

			if _, exists := seen[pr.URL]; exists {
				continue
			}
			seen[pr.URL] = struct{}{}

			label := strings.TrimSpace(pr.Name)
			if label == "" {
				label = fmt.Sprintf("PR %d", pr.Index+1)
			}
			label = escapeMarkdownLinkText(label)

			links = append(links, fmt.Sprintf("[Create PR: %s ➔](%s)", label, pr.URL))
		}
	}

	if len(links) == 0 {
		return ""
	}

	// Format: —— link1 • link2 • link3 (with space before ——)
	return " —— " + strings.Join(links, " • ")
}

// Deprecated sections removed for simplified comment layout

// (removed) instruction section builder to avoid dumping full prompt into comments

// buildModifiedFilesList builds the modified files list
func (t *CommentTracker) buildModifiedFilesList() string {
	state := t.State
	count := len(state.ModifiedFiles)

	var lines []string
	lines = append(lines, fmt.Sprintf("**Modified Files:** (%d)", count))

	for _, file := range state.ModifiedFiles {
		lines = append(lines, fmt.Sprintf("- `%s`", file))
	}

	return strings.Join(lines, "\n")
}

// buildFooter builds the footer with metadata
func (t *CommentTracker) buildFooter() string {
	state := t.State

	// For completed tasks, show cost if available
	if state.IsCompleted() && state.CostUSD > 0 {
		return fmt.Sprintf("*Generated with [SWE Agent](https://github.com/cexll/swe-agent) • Cost: $%.4f*", state.CostUSD)
	}

	return "*Generated with [SWE Agent](https://github.com/cexll/swe-agent)*"
}

// SetWorking sets the task status to working
func (t *CommentTracker) SetWorking() {
	t.State.Status = StatusWorking
}

// SetQueued sets the task status to queued
func (t *CommentTracker) SetQueued() {
	t.State.Status = StatusQueued
}

// SetCompleted sets the task status to completed
func (t *CommentTracker) SetCompleted(summary string, modifiedFiles []string, costUSD float64) {
	t.State.Status = StatusCompleted
	t.State.Summary = summary
	t.State.ModifiedFiles = modifiedFiles
	t.State.CostUSD = costUSD
}

// SetFailed sets the task status to failed
func (t *CommentTracker) SetFailed(errorDetails string) {
	t.State.Status = StatusFailed
	t.State.ErrorDetails = errorDetails
}

// SetBranch sets the branch information
func (t *CommentTracker) SetBranch(branchName, branchURL string) {
	t.State.BranchName = branchName
	t.State.BranchURL = branchURL
	if t.State.Context == nil {
		t.State.Context = make(map[string]string)
	}
	if branchName != "" {
		t.State.Context["claude_branch"] = branchName
	}
}

// SetPRURL sets the PR creation URL
func (t *CommentTracker) SetPRURL(prURL string) {
	t.State.PRURL = prURL
}

// SetJobURL sets the job/workflow run URL
func (t *CommentTracker) SetJobURL(jobURL string) {
	t.State.JobURL = jobURL
}

// MarkEnd marks the end time of the task
func (t *CommentTracker) MarkEnd() {
	now := time.Now()
	t.State.EndTime = &now
}

// SetSplitPlan sets the split plan for multi-PR workflow
func (t *CommentTracker) SetSplitPlan(plan *SplitPlan) {
	t.State.SplitPlan = plan
}

// AddCreatedPR adds a created PR to the tracking list
func (t *CommentTracker) AddCreatedPR(pr CreatedPR) {
	t.State.CreatedPRs = append(t.State.CreatedPRs, pr)
}

// SetCompletedWithSplit marks the task as completed with split workflow
func (t *CommentTracker) SetCompletedWithSplit(plan *SplitPlan, createdPRs []CreatedPR, costUSD float64) {
	t.State.Status = StatusCompleted

	if plan != nil {
		if t.State.SplitPlan == nil {
			t.State.SplitPlan = plan
		}

		if len(t.State.ModifiedFiles) == 0 {
			files := collectPlanFilePaths(plan)
			if len(files) > 0 {
				sort.Strings(files)
				t.State.ModifiedFiles = files
			}
		}

		if t.State.Summary == "" {
			t.State.Summary = fmt.Sprintf("Split into %d PRs", len(plan.SubPRs))
		}
	} else if t.State.Summary == "" {
		t.State.Summary = "Split into multiple PRs"
	}

	t.State.CreatedPRs = createdPRs
	t.State.CostUSD = costUSD
}

// buildSplitPlanSection builds the split plan display section
func (t *CommentTracker) buildSplitPlanSection() string {
	plan := t.State.SplitPlan
	if plan == nil {
		return ""
	}

	var lines []string

	// Add AI-generated summary at the top if available
	if t.State.Summary != "" && t.State.Summary != fmt.Sprintf("Split into %d PRs", len(plan.SubPRs)) {
		lines = append(lines, "### 📝 Changes Summary")
		lines = append(lines, "")
		lines = append(lines, t.State.Summary)
		lines = append(lines, "")
	}

	lines = append(lines, "### 🔀 Split into Multiple PRs")
	lines = append(lines, "")

	for i, subPR := range plan.SubPRs {
		// Find corresponding created PR
		var createdPR *CreatedPR
		for j := range t.State.CreatedPRs {
			if t.State.CreatedPRs[j].Index == i {
				createdPR = &t.State.CreatedPRs[j]
				break
			}
		}

		// Calculate total lines for this sub-PR
		totalLines := 0
		for _, file := range subPR.Files {
			totalLines += strings.Count(file.Content, "\n") + 1
		}

		var status string
		if createdPR != nil && createdPR.Status == "created" {
			status = fmt.Sprintf("✅ [%s](%s) — %d files, ~%d lines", subPR.Name, createdPR.URL, len(subPR.Files), totalLines)
		} else if len(subPR.DependsOn) > 0 {
			status = fmt.Sprintf("⏳ %s — %d files, ~%d lines (waiting for dependencies)", subPR.Name, len(subPR.Files), totalLines)
		} else {
			status = fmt.Sprintf("⏳ %s — %d files, ~%d lines (pending)", subPR.Name, len(subPR.Files), totalLines)
		}

		lines = append(lines, fmt.Sprintf("%d. %s", i+1, status))
	}

	return strings.Join(lines, "\n")
}

func collectPlanFilePaths(plan *SplitPlan) []string {
	if plan == nil {
		return nil
	}

	seen := make(map[string]struct{})
	for _, subPR := range plan.SubPRs {
		for _, file := range subPR.Files {
			if file.Path == "" {
				continue
			}
			seen[file.Path] = struct{}{}
		}
	}

	if len(seen) == 0 {
		return nil
	}

	files := make([]string, 0, len(seen))
	for path := range seen {
		files = append(files, path)
	}

	return files
}

func escapeMarkdownLinkText(text string) string {
	replacer := strings.NewReplacer("[", "\\[", "]", "\\]")
	return replacer.Replace(text)
}

// AddTask adds a new task step to the progress tracker
func (t *CommentTracker) AddTask(name string) {
	t.State.Tasks = append(t.State.Tasks, TaskStep{
		Name:      name,
		Status:    "pending",
		Timestamp: time.Now(),
	})
}

// StartTask marks a task as running
func (t *CommentTracker) StartTask(name string) {
	for i, task := range t.State.Tasks {
		if task.Name == name && task.Status == "pending" {
			t.State.Tasks[i].Status = "running"
			t.State.Tasks[i].Timestamp = time.Now()
			return
		}
	}
}

// CompleteTask marks a task as completed
func (t *CommentTracker) CompleteTask(name string) {
	for i, task := range t.State.Tasks {
		if task.Name == name {
			t.State.Tasks[i].Status = "completed"
			t.State.Tasks[i].Timestamp = time.Now()
			return
		}
	}
}

// FailTask marks a task as failed
func (t *CommentTracker) FailTask(name string) {
	for i, task := range t.State.Tasks {
		if task.Name == name {
			t.State.Tasks[i].Status = "failed"
			t.State.Tasks[i].Timestamp = time.Now()
			return
		}
	}
}
