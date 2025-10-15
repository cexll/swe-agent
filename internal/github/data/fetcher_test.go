package data

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	gh "github.com/cexll/swe/internal/github"
)

func TestSplitRepo_Valid(t *testing.T) {
	owner, repo, err := splitRepo("owner/repo")
	if err != nil {
		t.Fatalf("splitRepo() returned unexpected error: %v", err)
	}
	if owner != "owner" {
		t.Errorf("owner = %q, want 'owner'", owner)
	}
	if repo != "repo" {
		t.Errorf("repo = %q, want 'repo'", repo)
	}
}

func TestSplitRepo_InvalidFormat(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"no slash", "invalid"},
		{"too many parts", "owner/repo/extra"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := splitRepo(tt.input)
			if err == nil {
				t.Errorf("splitRepo(%q) should return error", tt.input)
				return
			}
			if !strings.Contains(err.Error(), "invalid repository format") {
				t.Errorf("error should mention invalid format, got: %v", err)
			}
		})
	}
}

func TestFilterComments_NoTriggerTime(t *testing.T) {
	comments := []Comment{
		{Body: "first", CreatedAt: "2024-01-01T10:00:00Z"},
		{Body: "second", CreatedAt: "2024-01-02T10:00:00Z"},
	}

	result := FilterComments(comments, "")
	if len(result) != 2 {
		t.Errorf("FilterComments with empty trigger should return all comments, got %d", len(result))
	}
}

func TestFilterComments_FiltersByTriggerTime(t *testing.T) {
	comments := []Comment{
		{
			Body:      "before trigger",
			CreatedAt: "2024-01-01T10:00:00Z",
			UpdatedAt: "2024-01-01T11:00:00Z",
		},
		{
			Body:      "at trigger time",
			CreatedAt: "2024-01-02T10:00:00Z",
		},
		{
			Body:      "after trigger",
			CreatedAt: "2024-01-03T10:00:00Z",
		},
	}

	triggerTime := "2024-01-02T10:00:00Z"
	result := FilterComments(comments, triggerTime)

	if len(result) != 1 {
		t.Errorf("FilterComments should return 1 comment before trigger, got %d", len(result))
	}
	if len(result) > 0 && result[0].Body != "before trigger" {
		t.Errorf("Remaining comment should be 'before trigger', got: %q", result[0].Body)
	}
}

func TestFilterComments_ExcludesEditedAfterTrigger(t *testing.T) {
	now := time.Now()
	before := now.Add(-2 * time.Hour).Format(time.RFC3339)
	trigger := now.Add(-1 * time.Hour).Format(time.RFC3339)
	after := now.Format(time.RFC3339)

	comments := []Comment{
		{
			Body:         "created before, edited after trigger",
			CreatedAt:    before,
			LastEditedAt: after,
		},
		{
			Body:      "created and edited before trigger",
			CreatedAt: before,
			UpdatedAt: before,
		},
	}

	result := FilterComments(comments, trigger)

	if len(result) != 1 {
		t.Errorf("Should return 1 comment, got %d", len(result))
	}
	if len(result) > 0 && !strings.Contains(result[0].Body, "created and edited before") {
		t.Errorf("Wrong comment filtered, got: %q", result[0].Body)
	}
}

func TestFilterComments_InvalidTriggerTime(t *testing.T) {
	comments := []Comment{
		{Body: "test", CreatedAt: "2024-01-01T10:00:00Z"},
	}

	result := FilterComments(comments, "invalid-time-format")

	if len(result) != 1 {
		t.Error("FilterComments with invalid trigger time should return all comments")
	}
}

func TestFilterReviews_NoTriggerTime(t *testing.T) {
	reviews := []Review{
		{Body: "first review", SubmittedAt: "2024-01-01T10:00:00Z"},
		{Body: "second review", SubmittedAt: "2024-01-02T10:00:00Z"},
	}

	result := FilterReviews(reviews, "")
	if len(result) != 2 {
		t.Errorf("FilterReviews with empty trigger should return all reviews, got %d", len(result))
	}
}

func TestFilterReviews_FiltersByTriggerTime(t *testing.T) {
	reviews := []Review{
		{
			Body:        "before trigger",
			SubmittedAt: "2024-01-01T10:00:00Z",
		},
		{
			Body:        "at trigger time",
			SubmittedAt: "2024-01-02T10:00:00Z",
		},
		{
			Body:        "after trigger",
			SubmittedAt: "2024-01-03T10:00:00Z",
		},
	}

	triggerTime := "2024-01-02T10:00:00Z"
	result := FilterReviews(reviews, triggerTime)

	if len(result) != 1 {
		t.Errorf("FilterReviews should return 1 review before trigger, got %d", len(result))
	}
	if len(result) > 0 && result[0].Body != "before trigger" {
		t.Errorf("Remaining review should be 'before trigger', got: %q", result[0].Body)
	}
}

