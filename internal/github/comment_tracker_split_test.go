package github

import (
	"strings"
	"testing"

	"github.com/cexll/swe/internal/provider/claude"
)

// ===== Phase 1: Split Plan Rendering Tests =====

// TestCommentTracker_BuildSplitPlanSection_WithCreatedPRs verifies display of created PRs
func TestCommentTracker_BuildSplitPlanSection_WithCreatedPRs(t *testing.T) {
	// Create split plan
	plan := &SplitPlan{
		SubPRs: []SubPR{
			{
				Index:    0,
				Name:     "Add test infrastructure",
				Category: CategoryTests,
				Files:    make([]claude.FileChange, 5),
			},
			{
				Index:    1,
				Name:     "Update documentation",
				Category: CategoryDocs,
				Files:    make([]claude.FileChange, 2),
			},
			{
				Index:    2,
				Name:     "Implement core functionality",
				Category: CategoryCore,
				Files:    make([]claude.FileChange, 8),
			},
		},
	}

	// Create PR records (first and second PRs created)
	createdPRs := []CreatedPR{
		{
			Index:      0,
			Name:       "Add test infrastructure",
			BranchName: "pilot/123-tests-1234567890",
			URL:        "https://github.com/owner/repo/compare/main...pilot/123-tests-1234567890?expand=1",
			BranchURL:  "https://github.com/owner/repo/tree/pilot/123-tests-1234567890",
			Status:     "created",
			Category:   CategoryTests,
		},
		{
			Index:      1,
			Name:       "Update documentation",
			BranchName: "pilot/123-docs-1234567891",
			URL:        "https://github.com/owner/repo/compare/main...pilot/123-docs-1234567891?expand=1",
			BranchURL:  "https://github.com/owner/repo/tree/pilot/123-docs-1234567891",
			Status:     "created",
			Category:   CategoryDocs,
		},
	}

	tracker := NewCommentTracker("owner/repo", 123, "user")
	tracker.State.SplitPlan = plan
	tracker.State.CreatedPRs = createdPRs

	// Build split plan section
	output := tracker.buildSplitPlanSection()

	// Verify header
	if !strings.Contains(output, "### 📋 Split Plan") {
		t.Error("Output should contain split plan header")
	}

	// Verify created PRs show ✅ with links
	if !strings.Contains(output, "✅ [Add test infrastructure](https://github.com/owner/repo/compare/main...pilot/123-tests-1234567890?expand=1)") {
		t.Error("Output should show created PR 1 with checkmark and link")
	}
	if !strings.Contains(output, "✅ [Update documentation](https://github.com/owner/repo/compare/main...pilot/123-docs-1234567891?expand=1)") {
		t.Error("Output should show created PR 2 with checkmark and link")
	}

	// Verify pending PR shows ⏳
	if !strings.Contains(output, "⏳ Implement core functionality (pending)") {
		t.Error("Output should show pending PR with hourglass")
	}

	// Verify file counts
	if !strings.Contains(output, "— 5 files") {
		t.Error("Output should show file count for PR 1")
	}
	if !strings.Contains(output, "— 2 files") {
		t.Error("Output should show file count for PR 2")
	}
	if !strings.Contains(output, "— 8 files") {
		t.Error("Output should show file count for PR 3")
	}
}

// TestCommentTracker_BuildSplitPlanSection_WithPendingPRs verifies display of pending PRs
func TestCommentTracker_BuildSplitPlanSection_WithPendingPRs(t *testing.T) {
	plan := &SplitPlan{
		SubPRs: []SubPR{
			{
				Index:    0,
				Name:     "Add tests",
				Category: CategoryTests,
				Files:    make([]claude.FileChange, 3),
			},
			{
				Index:    1,
				Name:     "Add docs",
				Category: CategoryDocs,
				Files:    make([]claude.FileChange, 1),
			},
		},
	}

	tracker := NewCommentTracker("owner/repo", 456, "alice")
	tracker.State.SplitPlan = plan
	tracker.State.CreatedPRs = []CreatedPR{} // No PRs created yet

	output := tracker.buildSplitPlanSection()

	// All PRs should show as pending
	if !strings.Contains(output, "1. ⏳ Add tests (pending) — 3 files") {
		t.Error("Output should show PR 1 as pending")
	}
	if !strings.Contains(output, "2. ⏳ Add docs (pending) — 1 files") {
		t.Error("Output should show PR 2 as pending")
	}
}

