package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	gh "github.com/google/go-github/v66/github"
)

func TestCommitFilesREST_CreatesBranchAndCommits(t *testing.T) {
	mux := http.NewServeMux()

	// 1) Branch ref is missing -> 404
	mux.HandleFunc("/repos/o/r/git/refs/heads/feat", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			http.NotFound(w, r)
		case http.MethodPatch:
			_ = json.NewEncoder(w).Encode(map[string]any{"ref": "refs/heads/feat"})
		default:
			http.Error(w, "method", http.StatusMethodNotAllowed)
		}
	})

	// 2) Repo info -> default branch
	mux.HandleFunc("/repos/o/r", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"default_branch": "main"})
	})

	// 3) Default branch ref
	mux.HandleFunc("/repos/o/r/git/refs/heads/main", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"object": map[string]any{"sha": "base-sha"}})
	})

	// 4) Create branch
	mux.HandleFunc("/repos/o/r/git/refs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ref": "refs/heads/feat"})
	})

	// 5) Get base commit
	mux.HandleFunc("/repos/o/r/git/commits/base-sha", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"tree": map[string]any{"sha": "tree-sha"}})
	})

	// 6) Create tree
	mux.HandleFunc("/repos/o/r/git/trees", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"sha": "new-tree-sha"})
	})

	// 7) Create commit
	mux.HandleFunc("/repos/o/r/git/commits", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"sha": "new-commit-sha"})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := gh.NewClient(srv.Client())
	base, _ := url.Parse(srv.URL + "/")
	client.BaseURL = base

	sha, err := CommitFiles(context.Background(), client, CommitFilesOptions{
		Owner:   "o",
		Repo:    "r",
		Branch:  "feat",
		Message: "m",
		Files:   map[string]string{"a.txt": "hello"},
	})
	if err != nil {
		t.Fatalf("CommitFiles error: %v", err)
	}
	if sha != "new-commit-sha" {
		t.Fatalf("commit sha = %q", sha)
	}
}
