package github

import (
	"fmt"
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

	// Build header based on status
	header := t.buildHeader()

	// Build links section
	links := t.buildLinks()

	// Build body sections
	var sections []string

	// Add header with links
	if links != "" {
		sections = append(sections, header+" "+links)
	} else {
		sections = append(sections, header)
	}

	// Add separator
	sections = append(sections, "---")

	// Add original request if available
	if state.OriginalBody != "" {
		sections = append(sections, state.OriginalBody)
	}

	// Add split plan section if present
	if state.SplitPlan != nil {
		sections = append(sections, "", t.buildSplitPlanSection())
	}

	// Add summary for completed tasks
	if state.IsCompleted() && state.Summary != "" {
		sections = append(sections, "", "**Summary:** "+state.Summary)

		// Add modified files if available (only for non-split workflows)
		if len(state.ModifiedFiles) > 0 && state.SplitPlan == nil {
			sections = append(sections, "", t.buildModifiedFilesList())
		}
	}

	// Add error details for failed tasks
	if state.IsFailed() && state.ErrorDetails != "" {
		sections = append(sections, "", "```", state.ErrorDetails, "```")
	}

	// Add footer
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
		return fmt.Sprintf("⏳ **Pilot queued @%s's task...**", username)
	case StatusWorking:
		return fmt.Sprintf("🤖 **Pilot is working on @%s's task...**", username)

	case StatusCompleted:
		duration := state.Duration()
		if duration != "" {
			return fmt.Sprintf("✅ **Pilot finished @%s's task in %s**", username, duration)
		}
		return fmt.Sprintf("✅ **Pilot finished @%s's task**", username)

	case StatusFailed:
		duration := state.Duration()
		if duration != "" {
			return fmt.Sprintf("❌ **Pilot encountered an error after %s**", duration)
		}
		return "❌ **Pilot encountered an error**"

	default:
		return "**Pilot Task Status**"
	}
}

// buildLinks builds the links section (job, branch, PR)
func (t *CommentTracker) buildLinks() string {
	state := t.State
	var links []string

	// Add branch link
	if state.BranchName != "" {
		if state.BranchURL != "" {
			links = append(links, fmt.Sprintf("[`%s`](%s)", state.BranchName, state.BranchURL))
		} else {
			links = append(links, fmt.Sprintf("`%s`", state.BranchName))
		}
	}

	// Add PR link
	if state.PRURL != "" {
		links = append(links, fmt.Sprintf("[Create PR ➔](%s)", state.PRURL))
	}

	// Add job link
	if state.JobURL != "" {
		links = append(links, fmt.Sprintf("[View job](%s)", state.JobURL))
	}

	if len(links) == 0 {
		return ""
	}

	// Format: —— link1 • link2 • link3
	return "—— " + strings.Join(links, " • ")
}

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
		return fmt.Sprintf("*Generated by Pilot SWE • Cost: $%.4f*", state.CostUSD)
	}

	return "*Generated by Pilot SWE*"
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
	t.State.Summary = fmt.Sprintf("Split into %d PRs", len(plan.SubPRs))
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