// TestCommentTracker_BuildSplitPlanSection_WithDependencies verifies display of dependent PRs
func TestCommentTracker_BuildSplitPlanSection_WithDependencies(t *testing.T) {
	plan := &SplitPlan{
		SubPRs: []SubPR{
			{
				Index:     0,
				Name:      "Add test infrastructure",
				Category:  CategoryTests,
				Files:     make([]claude.FileChange, 3),
				DependsOn: []int{}, // Independent
			},
			{
				Index:     1,
				Name:      "Add internal infrastructure",
				Category:  CategoryInternal,
				Files:     make([]claude.FileChange, 5),
				DependsOn: []int{0}, // Depends on tests
			},
			{
				Index:     2,
				Name:      "Implement core functionality",
				Category:  CategoryCore,
				Files:     make([]claude.FileChange, 8),
				DependsOn: []int{1}, // Depends on internal
			},
		},
	}

	createdPRs := []CreatedPR{
		{
			Index:    0,
			Name:     "Add test infrastructure",
			URL:      "https://github.com/owner/repo/pulls/1",
			Status:   "created",
			Category: CategoryTests,
		},
	}

	tracker := NewCommentTracker("owner/repo", 789, "bob")
	tracker.State.SplitPlan = plan
	tracker.State.CreatedPRs = createdPRs

	output := tracker.buildSplitPlanSection()

	// PR 1: created (independent)
	if !strings.Contains(output, "✅ [Add test infrastructure](https://github.com/owner/repo/pulls/1)") {
		t.Error("Output should show PR 1 as created")
	}

	// PR 2: waiting for dependencies
	if !strings.Contains(output, "⏳ Add internal infrastructure (waiting for dependencies)") {
		t.Error("Output should show PR 2 waiting for dependencies")
	}

	// PR 3: waiting for dependencies
	if !strings.Contains(output, "⏳ Implement core functionality (waiting for dependencies)") {
		t.Error("Output should show PR 3 waiting for dependencies")
	}
}

// TestCommentTracker_SetSplitPlan verifies split plan setting
func TestCommentTracker_SetSplitPlan(t *testing.T) {
	tracker := NewCommentTracker("owner/repo", 100, "user")

	if tracker.State.SplitPlan != nil {
		t.Error("SplitPlan should be nil initially")
	}

	plan := &SplitPlan{
		SubPRs: []SubPR{
			{Index: 0, Name: "PR 1"},
			{Index: 1, Name: "PR 2"},
		},
		TotalFiles: 10,
		TotalLines: 200,
	}

	tracker.SetSplitPlan(plan)

	if tracker.State.SplitPlan == nil {
		t.Fatal("SplitPlan should not be nil after SetSplitPlan")
	}
	if len(tracker.State.SplitPlan.SubPRs) != 2 {
		t.Errorf("SubPRs length = %d, want 2", len(tracker.State.SplitPlan.SubPRs))
	}
	if tracker.State.SplitPlan.TotalFiles != 10 {
		t.Errorf("TotalFiles = %d, want 10", tracker.State.SplitPlan.TotalFiles)
	}
	if tracker.State.SplitPlan.TotalLines != 200 {
		t.Errorf("TotalLines = %d, want 200", tracker.State.SplitPlan.TotalLines)
	}
}

// TestCommentTracker_AddCreatedPR verifies PR record appending
func TestCommentTracker_AddCreatedPR(t *testing.T) {
	tracker := NewCommentTracker("owner/repo", 200, "user")

	if len(tracker.State.CreatedPRs) != 0 {
		t.Error("CreatedPRs should be empty initially")
	}

	pr1 := CreatedPR{
		Index:      0,
		Name:       "First PR",
		BranchName: "pilot/200-tests-111",
		URL:        "https://github.com/owner/repo/pulls/1",
		Status:     "created",
		Category:   CategoryTests,
	}

	tracker.AddCreatedPR(pr1)

	if len(tracker.State.CreatedPRs) != 1 {
		t.Fatalf("CreatedPRs length = %d, want 1", len(tracker.State.CreatedPRs))
	}
	if tracker.State.CreatedPRs[0].Name != "First PR" {
		t.Errorf("PR name = %s, want First PR", tracker.State.CreatedPRs[0].Name)
	}
	if tracker.State.CreatedPRs[0].URL != "https://github.com/owner/repo/pulls/1" {
		t.Errorf("PR URL = %s, want specific URL", tracker.State.CreatedPRs[0].URL)
	}

	pr2 := CreatedPR{
		Index:      1,
		Name:       "Second PR",
		BranchName: "pilot/200-docs-222",
		URL:        "https://github.com/owner/repo/pulls/2",
		Status:     "created",
		Category:   CategoryDocs,
	}

	tracker.AddCreatedPR(pr2)

	if len(tracker.State.CreatedPRs) != 2 {
		t.Errorf("CreatedPRs length = %d, want 2 after adding second PR", len(tracker.State.CreatedPRs))
	}
	if tracker.State.CreatedPRs[1].Name != "Second PR" {
		t.Errorf("Second PR name = %s, want Second PR", tracker.State.CreatedPRs[1].Name)
	}
}

