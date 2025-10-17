package comment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	gh "github.com/google/go-github/v66/github"
)

func setupIssueCommentsServer(t *testing.T) (*httptest.Server, *gh.Client) {
	t.Helper()
	mux := http.NewServeMux()

	// Create comment
	mux.HandleFunc("/repos/o/r/issues/99/comments", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1001, "body": "init"})
	})

	// Edit comment
	mux.HandleFunc("/repos/o/r/issues/comments/1001", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1001, "body": "updated"})
	})

	srv := httptest.NewServer(mux)
	client := gh.NewClient(srv.Client())
	base, _ := url.Parse(srv.URL + "/")
	client.BaseURL = base
	return srv, client
}

func TestTracker_CreateInitialAndUpdate(t *testing.T) {
	srv, client := setupIssueCommentsServer(t)
	defer srv.Close()

	tr := NewTracker(client, "o", "r", 99)
	id, err := tr.CreateInitial(context.Background())
	if err != nil {
		t.Fatalf("CreateInitial error: %v", err)
	}
	if id == 0 || tr.GetCommentID() != id {
		t.Fatalf("invalid comment id: %d", id)
	}

	if err := tr.Update(context.Background(), "new body"); err != nil {
		t.Fatalf("Update error: %v", err)
	}
}
