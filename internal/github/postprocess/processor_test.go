package postprocess

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-github/v66/github"
)

func TestProcessor_NoBranch_Noop(t *testing.T) {
	// Stub helpers
	calledDelete := false
	calledUpdate := false
	checkBranchStatus = func(ctx context.Context, client *github.Client, owner, repo, branch, baseBranch string) (*BranchStatus, error) {
		return &BranchStatus{Exists: false}, nil
	}
	deleteBranch = func(ctx context.Context, client *github.Client, owner, repo, branch string) error {
		calledDelete = true
		return nil
	}
	updateCommentWithLinks = func(ctx context.Context, client *github.Client, owner, repo string, commentID int64, branchLink, prLink string) error {
		calledUpdate = true
		return nil
	}

	p := NewProcessor(&github.Client{}, "o", "r", 123, "feature", "main", 1, false)
	if err := p.Process(context.Background()); err != nil {
		t.Fatalf("Process error: %v", err)
	}
	if calledDelete {
		t.Fatalf("unexpected delete branch call")
	}
	if calledUpdate {
		t.Fatalf("unexpected update comment call")
	}
}

func TestProcessor_EmptyBranch_Delete(t *testing.T) {
	calledDelete := false
	calledUpdate := false
	checkBranchStatus = func(ctx context.Context, client *github.Client, owner, repo, branch, baseBranch string) (*BranchStatus, error) {
		return &BranchStatus{Exists: true, HasCommits: false, TotalCommits: 0, FilesChanged: 0}, nil
	}
	deleteBranch = func(ctx context.Context, client *github.Client, owner, repo, branch string) error {
		calledDelete = true
		return nil
	}
	updateCommentWithLinks = func(ctx context.Context, client *github.Client, owner, repo string, commentID int64, branchLink, prLink string) error {
		calledUpdate = true
		return nil
	}

	p := NewProcessor(&github.Client{}, "o", "r", 456, "feature2", "main", 2, true)
	if err := p.Process(context.Background()); err != nil {
		t.Fatalf("Process error: %v", err)
	}
	if !calledDelete {
		t.Fatalf("expected delete branch to be called")
	}
	if calledUpdate {
		t.Fatalf("unexpected update comment call for empty branch")
	}
}

func TestProcessor_WithChanges_UpdateComment(t *testing.T) {
	calledDelete := false
	calledUpdate := false
	var gotBranchLink, gotPRLink string
	checkBranchStatus = func(ctx context.Context, client *github.Client, owner, repo, branch, baseBranch string) (*BranchStatus, error) {
		return &BranchStatus{Exists: true, HasCommits: true, TotalCommits: 1, FilesChanged: 1}, nil
	}
	deleteBranch = func(ctx context.Context, client *github.Client, owner, repo, branch string) error {
		calledDelete = true
		return nil
	}
	updateCommentWithLinks = func(ctx context.Context, client *github.Client, owner, repo string, commentID int64, branchLink, prLink string) error {
		calledUpdate = true
		gotBranchLink, gotPRLink = branchLink, prLink
		return nil
	}

	p := NewProcessor(&github.Client{}, "owner", "repo", 789, "feat", "main", 42, true)
	if err := p.Process(context.Background()); err != nil {
		t.Fatalf("Process error: %v", err)
	}
	if calledDelete {
		t.Fatalf("delete branch should not be called when changes present")
	}
	if !calledUpdate {
		t.Fatalf("expected update comment to be called")
	}
	if gotBranchLink == "" || gotPRLink == "" {
		t.Fatalf("expected non-empty links, got branch=%q pr=%q", gotBranchLink, gotPRLink)
	}
}

func TestProcessor_ValidationErrors(t *testing.T) {
	p := NewProcessor(nil, "o", "r", 1, "b", "main", 1, false)
	if err := p.Process(context.Background()); err == nil || !strings.Contains(err.Error(), "nil github client") {
		t.Fatalf("expected nil client error, got %v", err)
	}
	p = NewProcessor(&github.Client{}, "", "r", 1, "b", "main", 1, false)
	if err := p.Process(context.Background()); err == nil || !strings.Contains(err.Error(), "missing owner/repo/branch") {
		t.Fatalf("expected missing fields error, got %v", err)
	}
}

func TestProcessor_WithChanges_NoCommentID_NoUpdate(t *testing.T) {
	calledUpdate := false
	checkBranchStatus = func(ctx context.Context, client *github.Client, owner, repo, branch, baseBranch string) (*BranchStatus, error) {
		return &BranchStatus{Exists: true, HasCommits: true, TotalCommits: 1, FilesChanged: 1}, nil
	}
	updateCommentWithLinks = func(ctx context.Context, client *github.Client, owner, repo string, commentID int64, branchLink, prLink string) error {
		calledUpdate = true
		return nil
	}
	p := NewProcessor(&github.Client{}, "o", "r", 0, "b", "main", 2, false)
	if err := p.Process(context.Background()); err != nil {
		t.Fatalf("Process error: %v", err)
	}
	if calledUpdate {
		t.Fatalf("should not update when commentID is 0")
	}
}
