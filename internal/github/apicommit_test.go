package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// rewriteTransport rewrites requests targeting api.github.com to a local server.
type rewriteTransport struct{ target *url.URL }

func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Only rewrite GitHub API requests
	if strings.EqualFold(req.URL.Host, "api.github.com") {
		// Clone request with rewritten scheme/host while preserving path/query
		u := *req.URL
		u.Scheme = rt.target.Scheme
		u.Host = rt.target.Host
		// Build a new request to avoid mutating original
		var body io.ReadCloser
		if req.Body != nil {
			// Read entire body and recreate ReadCloser for server and for Do()
			b, _ := io.ReadAll(req.Body)
			_ = req.Body.Close()
			body = io.NopCloser(bytes.NewReader(b))
			req.Body = io.NopCloser(bytes.NewReader(b))
		}
		newReq, err := http.NewRequest(req.Method, u.String(), body)
		if err != nil {
			return nil, err
		}
		newReq.Header = req.Header.Clone()
		return http.DefaultTransport.RoundTrip(newReq)
	}
	return http.DefaultTransport.RoundTrip(req)
}

// withGitHubAPIMock sets a transport to route GitHub API calls to the provided server.
func withGitHubAPIMock(t *testing.T, srv *httptest.Server) func() {
	t.Helper()
	parsed, _ := url.Parse(srv.URL)
	old := http.DefaultClient.Transport
	http.DefaultClient.Transport = &rewriteTransport{target: parsed}
	return func() { http.DefaultClient.Transport = old }
}

// mock server state for branches, commits, blobs, etc.
type apiState struct {
	// simple in-memory refs
	refs map[string]string // key: heads/<branch> -> sha
}

func newAPIState() *apiState { return &apiState{refs: map[string]string{}} }