func TestFilterReviews_ExcludesEditedAfterTrigger(t *testing.T) {
	now := time.Now()
	before := now.Add(-2 * time.Hour).Format(time.RFC3339)
	trigger := now.Add(-1 * time.Hour).Format(time.RFC3339)
	after := now.Format(time.RFC3339)

	reviews := []Review{
		{
			Body:         "submitted before, edited after",
			SubmittedAt:  before,
			LastEditedAt: after,
		},
		{
			Body:        "submitted and updated before",
			SubmittedAt: before,
			UpdatedAt:   before,
		},
	}

	result := FilterReviews(reviews, trigger)

	if len(result) != 1 {
		t.Errorf("Should return 1 review, got %d", len(result))
	}
	if len(result) > 0 && !strings.Contains(result[0].Body, "submitted and updated before") {
		t.Errorf("Wrong review filtered, got: %q", result[0].Body)
	}
}

func TestFormatComments_Empty(t *testing.T) {
	result := FormatComments([]Comment{})
	if result != "" {
		t.Errorf("FormatComments([]) should return empty string, got: %q", result)
	}
}

func TestFormatComments_SingleComment(t *testing.T) {
	comments := []Comment{
		{
			Body:      "test comment body",
			Author:    Author{Login: "testuser"},
			CreatedAt: "2024-01-01T10:00:00Z",
		},
	}

	result := FormatComments(comments)

	if !strings.Contains(result, "testuser") {
		t.Error("Formatted comments should contain author login")
	}
	if !strings.Contains(result, "2024-01-01T10:00:00Z") {
		t.Error("Formatted comments should contain timestamp")
	}
	if !strings.Contains(result, "test comment body") {
		t.Error("Formatted comments should contain body text")
	}
	if !strings.Contains(result, "[testuser at 2024-01-01T10:00:00Z]:") {
		t.Error("Formatted comments should have correct format")
	}
}

func TestFormatComments_MultipleComments(t *testing.T) {
	comments := []Comment{
		{Body: "first", Author: Author{Login: "user1"}, CreatedAt: "2024-01-01T10:00:00Z"},
		{Body: "second", Author: Author{Login: "user2"}, CreatedAt: "2024-01-02T10:00:00Z"},
	}

	result := FormatComments(comments)

	if !strings.Contains(result, "first") {
		t.Error("Should contain first comment")
	}
	if !strings.Contains(result, "second") {
		t.Error("Should contain second comment")
	}
	if !strings.Contains(result, "user1") {
		t.Error("Should contain first user")
	}
	if !strings.Contains(result, "user2") {
		t.Error("Should contain second user")
	}
}

func TestFormatComments_SkipsMinimizedComments(t *testing.T) {
	comments := []Comment{
		{Body: "visible", Author: Author{Login: "user1"}, CreatedAt: "2024-01-01T10:00:00Z", IsMinimized: false},
		{Body: "hidden", Author: Author{Login: "user2"}, CreatedAt: "2024-01-02T10:00:00Z", IsMinimized: true},
	}

	result := FormatComments(comments)

	if !strings.Contains(result, "visible") {
		t.Error("Should contain visible comment")
	}
	if strings.Contains(result, "hidden") {
		t.Error("Should not contain minimized comment")
	}
}

func TestFormatChangedFilesWithSHA_Empty(t *testing.T) {
	result := FormatChangedFilesWithSHA([]GitHubFileWithSHA{})
	if result != "" {
		t.Errorf("FormatChangedFilesWithSHA([]) should return empty string, got: %q", result)
	}
}

func TestFormatChangedFilesWithSHA_SingleFile(t *testing.T) {
	files := []GitHubFileWithSHA{
		{
			File: File{
				Path:       "main.go",
				ChangeType: "MODIFIED",
				Additions:  10,
				Deletions:  5,
			},
			SHA: "abc123",
		},
	}

	result := FormatChangedFilesWithSHA(files)

	if !strings.Contains(result, "main.go") {
		t.Error("Should contain file path")
	}
	if !strings.Contains(result, "MODIFIED") {
		t.Error("Should contain change type")
	}
	if !strings.Contains(result, "+10") {
		t.Error("Should contain additions")
	}
	if !strings.Contains(result, "-5") {
		t.Error("Should contain deletions")
	}
	if !strings.Contains(result, "SHA: abc123") {
		t.Error("Should contain SHA")
	}
}

