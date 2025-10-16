package branch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	gh "github.com/google/go-github/v66/github"
)

// mockGitHubServer provides minimal endpoints used by Manager and CleanupOldBranches.
func mockGitHubServer(t *testing.T) (*httptest.Server, *gh.Client) {
	t.Helper()
	mux := http.NewServeMux()

	// Storage
	var baseSHA = "base-sha-123"
	createdRefs := map[string]bool{}

	// GET /repos/o/r/git/ref/heads/{branch}
	mux.HandleFunc("/repos/o/r/git/ref/heads/", func(w http.ResponseWriter, r *http.Request) {
		branch := strings.TrimPrefix(r.URL.Path, "/repos/o/r/git/ref/heads/")
		if branch == "main" || createdRefs[branch] {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ref": "refs/heads/" + branch,
				"object": map[string]any{"sha": func() string {
					if branch == "main" {
						return baseSHA
					}
					return "sha-" + branch
				}()},
			})
			return
		}
		http.NotFound(w, r)
	})

	// POST /repos/o/r/git/refs
	mux.HandleFunc("/repos/o/r/git/refs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Ref    string
			Object struct{ SHA string }
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		ref := strings.TrimPrefix(body.Ref, "refs/heads/")
		createdRefs[ref] = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ref": body.Ref, "object": map[string]any{"sha": body.Object.SHA}})
	})

	// DELETE /repos/o/r/git/refs/heads/{branch}
	mux.HandleFunc("/repos/o/r/git/refs/heads/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		branch := strings.TrimPrefix(r.URL.Path, "/repos/o/r/git/refs/heads/")
		delete(createdRefs, branch)
		w.WriteHeader(http.StatusNoContent)
	})

	// GET /repos/o/r/git/matching-refs/heads/{prefix}
	mux.HandleFunc("/repos/o/r/git/matching-refs/heads/", func(w http.ResponseWriter, r *http.Request) {
		prefix := strings.TrimPrefix(r.URL.Path, "/repos/o/r/git/matching-refs/heads/")
		// Return two refs under the prefix
		refs := []map[string]any{
			{"ref": "refs/heads/" + prefix + "old-1", "object": map[string]any{"sha": "sha-old-1"}},
			{"ref": "refs/heads/" + prefix + "young-2", "object": map[string]any{"sha": "sha-young-2"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(refs)
	})

	// GET /repos/o/r/commits/{sha}
	mux.HandleFunc("/repos/o/r/commits/", func(w http.ResponseWriter, r *http.Request) {
		sha := strings.TrimPrefix(r.URL.Path, "/repos/o/r/commits/")
		date := time.Now()
		if strings.Contains(sha, "old-1") {
			date = time.Now().Add(-40 * 24 * time.Hour) // 40 days ago
		}
		payload := map[string]any{
			"commit": map[string]any{
				"author": map[string]any{
					"date": date.Format(time.RFC3339),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	})

	srv := httptest.NewServer(mux)
	client := gh.NewClient(srv.Client())
	base, _ := url.Parse(srv.URL + "/")
	client.BaseURL = base
	return srv, client
}

func TestManager_CreateBranchAndExistsDelete(t *testing.T) {
	srv, client := mockGitHubServer(t)
	defer srv.Close()

	m := NewManager(client, "o", "r")

	// Create branch when not exists
	name, err := m.CreateBranch(context.Background(), "main", 12, "Fix bug")
	if err != nil {
		t.Fatalf("CreateBranch error: %v", err)
	}
	if !ValidateBranchName(name) {
		t.Fatalf("generated invalid branch name: %s", name)
	}

	// Exists should be true now
	exists, err := m.BranchExists(context.Background(), name)
	if err != nil || !exists {
		t.Fatalf("BranchExists = %v, err=%v", exists, err)
	}

	// Delete and ensure non-existence returns false without error
	if err := m.DeleteBranch(context.Background(), name); err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}
	exists, err = m.BranchExists(context.Background(), name)
	if err != nil {
		t.Fatalf("BranchExists after delete err: %v", err)
	}
	if exists {
		t.Fatalf("expected branch to not exist after delete")
	}
}

func TestManager_CleanupOldBranches(t *testing.T) {
	srv, client := mockGitHubServer(t)
	defer srv.Close()
	m := NewManager(client, "o", "r")

	deleted, err := m.CleanupOldBranches(context.Background(), CleanupOptions{MaxAge: 30 * 24 * time.Hour, Prefix: "swe/"})
	if err != nil {
		t.Fatalf("CleanupOldBranches error: %v", err)
	}
	// Expect the "old-1" branch to be deleted, but not the young one
	if len(deleted) != 1 || !strings.Contains(deleted[0], "old-1") {
		t.Fatalf("unexpected deleted list: %v", deleted)
	}
}