func (s *apiState) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := r.URL.Path

	// GET /repos/{owner}/{repo}
	if r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/") && !strings.Contains(path, "/git/") {
		// reply with default branch main by default
		_ = json.NewEncoder(w).Encode(map[string]any{"default_branch": "main"})
		return
	}

	// GET /repos/{owner}/{repo}/git/refs/heads/{branch}
	if r.Method == http.MethodGet && strings.Contains(path, "/git/refs/heads/") {
		parts := strings.Split(path, "/git/refs/heads/")
		branch := parts[len(parts)-1]
		if sha, ok := s.refs["heads/"+branch]; ok {
			_ = json.NewEncoder(w).Encode(map[string]any{"object": map[string]any{"sha": sha}})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// POST /repos/{owner}/{repo}/git/refs
	if r.Method == http.MethodPost && strings.HasSuffix(path, "/git/refs") {
		var body struct{ Ref, Sha string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.refs[strings.TrimPrefix(body.Ref, "refs/")] = body.Sha
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"ok":true}`)
		return
	}

	// GET /git/commits/{sha}
	if r.Method == http.MethodGet && strings.Contains(path, "/git/commits/") {
		_, _ = io.WriteString(w, `{"tree":{"sha":"treesha"}}`)
		return
	}

	// POST /git/blobs
	if r.Method == http.MethodPost && strings.HasSuffix(path, "/git/blobs") {
		_, _ = io.WriteString(w, `{"sha":"blobsha"}`)
		return
	}

	// POST /git/trees
	if r.Method == http.MethodPost && strings.HasSuffix(path, "/git/trees") {
		_, _ = io.WriteString(w, `{"sha":"treesha2"}`)
		return
	}

	// POST /git/commits
	if r.Method == http.MethodPost && strings.HasSuffix(path, "/git/commits") {
		_, _ = io.WriteString(w, `{"sha":"commitsha","message":"ok","author":{"name":"bot","date":"2020-01-01T00:00:00Z"}}`)
		return
	}

	// PATCH /git/refs/heads/{branch}
	if r.Method == http.MethodPatch && strings.Contains(path, "/git/refs/heads/") {
		_, _ = io.WriteString(w, `{"ok":true}`)
		return
	}

	http.Error(w, "unexpected route", http.StatusTeapot)
}

func TestCommitFilesAPI_Success_TextAndBinary(t *testing.T) {
	state := newAPIState()
	// base branch main exists
	state.refs["heads/main"] = "basesha"
	srv := httptest.NewServer(http.HandlerFunc(state.handler))
	defer srv.Close()
	restore := withGitHubAPIMock(t, srv)
	defer restore()

	files := []APIFile{
		{Path: "a.txt", Content: []byte("hello"), Mode: "", Binary: false},
		{Path: "img.png", Content: []byte{0x89, 0x50}, Mode: "100644", Binary: true},
	}

	sha, err := CommitFilesAPI("owner", "repo", "feature/x", "main", "msg", "t", files)
	if err != nil {
		t.Fatalf("CommitFilesAPI error: %v", err)
	}
	if sha != "commitsha" {
		t.Fatalf("got sha %q, want commitsha", sha)
	}
	if _, ok := state.refs["heads/feature/x"]; !ok {
		t.Fatalf("branch ref not created")
	}
}

func TestGetOrCreateBranchRef_ExistingRef(t *testing.T) {
	state := newAPIState()
	state.refs["heads/feature/x"] = "abc123"
	srv := httptest.NewServer(http.HandlerFunc(state.handler))
	defer srv.Close()
	restore := withGitHubAPIMock(t, srv)
	defer restore()

	sha, err := getOrCreateBranchRef("o", "r", "feature/x", "main", "t")
	if err != nil {
		t.Fatalf("getOrCreateBranchRef error: %v", err)
	}
	if sha != "abc123" {
		t.Fatalf("sha = %q, want abc123", sha)
	}
}

func TestGetOrCreateBranchRef_DefaultBranchFallback(t *testing.T) {
	state := newAPIState()
	// only main exists
	state.refs["heads/main"] = "ms"
	srv := httptest.NewServer(http.HandlerFunc(state.handler))
	defer srv.Close()
	restore := withGitHubAPIMock(t, srv)
	defer restore()

	sha, err := getOrCreateBranchRef("o", "r", "feat", "dev", "t")
	if err != nil {
		t.Fatalf("getOrCreateBranchRef error: %v", err)
	}
	if sha != "ms" {
		t.Fatalf("sha = %q, want ms (default branch)", sha)
	}
	if _, ok := state.refs["heads/feat"]; !ok {
		t.Fatal("expected new ref created for feat")
	}
}

func TestGetRef_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	restore := withGitHubAPIMock(t, srv)
	defer restore()

	if _, err := getRef("o", "r", "heads/x", "t"); err == nil {
		t.Fatal("getRef expected error on 500")
	}
}

func TestGetCommit_ParseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		// invalid JSON to force unmarshal error
		_, _ = io.WriteString(w, `{invalid json`)
	}))
	defer srv.Close()
	restore := withGitHubAPIMock(t, srv)
	defer restore()

	if _, err := getCommit("o", "r", "sha", "t"); err == nil {
		t.Fatal("getCommit expected parse error")
	}
}

func TestCreateTree_TextOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/git/trees") {
			_, _ = io.WriteString(w, `{"sha":"treesum"}`)
			return
		}
		http.Error(w, "unexpected", 400)
	}))
	defer srv.Close()
	restore := withGitHubAPIMock(t, srv)
	defer restore()

	sha, err := createTree("o", "r", "base", "t", []APIFile{{Path: "a.txt", Content: []byte("x")}})
	if err != nil {
		t.Fatalf("createTree error: %v", err)
	}
	if sha != "treesum" {
		t.Fatalf("sha = %q, want treesum", sha)
	}
}

func TestCreateCommit_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"sha":"newc","message":"ok","author":{"name":"bot","date":"2020-01-01T00:00:00Z"}}`)
	}))
	defer srv.Close()
	restore := withGitHubAPIMock(t, srv)
	defer restore()

	c, err := createCommit("o", "r", "m", "tree", "parent", "t")
	if err != nil {
		t.Fatalf("createCommit error: %v", err)
	}
	if c.SHA != "newc" || c.Message != "ok" {
		t.Fatalf("unexpected commit: %+v", c)
	}
}