func TestFormatChangedFilesWithSHA_MultipleFiles(t *testing.T) {
	files := []GitHubFileWithSHA{
		{
			File: File{Path: "file1.go", ChangeType: "ADDED", Additions: 100, Deletions: 0},
			SHA:  "sha1",
		},
		{
			File: File{Path: "file2.go", ChangeType: "DELETED", Additions: 0, Deletions: 50},
			SHA:  "deleted",
		},
	}

	result := FormatChangedFilesWithSHA(files)

	if !strings.Contains(result, "file1.go") {
		t.Error("Should contain first file")
	}
	if !strings.Contains(result, "file2.go") {
		t.Error("Should contain second file")
	}
	if !strings.Contains(result, "ADDED") {
		t.Error("Should contain ADDED change type")
	}
	if !strings.Contains(result, "DELETED") {
		t.Error("Should contain DELETED change type")
	}
	if !strings.Contains(result, "sha1") {
		t.Error("Should contain first SHA")
	}
	if !strings.Contains(result, "deleted") {
		t.Error("Should contain deleted marker")
	}
}

func TestFormatChangedFilesWithSHA_LineFormat(t *testing.T) {
	files := []GitHubFileWithSHA{
		{
			File: File{Path: "test.go", ChangeType: "MODIFIED", Additions: 5, Deletions: 3},
			SHA:  "abc123",
		},
	}

	result := FormatChangedFilesWithSHA(files)

	expected := "- test.go (MODIFIED) +5/-3 SHA: abc123"
	if result != expected {
		t.Errorf("FormatChangedFilesWithSHA() = %q, want %q", result, expected)
	}
}

// -------------------- New tests for fetcher.go behaviors --------------------

type fakeAuth2 struct{}

func (f fakeAuth2) GetInstallationToken(repo string) (*gh.InstallationToken, error) {
	return &gh.InstallationToken{Token: "t", ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (f fakeAuth2) GetInstallationOwner(repo string) (string, error) { return "o", nil }

// helper to create graphql test server with programmable responses
func newGraphQLServer(t *testing.T, handler func(query string, vars map[string]any) (status int, body any)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errors":[{"message":"bad req"}]}`))
			return
		}
		status, body := handler(req.Query, req.Variables)
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
}

func TestFetchGitHubData_Issue(t *testing.T) {
	ts := newGraphQLServer(t, func(query string, vars map[string]any) (int, any) {
		if strings.Contains(query, "issue(") {
			return 200, map[string]any{"data": map[string]any{"repository": map[string]any{"issue": map[string]any{
				"title":     "Bug",
				"body":      "Body",
				"author":    map[string]any{"login": "bob"},
				"createdAt": "t",
				"state":     "OPEN",
				"comments": map[string]any{"nodes": []any{
					map[string]any{"id": "1", "databaseId": 1, "body": "c1", "author": map[string]any{"login": "u1"}, "createdAt": "t1", "isMinimized": false},
				}},
			}}}}
		}
		if strings.Contains(query, "User(") {
			login, _ := vars["login"].(string)
			name := ""
			if login == "trig" {
				name = "Trigger User"
			}
			return 200, map[string]any{"data": map[string]any{"user": map[string]any{"name": name}}}
		}
		t.Fatalf("unexpected query: %s", query)
		return 200, nil
	})
	defer ts.Close()

	c := NewClient(fakeAuth2{})
	c.endpoint = ts.URL

	res, err := FetchGitHubData(context.Background(), FetchParams{Client: c, Repository: "o/r", Number: 1, IsPR: false, TriggerUsername: "trig"})
	if err != nil {
		t.Fatalf("FetchGitHubData issue: %v", err)
	}
	if _, ok := res.ContextData.(Issue); !ok {
		t.Fatalf("ContextData should be Issue")
	}
	if len(res.Comments) != 1 {
		t.Fatalf("want 1 comment, got %d", len(res.Comments))
	}
	if res.TriggerName == nil || *res.TriggerName != "Trigger User" {
		t.Fatalf("unexpected trigger name: %+v", res.TriggerName)
	}
}

func TestFetchGitHubData_PR(t *testing.T) {
	tmp := t.TempDir()
	fpath := filepath.Join(tmp, "file.go")
	if err := os.WriteFile(fpath, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ts := newGraphQLServer(t, func(query string, vars map[string]any) (int, any) {
		if strings.Contains(query, "pullRequest(") {
			return 200, map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
				"title": "P", "body": "B", "author": map[string]any{"login": "alice"},
				"baseRefName": "main", "headRefName": "f", "headRefOid": "deadbeef", "createdAt": "t", "additions": 1, "deletions": 0, "state": "OPEN",
				"commits": map[string]any{"totalCount": 1, "nodes": []any{map[string]any{"commit": map[string]any{"oid": "c", "message": "m", "author": map[string]any{"name": "n", "email": "e"}}}}},
				"files": map[string]any{"nodes": []any{
					map[string]any{"path": fpath, "additions": 1, "deletions": 0, "changeType": "MODIFIED"},
					map[string]any{"path": "deleted.txt", "additions": 0, "deletions": 0, "changeType": "DELETED"},
				}},
				"comments": map[string]any{"nodes": []any{}},
				"reviews": map[string]any{"nodes": []any{map[string]any{"id": "r1", "databaseId": 1, "author": map[string]any{"login": "rev"}, "body": "ok", "state": "COMMENTED", "submittedAt": "t",
					"comments": map[string]any{"nodes": []any{map[string]any{"id": "c1", "databaseId": 2, "body": "inl", "author": map[string]any{"login": "u"}, "createdAt": "t", "isMinimized": false, "path": "p.go", "line": 10}}},
				}}},
			}}}}
		}
		if strings.Contains(query, "User(") {
			return 200, map[string]any{"data": map[string]any{"user": map[string]any{"name": "Alice"}}}
		}
		t.Fatalf("unexpected query: %s", query)
		return 200, nil
	})
	defer ts.Close()

	c := NewClient(fakeAuth2{})
	c.endpoint = ts.URL
	res, err := FetchGitHubData(context.Background(), FetchParams{Client: c, Repository: "o/r", Number: 2, IsPR: true, TriggerUsername: "alice"})
	if err != nil {
		t.Fatalf("FetchGitHubData pr: %v", err)
	}
	if _, ok := res.ContextData.(PullRequest); !ok {
		t.Fatalf("ContextData should be PR")
	}
	if len(res.Changed) != 2 {
		t.Fatalf("want 2 files, got %d", len(res.Changed))
	}
	if len(res.ChangedSHA) != 2 {
		t.Fatalf("want 2 with sha, got %d", len(res.ChangedSHA))
	}
	// one deleted marker present
	foundDeleted := false
	for _, f := range res.ChangedSHA {
		if f.SHA == "deleted" {
			foundDeleted = true
		}
	}
	if !foundDeleted {
		t.Fatalf("missing deleted marker in ChangedSHA: %+v", res.ChangedSHA)
	}
	// non-deleted has non-empty and not unknown
	reHex := regexp.MustCompile(`^[a-f0-9]{7,64}$`)
	oksha := false
	for _, f := range res.ChangedSHA {
		if f.Path == fpath {
			oksha = f.SHA != "" && f.SHA != "unknown" && reHex.MatchString(f.SHA)
		}
	}
	if !oksha {
		t.Fatalf("unexpected blob sha: %+v", res.ChangedSHA)
	}
	if res.TriggerName == nil || *res.TriggerName != "Alice" {
		t.Fatalf("bad trigger name: %+v", res.TriggerName)
	}
}

