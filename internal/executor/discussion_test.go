package executor

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/webhook"
)

func TestExecutor_ComposeDiscussionSection_PRDiscussion(t *testing.T) {
	now := time.Date(2025, 10, 13, 12, 0, 0, 0, time.UTC)

	mockGH := github.NewMockGHClient()
	mockGH.ListIssueCommentsFunc = func(repo string, number int, token string) ([]github.IssueComment, error) {
		return []github.IssueComment{
			{Author: "alice", Body: "  looks good  ", CreatedAt: now},
			{Author: "", Body: "needs more tests", CreatedAt: now.Add(2 * time.Minute)},
		}, nil
	}
	mockGH.ListReviewCommentsFunc = func(repo string, number int, token string) ([]github.ReviewComment, error) {
		return []github.ReviewComment{
			{Author: "bob", Body: "  inline nit  ", Path: "pkg/service.go", DiffHunk: "+func()", CreatedAt: now.Add(1 * time.Minute)},
			{Author: "", Body: " final thoughts ", CreatedAt: time.Time{}},
		}, nil
	}

	executor := &Executor{ghClient: mockGH}
	task := &webhook.Task{
		Repo:   "owner/repo",
		Number: 7,
		IsPR:   true,
	}

	result := executor.composeDiscussionSection(task, "token")
	if result == "" {
		t.Fatalf("composeDiscussionSection returned empty discussion")
	}
	if !strings.Contains(result, "@alice") || !strings.Contains(result, "@unknown") {
		t.Fatalf("discussion should include authors, got:\n%s", result)
	}
	if !strings.Contains(result, "_File: pkg/service.go_") {
		t.Fatalf("discussion should include file metadata, got:\n%s", result)
	}
	if !strings.Contains(result, "```diff") {
		t.Fatalf("discussion should include diff hunk, got:\n%s", result)
	}
	if !strings.Contains(result, now.UTC().Format(time.RFC3339)) {
		t.Fatalf("discussion should include timestamp, got:\n%s", result)
	}
}

func TestExecutor_ComposeDiscussionSection_NonPRSkipsReviews(t *testing.T) {
	mockGH := github.NewMockGHClient()
	mockGH.ListIssueCommentsFunc = func(repo string, number int, token string) ([]github.IssueComment, error) {
		return []github.IssueComment{
			{Author: "alice", Body: "Issue update"},
		}, nil
	}
	called := false
	mockGH.ListReviewCommentsFunc = func(repo string, number int, token string) ([]github.ReviewComment, error) {
		called = true
		return nil, nil
	}

	executor := &Executor{ghClient: mockGH}
	task := &webhook.Task{
		Repo:   "owner/repo",
		Number: 42,
		IsPR:   false,
	}

	result := executor.composeDiscussionSection(task, "token")
	if called {
		t.Fatal("ListReviewComments should not be called for non-PR tasks")
	}
	if !strings.Contains(result, "@alice") {
		t.Fatalf("discussion should include issue comment, got:\n%s", result)
	}
}

func TestExecutor_ComposeDiscussionSection_ErrorFallback(t *testing.T) {
	mockGH := github.NewMockGHClient()
	mockGH.ListIssueCommentsFunc = func(repo string, number int, token string) ([]github.IssueComment, error) {
		return nil, errors.New("boom")
	}
	mockGH.ListReviewCommentsFunc = func(repo string, number int, token string) ([]github.ReviewComment, error) {
		return nil, errors.New("boom")
	}

	executor := &Executor{ghClient: mockGH}
	task := &webhook.Task{
		Repo:   "owner/repo",
		Number: 1,
		IsPR:   true,
	}

	if discussion := executor.composeDiscussionSection(task, "token"); discussion != "" {
		t.Fatalf("discussion = %q, want empty string on fetch errors", discussion)
	}
}

func TestFormatDiscussion_OrderingAndDefaults(t *testing.T) {
	now := time.Now()
	issue := []github.IssueComment{
		{Author: "", Body: "first zero time"}, // zero timestamp, author default
		{Author: "carol", Body: "   "},        // blank body skipped
		{Author: "dave", Body: "later", CreatedAt: now.Add(time.Hour)},
	}
	reviews := []github.ReviewComment{
		{Author: "", Body: "zero review", CreatedAt: time.Time{}}, // triggers unknown author + zero time
		{Author: "bob", Body: "with path", Path: "file.go", DiffHunk: "+change", CreatedAt: now},
	}

	result := formatDiscussion(issue, reviews)
	if !strings.HasPrefix(result, "## Discussion") {
		t.Fatalf("formatDiscussion should include header, got:\n%s", result)
	}
	if !strings.Contains(result, "@unknown (unknown):") {
		t.Fatalf("expected unknown author entry, got:\n%s", result)
	}
	if !strings.Contains(result, "with path") {
		t.Fatalf("expected review body, got:\n%s", result)
	}
	if !strings.Contains(result, "```diff") {
		t.Fatalf("expected diff block, got:\n%s", result)
	}
}

func TestFormatDiscussion_Empty(t *testing.T) {
	if got := formatDiscussion(nil, nil); got != "" {
		t.Fatalf("formatDiscussion(nil,nil) = %q, want empty", got)
	}
}
