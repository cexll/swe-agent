package postprocess

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	gh "github.com/google/go-github/v66/github"
)

// mockTransport redirects api.github.com to test server.
type mockTransport struct {
	base *url.URL
	c    *http.Client
}

func (mt mockTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.URL.Scheme, r.URL.Host = mt.base.Scheme, mt.base.Host
	r.Host = mt.base.Host
	return mt.c.Transport.RoundTrip(r)
}

func TestCommentUpdater_UpdateCommentWithLinks(t *testing.T) {
	mux := http.NewServeMux()

	var updated string
	mux.HandleFunc("/repos/o/r/issues/comments/1001", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 1001, "body": "Initial body"})
		case http.MethodPatch:
			var req gh.IssueComment
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if req.Body == nil {
				t.Fatalf("nil body in edit request")
			}
			updated = *req.Body
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 1001, "body": updated})
		default:
			http.Error(w, "method", http.StatusMethodNotAllowed)
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	base, _ := url.Parse(srv.URL)
	old := http.DefaultTransport
	http.DefaultTransport = mockTransport{base: base, c: srv.Client()}
	defer func() { http.DefaultTransport = old }()

	client := gh.NewClient(nil)
	cu := NewCommentUpdater(client, "o", "r")

	branchLink := "\n[View branch](https://example.com/branch)"
	prLink := "\n[Create a PR](https://example.com/pr)"
	if err := cu.UpdateCommentWithLinks(context.Background(), 1001, branchLink, prLink); err != nil {
		t.Fatalf("UpdateCommentWithLinks error: %v", err)
	}
	if updated == "" || updated == "Initial body" {
		t.Fatalf("expected updated body, got: %q", updated)
	}
}

func TestCommentUpdater_NoDuplicateLinks(t *testing.T) {
	mux := http.NewServeMux()
	// comment already has links; PATCH should not be called
	mux.HandleFunc("/repos/o/r/issues/comments/2002", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 2002, "body": "Hello\n[View branch](x)\n[Create a PR](y)"})
			return
		}
		if r.Method == http.MethodPatch {
			http.Error(w, "should not patch", http.StatusInternalServerError)
			return
		}
		http.Error(w, "method", http.StatusMethodNotAllowed)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	base, _ := url.Parse(srv.URL)
	old := http.DefaultTransport
	http.DefaultTransport = mockTransport{base: base, c: srv.Client()}
	defer func() { http.DefaultTransport = old }()

	client := gh.NewClient(nil)
	cu := NewCommentUpdater(client, "o", "r")
	if err := cu.UpdateCommentWithLinks(context.Background(), 2002, "\n[View branch](x)", "\n[Create a PR](y)"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCommentUpdater_APIErrors(t *testing.T) {
	mux := http.NewServeMux()
	// 404 on get
	mux.HandleFunc("/repos/o/r/issues/comments/3003", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	base, _ := url.Parse(srv.URL)
	old := http.DefaultTransport
	http.DefaultTransport = mockTransport{base: base, c: srv.Client()}
	defer func() { http.DefaultTransport = old }()

	client := gh.NewClient(nil)
	cu := NewCommentUpdater(client, "o", "r")
	if err := cu.UpdateCommentWithLinks(context.Background(), 3003, "", ""); err == nil {
		t.Fatalf("expected error on GetComment")
	}
}
