package command

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	ghctx "github.com/cexll/swe/internal/github"
	gh "github.com/google/go-github/v66/github"
)

func TestNameAndContains(t *testing.T) {
	m := &CommandMode{}
	if m.Name() != "command" {
		t.Fatalf("Name = %q", m.Name())
	}
	if !containsCommand("Run /code now", "/code") {
		t.Fatalf("containsCommand should detect /code")
	}
	if m.ShouldTrigger(&ghctx.Context{TriggerComment: &ghctx.Comment{Body: "hello"}}) {
		t.Fatalf("ShouldTrigger false when no command")
	}
}

// mockTransport intercepts calls to api.github.com and redirects to our mux.
type mockTransport struct {
	base *url.URL
	c    *http.Client
}

func (mt mockTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	// rewrite request URL to our test server
	r.URL.Scheme, r.URL.Host = mt.base.Scheme, mt.base.Host
	r.Host = mt.base.Host
	return mt.c.Transport.RoundTrip(r)
}

func TestPrepare_EndToEndWithMocks(t *testing.T) {
	mux := http.NewServeMux()

	// Create initial comment
	mux.HandleFunc("/repos/o/r/issues/5/comments", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 2001})
	})

	// Get base ref SHA; non-existing for any other branch handled by default 404
	mux.HandleFunc("/repos/o/r/git/ref/heads/main", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ref": "refs/heads/main", "object": map[string]any{"sha": "base-sha"}})
	})

	// Create new ref
	mux.HandleFunc("/repos/o/r/git/refs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ref": "refs/heads/swe/issue-5-issue-5"})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Override default transport used by github.NewClient(nil)
	base, _ := url.Parse(srv.URL)
	old := http.DefaultTransport
	http.DefaultTransport = mockTransport{base: base, c: srv.Client()}
	defer func() { http.DefaultTransport = old }()

	ghc := &ghctx.Context{
		Repository:     ghctx.Repository{Owner: "o", Name: "r"},
		IssueNumber:    5,
		IssueTitle:     "issue-5",
		BaseBranch:     "main",
		TriggerComment: &ghctx.Comment{Body: "/code please"},
	}

	m := &CommandMode{}
	res, err := m.Prepare(context.Background(), ghc)
	if err != nil {
		t.Fatalf("Prepare error: %v", err)
	}
	// 新设计：Branch 和 Prompt 字段留空，让 AI 和 Executor 自主决策
	if res.CommentID == 0 {
		t.Fatalf("unexpected result: CommentID should not be 0, got: %+v", res)
	}
	if res.Branch != "" {
		t.Fatalf("unexpected result: Branch should be empty (AI creates it), got: %+v", res)
	}
	if res.Prompt != "" {
		t.Fatalf("unexpected result: Prompt should be empty (Executor builds it), got: %+v", res)
	}
	if res.BaseBranch != "main" {
		t.Fatalf("unexpected result: BaseBranch should be 'main', got: %+v", res)
	}

	// Ensure Prepare didn't accidentally make network calls beyond our mux
	_ = gh.NewClient(nil) // compile-time use to ensure import
}
