package postprocess

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	gh "github.com/google/go-github/v66/github"
)

func setupPostprocessServer(t *testing.T, totalCommits int, filesChanged int) (*httptest.Server, *gh.Client) {
	t.Helper()
	mux := http.NewServeMux()

	// GET branch: /repos/o/r/branches/{branch}
	mux.HandleFunc("/repos/o/r/branches/", func(w http.ResponseWriter, r *http.Request) {
		br := strings.TrimPrefix(r.URL.Path, "/repos/o/r/branches/")
		if br == "missing" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"name": br})
	})

	// GET compare: /repos/o/r/compare/{base}...{head}
	mux.HandleFunc("/repos/o/r/compare/", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_commits": totalCommits,
			"files":         make([]any, filesChanged),
		})
	})

	// DELETE ref
	mux.HandleFunc("/repos/o/r/git/refs/heads/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(mux)
	client := gh.NewClient(srv.Client())
	base, _ := url.Parse(srv.URL + "/")
	client.BaseURL = base
	return srv, client
}

func TestCheckBranchStatus_NoBranch(t *testing.T) {
	srv, client := setupPostprocessServer(t, 0, 0)
	defer srv.Close()
	st, err := CheckBranchStatus(context.Background(), client, "o", "r", "missing", "main")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if st.Exists {
		t.Fatalf("expected Exists=false for missing branch")
	}
}

func TestCheckBranchStatus_WithChanges(t *testing.T) {
	srv, client := setupPostprocessServer(t, 2, 1)
	defer srv.Close()
	st, err := CheckBranchStatus(context.Background(), client, "o", "r", "feature", "main")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !st.Exists || !st.HasCommits || st.TotalCommits != 2 || st.FilesChanged != 1 {
		t.Fatalf("unexpected status: %+v", st)
	}
}

func TestDeleteBranch_OK(t *testing.T) {
	srv, client := setupPostprocessServer(t, 0, 0)
	defer srv.Close()
	if err := DeleteBranch(context.Background(), client, "o", "r", "feature"); err != nil {
		t.Fatalf("DeleteBranch error: %v", err)
	}
}