// TestCommentTracker_SetCompletedWithSplit verifies completion status with split workflow
func TestCommentTracker_SetCompletedWithSplit(t *testing.T) {
	tracker := NewCommentTracker("owner/repo", 300, "user")

	plan := &SplitPlan{
		SubPRs: []SubPR{
			{Index: 0, Name: "PR 1"},
			{Index: 1, Name: "PR 2"},
			{Index: 2, Name: "PR 3"},
		},
	}

	createdPRs := []CreatedPR{
		{Index: 0, Name: "PR 1", Status: "created"},
		{Index: 1, Name: "PR 2", Status: "created"},
		{Index: 2, Name: "PR 3", Status: "created"},
	}

	tracker.SetCompletedWithSplit(plan, createdPRs, 0.15)

	if tracker.State.Status != StatusCompleted {
		t.Errorf("Status = %v, want StatusCompleted", tracker.State.Status)
	}
	if tracker.State.Summary != "Split into 3 PRs" {
		t.Errorf("Summary = %q, want 'Split into 3 PRs'", tracker.State.Summary)
	}
	if len(tracker.State.CreatedPRs) != 3 {
		t.Errorf("CreatedPRs length = %d, want 3", len(tracker.State.CreatedPRs))
	}
	if tracker.State.CostUSD != 0.15 {
		t.Errorf("CostUSD = %.4f, want 0.15", tracker.State.CostUSD)
	}
}

// TestCommentTracker_RenderBody_WithSplitPlan verifies rendering with split plan
func TestCommentTracker_RenderBody_WithSplitPlan(t *testing.T) {
	tracker := NewCommentTracker("owner/repo", 400, "testuser")

	plan := &SplitPlan{
		SubPRs: []SubPR{
			{
				Index:    0,
				Name:     "Add tests",
				Files:    make([]claude.FileChange, 3),
				Category: CategoryTests,
			},
			{
				Index:    1,
				Name:     "Add docs",
				Files:    make([]claude.FileChange, 2),
				Category: CategoryDocs,
			},
		},
	}

	createdPRs := []CreatedPR{
		{
			Index:    0,
			Name:     "Add tests",
			URL:      "https://github.com/owner/repo/pulls/1",
			Status:   "created",
			Category: CategoryTests,
		},
	}

	tracker.State.SplitPlan = plan
	tracker.State.CreatedPRs = createdPRs
	tracker.State.OriginalBody = "Please add tests and docs"
	tracker.SetCompletedWithSplit(plan, createdPRs, 0.08)
	tracker.MarkEnd()

	body := tracker.renderBody()

	// Verify split plan section is included
	if !strings.Contains(body, "### 📋 Split Plan") {
		t.Error("Body should contain split plan section")
	}

	// Verify created PR is shown
	if !strings.Contains(body, "✅ [Add tests]") {
		t.Error("Body should show created PR")
	}

	// Verify pending PR is shown
	if !strings.Contains(body, "⏳ Add docs (pending)") {
		t.Error("Body should show pending PR")
	}

	// Verify summary is shown
	if !strings.Contains(body, "**Summary:** Split into 2 PRs") {
		t.Error("Body should contain summary")
	}

	// Verify modified files list is NOT shown (split workflow)
	if strings.Contains(body, "**Modified Files:**") {
		t.Error("Body should not contain modified files list in split workflow")
	}

	// Verify cost is shown
	if !strings.Contains(body, "Cost: $0.0800") {
		t.Error("Body should show cost")
	}
}

// TestCommentTracker_BuildSplitPlanSection_EmptyPlan verifies handling of nil plan
func TestCommentTracker_BuildSplitPlanSection_EmptyPlan(t *testing.T) {
	tracker := NewCommentTracker("owner/repo", 500, "user")
	tracker.State.SplitPlan = nil

	output := tracker.buildSplitPlanSection()

	if output != "" {
		t.Errorf("Output should be empty for nil plan, got: %s", output)
	}
}