func TestUpdateRefWithRetry_403ThenSuccess(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			http.Error(w, "forbidden", 403)
			return
		}
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()
	restore := withGitHubAPIMock(t, srv)
	defer restore()

	if err := updateRefWithRetry("o", "r", "b", "sha", "t"); err != nil {
		t.Fatalf("updateRefWithRetry unexpected error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
}

func TestUpdateRefWithRetry_Permanent403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", 403)
	}))
	defer srv.Close()
	restore := withGitHubAPIMock(t, srv)
	defer restore()

	if err := updateRefWithRetry("o", "r", "b", "sha", "t"); err == nil || !strings.Contains(strings.ToLower(err.Error()), "permission denied") {
		t.Fatalf("expected permission denied error, got: %v", err)
	}
}

func TestUpdateRefWithRetry_OtherError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", 500)
	}))
	defer srv.Close()
	restore := withGitHubAPIMock(t, srv)
	defer restore()
	if err := updateRefWithRetry("o", "r", "b", "sha", "t"); err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestCollectChangedFilesForAPICommit(t *testing.T) {
	// Initialize a real git repo for status output
	dir := t.TempDir()
	must := func(cmd *exec.Cmd) {
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cmd failed: %v\n%s", err, out)
		}
	}
	must(exec.Command("git", "init"))

	// Create files
	write := func(path string, data []byte, mode os.FileMode) {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, data, mode); err != nil {
			t.Fatal(err)
		}
	}

	write("script.sh", []byte("echo hi\n"), 0o755)
	write("doc.txt", []byte("hello"), 0o644)
	write("asset/image.png", []byte{0x89, 0x50}, 0o644)
	write("font.woff2", []byte("xx"), 0o644)

	files, err := CollectChangedFilesForAPICommit(dir)
	if err != nil {
		t.Fatalf("CollectChangedFilesForAPICommit error: %v", err)
	}

	// Expect 4 files and correct modes/binary detection
	if len(files) != 4 {
		t.Fatalf("files = %d, want 4 (%v)", len(files), files)
	}

	// Map by path
	m := map[string]APIFile{}
	for _, f := range files {
		m[f.Path] = f
	}
	if m["script.sh"].Mode != "100755" {
		t.Fatalf("script mode = %s, want 100755", m["script.sh"].Mode)
	}
	if m["doc.txt"].Mode != "100644" {
		t.Fatalf("doc mode = %s, want 100644", m["doc.txt"].Mode)
	}
	if !m["asset/image.png"].Binary {
		t.Fatalf("image.png should be binary")
	}
	if !m["font.woff2"].Binary {
		t.Fatalf("woff2 should be binary")
	}
}

func TestIsBinaryFile_Table(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"a.png", true}, {"b.jpg", true}, {"c.gif", true}, {"d.webp", true},
		{"e.ico", true}, {"f.pdf", true}, {"g.zip", true}, {"h.tar", true},
		{"i.gz", true}, {"j.exe", true}, {"k.bin", true}, {"l.woff", true},
		{"m.woff2", true}, {"n.ttf", true}, {"o.eot", true}, {"p.txt", false},
		{"q.go", false}, {"r.md", false}, {"s.PNG", true}, {"t.WOFF2", true},
	}
	for _, tc := range cases {
		if got := isBinaryFile(tc.path); got != tc.want {
			t.Fatalf("isBinaryFile(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestAPIDo_ErrorPaths(t *testing.T) {
	// failed to marshal body
	if _, err := apiDo("POST", "http://example.com", "t", map[string]any{"fn": func() {}}); err == nil {
		t.Fatal("expected marshal error")
	}

	// failed to create request
	if _, err := apiDo("GET", "http://[::1", "t", nil); err == nil {
		t.Fatal("expected invalid URL error")
	}

	// request failed (DNS)
	if _, err := apiDo("GET", "http://nonexistent.invalid", "t", nil); err == nil {
		t.Fatal("expected request error")
	}

	// non-2xx status
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", 500) }))
	defer srv.Close()
	restore := withGitHubAPIMock(t, srv)
	defer restore()
	if _, err := apiDo("GET", fmt.Sprintf("%s/repos/o/r", githubAPIBase), "t", nil); err == nil {
		t.Fatal("expected status error")
	}
}
