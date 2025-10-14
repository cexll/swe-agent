package github

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func newRealClientWithRunner(r *MockCommandRunner) *RealGHClient {
	return &RealGHClient{runner: r}
}

func TestRealGHClient_UpdateComment(t *testing.T) {
	originalToken := "orig-token"
	if err := os.Setenv("GITHUB_TOKEN", originalToken); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	runner := NewMockCommandRunner()
	runner.RunFunc = func(name string, args ...string) ([]byte, error) {
		expect := []string{
			"api",
			"/repos/owner/repo/issues/comments/99",
			"-X", "PATCH",
			"-f", "body=updated body",
		}
		if name != "gh" || !reflect.DeepEqual(args, expect) {
			t.Fatalf("unexpected command invocation: %s %v", name, args)
		}
		return []byte(""), nil
	}

	client := newRealClientWithRunner(runner)
	if err := client.UpdateComment("owner/repo", 99, "updated body", "test-token"); err != nil {
		t.Fatalf("UpdateComment error: %v", err)
	}

	if os.Getenv("GITHUB_TOKEN") != originalToken {
		t.Fatalf("expected GITHUB_TOKEN restored to %q, got %q", originalToken, os.Getenv("GITHUB_TOKEN"))
	}
}

func TestRealGHClient_GetCommentBody(t *testing.T) {
	runner := NewMockCommandRunner()
	runner.RunFunc = func(name string, args ...string) ([]byte, error) {
		return []byte(`{"body":"hello world"}`), nil
	}

	client := newRealClientWithRunner(runner)
	body, err := client.GetCommentBody("owner/repo", 1, "token")
	if err != nil {
		t.Fatalf("GetCommentBody error: %v", err)
	}
	if body != "hello world" {
		t.Fatalf("body = %q, want %q", body, "hello world")
	}
}

func TestRealGHClient_ListIssueComments(t *testing.T) {
	runner := NewMockCommandRunner()
	runner.RunFunc = func(name string, args ...string) ([]byte, error) {
		return []byte(`[
			{"body":"comment","created_at":"2025-10-10T10:00:00Z","user":{"login":"alice"}}
		]`), nil
	}

	client := newRealClientWithRunner(runner)
	comments, err := client.ListIssueComments("owner/repo", 1, "token")
	if err != nil {
		t.Fatalf("ListIssueComments error: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if comments[0].Author != "alice" || comments[0].Body != "comment" {
		t.Fatalf("unexpected comment: %+v", comments[0])
	}
	if !comments[0].CreatedAt.Equal(time.Date(2025, 10, 10, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("createdAt mismatch: %v", comments[0].CreatedAt)
	}
}

func TestRealGHClient_ListReviewComments(t *testing.T) {
	runner := NewMockCommandRunner()
	runner.RunFunc = func(name string, args ...string) ([]byte, error) {
		return []byte(`[
			{"body":"nit","path":"main.go","diff_hunk":"@@","created_at":"2025-10-10T11:00:00Z","user":{"login":"bob"}}
		]`), nil
	}

	client := newRealClientWithRunner(runner)
	comments, err := client.ListReviewComments("owner/repo", 2, "token")
	if err != nil {
		t.Fatalf("ListReviewComments error: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if comments[0].Author != "bob" || comments[0].Path != "main.go" || comments[0].DiffHunk != "@@" {
		t.Fatalf("unexpected review comment: %+v", comments[0])
	}
}

func TestRealGHClient_AddLabel(t *testing.T) {
	runner := NewMockCommandRunner()
	runner.RunFunc = func(name string, args ...string) ([]byte, error) {
		expectArgs := []string{"issue", "edit", "7", "--repo", "owner/repo", "--add-label", "bug"}
		if !reflect.DeepEqual(args, expectArgs) {
			t.Fatalf("unexpected args: %v", args)
		}
		return []byte(""), nil
	}

	client := newRealClientWithRunner(runner)
	if err := client.AddLabel("owner/repo", 7, "bug", "token"); err != nil {
		t.Fatalf("AddLabel error: %v", err)
	}
}

func TestRealGHClient_Clone(t *testing.T) {
	runner := NewMockCommandRunner()
	runner.RunFunc = func(name string, args ...string) ([]byte, error) {
		if name != "gh" || !strings.Contains(strings.Join(args, " "), "repo clone owner/repo /tmp/path -- -b main") {
			t.Fatalf("unexpected clone invocation: %s %v", name, args)
		}
		return []byte("cloned"), nil
	}

	client := newRealClientWithRunner(runner)
	if err := client.Clone("owner/repo", "main", "/tmp/path"); err != nil {
		t.Fatalf("Clone error: %v", err)
	}
}

func TestRealGHClient_CreatePR(t *testing.T) {
	runner := NewMockCommandRunner()
	runner.RunInDirFunc = func(dir, name string, args ...string) ([]byte, error) {
		if dir != "/workdir" {
			t.Fatalf("unexpected dir %q", dir)
		}
		expectArgs := []string{
			"pr", "create",
			"--repo", "owner/repo",
			"--head", "feature",
			"--base", "main",
			"--title", "Add feature",
			"--body", "Details",
		}
		if !reflect.DeepEqual(args, expectArgs) {
			t.Fatalf("unexpected args: %v", args)
		}
		return []byte("https://github.com/owner/repo/pull/1"), nil
	}

	client := newRealClientWithRunner(runner)
	url, err := client.CreatePR("/workdir", "owner/repo", "feature", "main", "Add feature", "Details")
	if err != nil {
		t.Fatalf("CreatePR error: %v", err)
	}
	if url != "https://github.com/owner/repo/pull/1" {
		t.Fatalf("unexpected url: %s", url)
	}
}
