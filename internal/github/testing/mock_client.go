package testing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"regexp"
	"strings"

	gh "github.com/google/go-github/v66/github"
)

// NewMockGitHubClient returns a go-github client backed by a local httptest server
// that responds to the minimal set of endpoints used by tests:
// - POST /repos/{owner}/{repo}/issues/{number}/comments -> {"id": 123456}
// - GET  /repos/{owner}/{repo}/git/refs/heads/{branch}
//   - returns 200 for base branch "main"
//   - returns 404 for other branches (used for existence check)
//
// - POST /repos/{owner}/{repo}/git/refs -> returns 201
// - PATCH /repos/{owner}/{repo}/issues/comments/{id} -> 200
// The returned cleanup function must be called to close the server.
func NewMockGitHubClient() (*gh.Client, func()) {
	mux := http.NewServeMux()

	// Create comment
	mux.HandleFunc("/repos/owner/repo/issues/", func(w http.ResponseWriter, r *http.Request) {
		// Match .../comments
		if r.Method == http.MethodPost && regexp.MustCompile(`/issues/\d+/comments$`).MatchString(r.URL.Path) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]int{"id": 123456})
			return
		}
		http.NotFound(w, r)
	})

	// Edit comment
	mux.HandleFunc("/repos/owner/repo/issues/comments/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	})

	// Git refs (note: go-github uses singular "ref" for get and plural "refs" for create)
	mux.HandleFunc("/repos/owner/repo/git/ref/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Expect /git/ref/heads/{branch}
			if m := regexp.MustCompile(`/git/ref/heads/(.+)$`).FindStringSubmatch(r.URL.Path); len(m) == 2 {
				branch := m[1]
				// Treat non-swe branches as existing; test-created branches start with "swe/"
				if branch != "" && !strings.HasPrefix(branch, "swe/") {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{
						"ref": "refs/heads/main",
						"object": map[string]string{
							"sha":  "abc123",
							"type": "commit",
							"url":  "https://example.com/commit/abc123",
						},
					})
					return
				}
				// Non-existing branches -> 404
				http.Error(w, "Not Found", http.StatusNotFound)
				return
			}
			http.NotFound(w, r)
			return
		}
		http.NotFound(w, r)
	})

	// Create ref
	mux.HandleFunc("/repos/owner/repo/git/refs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && path.Clean(r.URL.Path) == "/repos/owner/repo/git/refs" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ref": "refs/heads/new-branch",
				"object": map[string]string{
					"sha":  "abc123",
					"type": "commit",
					"url":  "https://example.com/commit/abc123",
				},
			})
			return
		}
		http.NotFound(w, r)
	})

	srv := httptest.NewServer(mux)

	client := gh.NewClient(srv.Client())
	base, _ := url.Parse(srv.URL + "/")
	client.BaseURL = base
	client.UploadURL = base

	cleanup := func() { srv.Close() }
	return client, cleanup
}