func TestFetchGitHubData_GraphQLError(t *testing.T) {
	ts := newGraphQLServer(t, func(query string, vars map[string]any) (int, any) {
		if strings.Contains(query, "pullRequest(") {
			return 200, map[string]any{"errors": []map[string]string{{"message": "nope"}}}
		}
		if strings.Contains(query, "issue(") {
			return 200, map[string]any{"errors": []map[string]string{{"message": "nope"}}}
		}
		return 200, map[string]any{"data": map[string]any{}}
	})
	defer ts.Close()
	c := NewClient(fakeAuth2{})
	c.endpoint = ts.URL
	if _, err := FetchGitHubData(context.Background(), FetchParams{Client: c, Repository: "o/r", Number: 1, IsPR: true}); err == nil {
		t.Fatalf("expected error for PR fetch")
	}
	if _, err := FetchGitHubData(context.Background(), FetchParams{Client: c, Repository: "o/r", Number: 1, IsPR: false}); err == nil {
		t.Fatalf("expected error for Issue fetch")
	}
}

func TestFetchUserDisplayName(t *testing.T) {
	ts := newGraphQLServer(t, func(query string, vars map[string]any) (int, any) {
		login, _ := vars["login"].(string)
		if login == "has" {
			return 200, map[string]any{"data": map[string]any{"user": map[string]any{"name": "Has Name"}}}
		}
		// null name
		return 200, map[string]any{"data": map[string]any{"user": map[string]any{"name": nil}}}
	})
	defer ts.Close()
	c := NewClient(fakeAuth2{})
	c.endpoint = ts.URL
	name, err := FetchUserDisplayName(context.Background(), c, "o/r", "has")
	if err != nil || name == nil || *name != "Has Name" {
		t.Fatalf("unexpected: %v %v", err, name)
	}
	name, err = FetchUserDisplayName(context.Background(), c, "o/r", "none")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != nil {
		t.Fatalf("expected nil name when server returns null")
	}
}

func TestGitHashObject(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "x.txt")
	if err := os.WriteFile(p, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sha, err := gitHashObject(p)
	if err != nil || sha == "" {
		t.Fatalf("unexpected: %v %q", err, sha)
	}
	if _, err = gitHashObject(filepath.Join(tmp, "nope.txt")); err == nil {
		t.Fatalf("expected error for missing file")
	}
}
