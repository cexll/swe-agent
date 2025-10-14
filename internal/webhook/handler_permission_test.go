package webhook

import (
	"errors"
	"testing"

	"github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/taskstore"
)

type stubAuthProvider struct {
	owner string
	err   error
}

func (s *stubAuthProvider) GetInstallationToken(repo string) (*github.InstallationToken, error) {
	return nil, nil
}

func (s *stubAuthProvider) GetInstallationOwner(repo string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.owner, nil
}

func TestHandlerVerifyPermission(t *testing.T) {
	t.Run("no auth provider allows all", func(t *testing.T) {
		h := &Handler{}
		if !h.verifyPermission("owner/repo", "someone") {
			t.Fatal("verifyPermission should allow when appAuth is nil")
		}
	})

	t.Run("matching installer passes", func(t *testing.T) {
		h := &Handler{appAuth: &stubAuthProvider{owner: "installer"}}
		if !h.verifyPermission("owner/repo", "installer") {
			t.Fatal("expected permission check to pass for installer")
		}
	})

	t.Run("mismatched installer fails", func(t *testing.T) {
		h := &Handler{appAuth: &stubAuthProvider{owner: "installer"}}
		if h.verifyPermission("owner/repo", "contributor") {
			t.Fatal("expected permission check to fail for non-installer")
		}
	})

	t.Run("fail open on error", func(t *testing.T) {
		h := &Handler{appAuth: &stubAuthProvider{err: errors.New("boom")}}
		if !h.verifyPermission("owner/repo", "anyone") {
			t.Fatal("verifyPermission should fail-open on auth errors")
		}
	})
}

func TestHandlerCreateStoreTask(t *testing.T) {
	store := taskstore.NewStore()
	h := &Handler{store: store}

	task := &Task{
		ID:         "task-1",
		Repo:       "owner/repo",
		Number:     42,
		IssueTitle: "Example",
		Username:   "alice",
	}
	h.createStoreTask(task)

	got, ok := store.Get("task-1")
	if !ok {
		t.Fatal("stored task not found")
	}
	if got.RepoOwner != "owner" || got.RepoName != "repo" {
		t.Fatalf("unexpected repo split: %s/%s", got.RepoOwner, got.RepoName)
	}
	if len(got.Logs) != 1 || got.Logs[0].Message != "Task queued" {
		t.Fatalf("expected Task queued log, got %+v", got.Logs)
	}

	// Cover splitRepo fallback (no slash)
	task2 := &Task{
		ID:         "task-2",
		Repo:       "solo",
		Number:     7,
		IssueTitle: "Single",
		Username:   "bob",
	}
	h.createStoreTask(task2)

	got2, ok := store.Get("task-2")
	if !ok {
		t.Fatal("second task missing")
	}
	if got2.RepoOwner != "solo" || got2.RepoName != "" {
		t.Fatalf("expected fallback owner only, got %s/%s", got2.RepoOwner, got2.RepoName)
	}
}

func TestTruncateText(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		limit int
		want  string
	}{
		{
			name:  "limit zero",
			text:  "something",
			limit: 0,
			want:  "",
		},
		{
			name:  "under limit",
			text:  "short",
			limit: 10,
			want:  "short",
		},
		{
			name:  "over limit",
			text:  "this sentence is too long",
			limit: 10,
			want:  "this sente…",
		},
		{
			name:  "trailing whitespace trimmed",
			text:  "   padded text   ",
			limit: 6,
			want:  "padded…",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := truncateText(tc.text, tc.limit); got != tc.want {
				t.Fatalf("truncateText(%q, %d) = %q, want %q", tc.text, tc.limit, got, tc.want)
			}
		})
	}
}
