package executor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cexll/swe/internal/github"
	ghdata "github.com/cexll/swe/internal/github/data"
	"github.com/cexll/swe/internal/provider"
)

// mockProvider is a mock implementation of provider.Provider
type mockProvider struct {
	generateFunc func(ctx context.Context, req *provider.CodeRequest) (*provider.CodeResponse, error)
	name         string
}

func (m *mockProvider) GenerateCode(ctx context.Context, req *provider.CodeRequest) (*provider.CodeResponse, error) {
	if m.generateFunc != nil {
		return m.generateFunc(ctx, req)
	}
	return &provider.CodeResponse{
		Summary: "Test changes",
	}, nil
}

func (m *mockProvider) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock"
}

// mockAuthProvider is a mock for github.AuthProvider
type mockAuthProvider struct {
	tokenFunc func(repo string) (*github.InstallationToken, error)
	ownerFunc func(repo string) (string, error)
	lastRepo  string
}

func (m *mockAuthProvider) GetInstallationToken(repo string) (*github.InstallationToken, error) {
	m.lastRepo = repo
	if m.tokenFunc != nil {
		return m.tokenFunc(repo)
	}
	return &github.InstallationToken{Token: "test-token"}, nil
}

func (m *mockAuthProvider) GetInstallationOwner(repo string) (string, error) {
	if m.ownerFunc != nil {
		return m.ownerFunc(repo)
	}
	return "test-owner", nil
}

// mockFetcher implements fetcherIface for testing.
type mockFetcher struct {
	fetchFunc func(ctx context.Context, gctx *github.Context) (*ghdata.FetchResult, error)
}

func (m *mockFetcher) Fetch(ctx context.Context, gctx *github.Context) (*ghdata.FetchResult, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(ctx, gctx)
	}
	return &ghdata.FetchResult{}, nil
}

// helper to construct a minimal github.Context for tests
func buildTestCtx(isPR bool) *github.Context {
	ctx := &github.Context{
		EventName:   github.EventIssueComment,
		EventAction: github.ActionCreated,
		Repository: github.Repository{
			Owner:    "owner",
			Name:     "repo",
			FullName: "owner/repo",
		},
		Actor:       "tester",
		IsPR:        isPR,
		IssueNumber: 1,
		PRNumber:    0,
		BaseBranch:  "main",
		HeadBranch:  "feature",
		TriggerUser: "tester",
		TriggerComment: &github.Comment{
			ID:   1,
			Body: "/code do it",
			User: "tester",
		},
	}
	if isPR {
		ctx.PRNumber = 2
		ctx.IssueNumber = 2
	}
	return ctx
}

func runCtxMapTest(t *testing.T, ghCtx *github.Context, assert func(map[string]string)) {
	t.Helper()

	origClone := cloneRepo
	origRun := runCmd
	t.Cleanup(func() {
		cloneRepo = origClone
		runCmd = origRun
	})

	tmpDir := t.TempDir()
	cloneRepo = func(repo, branch, token string) (string, func(), error) {
		return tmpDir, func() {}, nil
	}
	runCmd = func(name string, args ...string) error { return nil }

	mp := &mockProvider{generateFunc: func(ctx context.Context, req *provider.CodeRequest) (*provider.CodeResponse, error) {
		if req.Context == nil {
			t.Fatal("provider request missing context map")
		}
		assert(req.Context)
		return &provider.CodeResponse{Summary: "ok"}, nil
	}}
	ma := &mockAuthProvider{}
	ex := New(mp, ma)
	ex.fetcher = &mockFetcher{fetchFunc: func(ctx context.Context, gctx *github.Context) (*ghdata.FetchResult, error) {
		return &ghdata.FetchResult{}, nil
	}}

	if ghCtx.PreparedPrompt == "" {
		ghCtx.PreparedPrompt = "stub prompt"
	}

	if err := ex.Execute(context.Background(), ghCtx); err != nil {
		t.Fatalf("Execute() err = %v", err)
	}
}

func expectField(t *testing.T, m map[string]string, key, want string) {
	t.Helper()
	got, ok := m[key]
	if !ok {
		t.Fatalf("context missing key %q", key)
	}
	if got != want {
		t.Fatalf("context[%q] = %q, want %q", key, got, want)
	}
}

func expectNoField(t *testing.T, m map[string]string, key string) {
	t.Helper()
	if _, ok := m[key]; ok {
		t.Fatalf("context should not contain key %q", key)
	}
}

func expectBaseFields(t *testing.T, ghCtx *github.Context, m map[string]string) {
	t.Helper()

	expectField(t, m, "github_token", "test-token")

	repo := ghCtx.GetRepositoryFullName()
	if repo == "" {
		repo = fmt.Sprintf("%s/%s", ghCtx.GetRepositoryOwner(), ghCtx.GetRepositoryName())
	}
	expectField(t, m, "repository", repo)

	base := ghCtx.PreparedBaseBranch
	if base == "" {
		base = ghCtx.GetBaseBranch()
		if base == "" {
			base = "main"
		}
	}
	expectField(t, m, "base_branch", base)
	expectField(t, m, "head_branch", ghCtx.GetHeadBranch())
}

func TestNew(t *testing.T) {
	provider := &mockProvider{}
	auth := &mockAuthProvider{}
	executor := New(provider, auth)

	if executor == nil {
		t.Fatal("New() returned nil")
	}
	if executor.provider == nil {
		t.Error("executor.provider is nil")
	}
	if executor.auth == nil {
		t.Error("executor.auth is nil")
	}
}

func TestFeatureBranchName(t *testing.T) {
	tests := []struct {
		name        string
		issueNumber int
		prNumber    int
		isPR        bool
		wantPrefix  string
	}{
		{
			name:        "issue 42",
			issueNumber: 42,
			isPR:        false,
			wantPrefix:  "swe-agent/42-",
		},
		{
			name:       "PR 123",
			prNumber:   123,
			isPR:       true,
			wantPrefix: "swe-agent/123-",
		},
		{
			name:        "zero issue",
			issueNumber: 0,
			isPR:        false,
			wantPrefix:  "swe-agent/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &github.Context{
				IsPR:        tt.isPR,
				IssueNumber: tt.issueNumber,
				PRNumber:    tt.prNumber,
			}

			before := time.Now().Unix()
			branch := featureBranchName(ctx)
			after := time.Now().Unix()

			if len(branch) < len(tt.wantPrefix) {
				t.Fatalf("branch %q is shorter than expected prefix %q", branch, tt.wantPrefix)
			}

			if !strings.HasPrefix(branch, "swe-agent/") {
				t.Fatalf("branch = %q, want swe-agent/ prefix", branch)
			}
			if branch[:len(tt.wantPrefix)] != tt.wantPrefix {
				t.Errorf("branch = %q, want prefix %q", branch, tt.wantPrefix)
			}

			if len(branch) <= len(tt.wantPrefix) {
				t.Error("branch should have timestamp suffix")
			}

			// Basic sanity: timestamp should be between before/after
			if after < before {
				t.Fatalf("time moved backwards: before=%d after=%d", before, after)
			}
		})
	}
}

// --- Execute() tests ---

func TestExecute_CtxMapWithCommentID(t *testing.T) {
	ctx := buildTestCtx(true)
	ctx.PreparedCommentID = 42

	runCtxMapTest(t, ctx, func(m map[string]string) {
		expectBaseFields(t, ctx, m)
		expectField(t, m, "comment_id", "42")
		expectField(t, m, "repo_owner", ctx.GetRepositoryOwner())
		expectField(t, m, "repo_name", ctx.GetRepositoryName())
		expectField(t, m, "event_name", ctx.GetEventName())
	})
}

func TestExecute_CtxMapWithoutCommentID(t *testing.T) {
	ctx := buildTestCtx(false)
	ctx.PreparedCommentID = 0

	runCtxMapTest(t, ctx, func(m map[string]string) {
		expectBaseFields(t, ctx, m)
		expectNoField(t, m, "comment_id")
		expectNoField(t, m, "repo_owner")
		expectNoField(t, m, "repo_name")
		expectNoField(t, m, "event_name")
	})
}

func TestExecute_CtxMapPRContext(t *testing.T) {
	ctx := buildTestCtx(true)
	ctx.PRNumber = 88

	runCtxMapTest(t, ctx, func(m map[string]string) {
		expectBaseFields(t, ctx, m)
		expectField(t, m, "pr_number", "88")
		expectNoField(t, m, "issue_number")
	})
}

func TestExecute_CtxMapIssueContext(t *testing.T) {
	ctx := buildTestCtx(false)
	ctx.IssueNumber = 17

	runCtxMapTest(t, ctx, func(m map[string]string) {
		expectBaseFields(t, ctx, m)
		expectField(t, m, "issue_number", "17")
		expectNoField(t, m, "pr_number")
	})
}

func TestExecute_CtxMapAllFields(t *testing.T) {
	ctx := buildTestCtx(true)
	ctx.PreparedCommentID = 101
	ctx.PRNumber = 202

	runCtxMapTest(t, ctx, func(m map[string]string) {
		expectBaseFields(t, ctx, m)
		expectField(t, m, "comment_id", "101")
		expectField(t, m, "repo_owner", ctx.GetRepositoryOwner())
		expectField(t, m, "repo_name", ctx.GetRepositoryName())
		expectField(t, m, "event_name", ctx.GetEventName())
		expectField(t, m, "pr_number", "202")
	})
}

func TestExecute_Success(t *testing.T) {
	// Save and restore globals
	origClone := cloneRepo
	origRun := runCmd
	defer func() { cloneRepo = origClone; runCmd = origRun }()

	// Mock cloneRepo to create a temp workdir and no error
	cloneRepo = func(repo, branch, token string) (string, func(), error) {
		dir := t.TempDir()
		return dir, func() {}, nil
	}
	// Mock runCmd to succeed
	runCmd = func(name string, args ...string) error { return nil }

	// Build executor with mocks
	mp := &mockProvider{generateFunc: func(ctx context.Context, req *provider.CodeRequest) (*provider.CodeResponse, error) {
		if req == nil || req.RepoPath == "" || req.Prompt == "" {
			t.Fatalf("provider got invalid request: %+v", req)
		}
		return &provider.CodeResponse{Summary: "ok"}, nil
	}}
	ma := &mockAuthProvider{}
	ex := New(mp, ma)
	ex.fetcher = &mockFetcher{fetchFunc: func(ctx context.Context, gctx *github.Context) (*ghdata.FetchResult, error) {
		// Return proper PullRequest data since buildTestCtx(true) creates a PR context
		return &ghdata.FetchResult{
			ContextData: ghdata.PullRequest{
				Title:       "Test PR",
				Body:        "Test body",
				Author:      ghdata.Author{Login: "testuser"},
				BaseRefName: "main",
				HeadRefName: "feature",
				State:       "open",
			},
		}, nil
	}}

	if err := ex.Execute(context.Background(), buildTestCtx(true)); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
}

func TestExecute_AuthFailure(t *testing.T) {
	origClone := cloneRepo
	origRun := runCmd
	defer func() { cloneRepo = origClone; runCmd = origRun }()

	// No cloning should occur, but keep safe defaults
	cloneRepo = func(repo, branch, token string) (string, func(), error) {
		return t.TempDir(), func() {}, nil
	}
	runCmd = func(name string, args ...string) error { return nil }

	mp := &mockProvider{}
	ma := &mockAuthProvider{tokenFunc: func(repo string) (*github.InstallationToken, error) {
		return nil, errors.New("boom")
	}}
	ex := New(mp, ma)
	ex.fetcher = &mockFetcher{}

	err := ex.Execute(context.Background(), buildTestCtx(false))
	if err == nil || !containsErr(err, "authenticate GitHub app") {
		t.Fatalf("Execute() err = %v, want contains 'authenticate GitHub app'", err)
	}
}

func TestExecute_FetchFailure(t *testing.T) {
	origClone := cloneRepo
	origRun := runCmd
	defer func() { cloneRepo = origClone; runCmd = origRun }()

	cloneRepo = func(repo, branch, token string) (string, func(), error) {
		return t.TempDir(), func() {}, nil
	}
	runCmd = func(name string, args ...string) error { return nil }

	mp := &mockProvider{}
	ma := &mockAuthProvider{}
	ex := New(mp, ma)
	ex.fetcher = &mockFetcher{fetchFunc: func(ctx context.Context, gctx *github.Context) (*ghdata.FetchResult, error) {
		return nil, errors.New("fetch fail")
	}}

	err := ex.Execute(context.Background(), buildTestCtx(true))
	if err == nil || !containsErr(err, "fetch GitHub data") {
		t.Fatalf("Execute() err = %v, want contains 'fetch GitHub data'", err)
	}
}

func TestExecute_CloneFailure(t *testing.T) {
	origClone := cloneRepo
	origRun := runCmd
	defer func() { cloneRepo = origClone; runCmd = origRun }()

	cloneRepo = func(repo, branch, token string) (string, func(), error) {
		return "", nil, errors.New("clone fail")
	}
	runCmd = func(name string, args ...string) error { return nil }

	mp := &mockProvider{}
	ma := &mockAuthProvider{}
	ex := New(mp, ma)
	ex.fetcher = &mockFetcher{fetchFunc: func(ctx context.Context, gctx *github.Context) (*ghdata.FetchResult, error) {
		return &ghdata.FetchResult{
			ContextData: ghdata.Issue{
				Title:  "Test Issue",
				Body:   "Test body",
				Author: ghdata.Author{Login: "testuser"},
				State:  "open",
			},
		}, nil
	}}

	err := ex.Execute(context.Background(), buildTestCtx(false))
	if err == nil || !containsErr(err, "clone repository") {
		t.Fatalf("Execute() err = %v, want contains 'clone repository'", err)
	}
}

func TestExecute_BranchCreationFailure(t *testing.T) {
	origClone := cloneRepo
	origRun := runCmd
	defer func() { cloneRepo = origClone; runCmd = origRun }()

	cloneRepo = func(repo, branch, token string) (string, func(), error) {
		return t.TempDir(), func() {}, nil
	}
	runCmd = func(name string, args ...string) error {
		// Fail specifically on checkout -b
		if name == "git" && len(args) >= 4 {
			joined := fmt.Sprint(args)
			if contains(joined, "checkout") && contains(joined, "-b") {
				return errors.New("branch fail")
			}
		}
		return nil
	}

	mp := &mockProvider{}
	ma := &mockAuthProvider{}
	ex := New(mp, ma)
	ex.fetcher = &mockFetcher{fetchFunc: func(ctx context.Context, gctx *github.Context) (*ghdata.FetchResult, error) {
		return &ghdata.FetchResult{
			ContextData: ghdata.Issue{
				Title:  "Test Issue",
				Body:   "Test body",
				Author: ghdata.Author{Login: "testuser"},
				State:  "open",
			},
		}, nil
	}}

	err := ex.Execute(context.Background(), buildTestCtx(false))
	if err == nil || !containsErr(err, "create feature branch") {
		t.Fatalf("Execute() err = %v, want contains 'create feature branch'", err)
	}
}

func TestExecute_ProviderFailure(t *testing.T) {
	origClone := cloneRepo
	origRun := runCmd
	defer func() { cloneRepo = origClone; runCmd = origRun }()

	cloneRepo = func(repo, branch, token string) (string, func(), error) {
		return t.TempDir(), func() {}, nil
	}
	runCmd = func(name string, args ...string) error { return nil }

	mp := &mockProvider{generateFunc: func(ctx context.Context, req *provider.CodeRequest) (*provider.CodeResponse, error) {
		return nil, errors.New("provider fail")
	}, name: "mockp"}
	ma := &mockAuthProvider{}
	ex := New(mp, ma)
	ex.fetcher = &mockFetcher{fetchFunc: func(ctx context.Context, gctx *github.Context) (*ghdata.FetchResult, error) {
		return &ghdata.FetchResult{
			ContextData: ghdata.Issue{
				Title:  "Test Issue",
				Body:   "Test body",
				Author: ghdata.Author{Login: "testuser"},
				State:  "open",
			},
		}, nil
	}}

	err := ex.Execute(context.Background(), buildTestCtx(false))
	if err == nil || !containsErr(err, "provider") {
		t.Fatalf("Execute() err = %v, want contains 'provider'", err)
	}
}

func TestExecute_EmptyRepoFallback(t *testing.T) {
	origClone := cloneRepo
	origRun := runCmd
	defer func() { cloneRepo = origClone; runCmd = origRun }()

	var cloneRepoArg string
	cloneRepo = func(repo, branch, token string) (string, func(), error) {
		cloneRepoArg = repo
		return t.TempDir(), func() {}, nil
	}
	runCmd = func(name string, args ...string) error { return nil }

	mp := &mockProvider{}
	ma := &mockAuthProvider{}
	ex := New(mp, ma)
	ex.fetcher = &mockFetcher{fetchFunc: func(ctx context.Context, gctx *github.Context) (*ghdata.FetchResult, error) {
		return &ghdata.FetchResult{
			ContextData: ghdata.PullRequest{
				Title:       "Test PR",
				Body:        "Test body",
				Author:      ghdata.Author{Login: "testuser"},
				BaseRefName: "main",
				HeadRefName: "feature",
				State:       "open",
			},
		}, nil
	}}

	// Build a ctx with empty FullName to trigger fallback
	ctx := buildTestCtx(true)
	ctx.Repository.FullName = ""

	if err := ex.Execute(context.Background(), ctx); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	// Verify auth received fallback owner/name and clone used it too
	if ma.lastRepo != "owner/repo" {
		t.Fatalf("auth repo = %q, want 'owner/repo'", ma.lastRepo)
	}
	if cloneRepoArg != "owner/repo" {
		t.Fatalf("clone repo = %q, want 'owner/repo'", cloneRepoArg)
	}
}

func TestExecute_GitRemoteConfiguration(t *testing.T) {
	origClone := cloneRepo
	origRun := runCmd
	defer func() { cloneRepo = origClone; runCmd = origRun }()

	cloneRepo = func(repo, branch, token string) (string, func(), error) {
		return t.TempDir(), func() {}, nil
	}

	// Track git commands to verify remote set-url was called
	var gitRemoteSetURLCalled bool
	var remoteURLValue string

	runCmd = func(name string, args ...string) error {
		if name == "git" && len(args) >= 4 {
			// Check for "git remote set-url origin <url>"
			if args[2] == "remote" && args[3] == "set-url" {
				gitRemoteSetURLCalled = true
				if len(args) >= 6 {
					remoteURLValue = args[5]
				}
			}
		}
		return nil
	}

	mp := &mockProvider{}
	ma := &mockAuthProvider{}
	ex := New(mp, ma)
	ex.fetcher = &mockFetcher{fetchFunc: func(ctx context.Context, gctx *github.Context) (*ghdata.FetchResult, error) {
		return &ghdata.FetchResult{
			ContextData: ghdata.Issue{
				Title:  "Test Issue",
				Body:   "Test body",
				Author: ghdata.Author{Login: "testuser"},
				State:  "open",
			},
		}, nil
	}}

	if err := ex.Execute(context.Background(), buildTestCtx(false)); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if !gitRemoteSetURLCalled {
		t.Fatal("git remote set-url was not called")
	}

	// Verify the URL contains token and repo
	if !contains(remoteURLValue, "x-access-token:test-token") {
		t.Errorf("remote URL = %q, want to contain x-access-token:test-token", remoteURLValue)
	}
	if !contains(remoteURLValue, "github.com/owner/repo.git") {
		t.Errorf("remote URL = %q, want to contain github.com/owner/repo.git", remoteURLValue)
	}
}

func TestExecute_GitRemoteConfigurationFailure(t *testing.T) {
	origClone := cloneRepo
	origRun := runCmd
	defer func() { cloneRepo = origClone; runCmd = origRun }()

	cloneRepo = func(repo, branch, token string) (string, func(), error) {
		return t.TempDir(), func() {}, nil
	}

	runCmd = func(name string, args ...string) error {
		// Fail on git remote set-url
		if name == "git" && len(args) >= 4 {
			if args[2] == "remote" && args[3] == "set-url" {
				return errors.New("remote config fail")
			}
		}
		return nil
	}

	mp := &mockProvider{}
	ma := &mockAuthProvider{}
	ex := New(mp, ma)
	ex.fetcher = &mockFetcher{fetchFunc: func(ctx context.Context, gctx *github.Context) (*ghdata.FetchResult, error) {
		return &ghdata.FetchResult{
			ContextData: ghdata.Issue{
				Title:  "Test Issue",
				Body:   "Test body",
				Author: ghdata.Author{Login: "testuser"},
				State:  "open",
			},
		}, nil
	}}

	err := ex.Execute(context.Background(), buildTestCtx(false))
	if err == nil || !containsErr(err, "configure git remote with token") {
		t.Fatalf("Execute() err = %v, want contains 'configure git remote with token'", err)
	}
}

// small helpers
func containsErr(err error, substr string) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), substr)
}

// TestExecute_PRContext_UsesHeadBranchFromFetchedData verifies that when PreparedBranch
// is empty in a PR context (e.g., issue_comment webhook), the executor extracts the head
// branch from fetched PR data instead of generating a new branch name.
func TestExecute_PRContext_UsesHeadBranchFromFetchedData(t *testing.T) {
	origClone := cloneRepo
	origRun := runCmd
	origLsRemote := gitLsRemoteHeads
	defer func() {
		cloneRepo = origClone
		runCmd = origRun
		gitLsRemoteHeads = origLsRemote
	}()

	// Mock cloneRepo
	cloneRepo = func(repo, branch, token string) (string, func(), error) {
		return t.TempDir(), func() {}, nil
	}

	// Track git commands to verify checkout uses correct branch
	var gitCheckoutCalled bool
	var checkoutBranch string
	runCmd = func(name string, args ...string) error {
		if name == "git" && len(args) > 0 {
			// Handle git -C workdir checkout ...
			cmdStart := 0
			if len(args) > 2 && args[0] == "-C" {
				cmdStart = 2
			}

			if args[cmdStart] == "checkout" {
				gitCheckoutCalled = true
				// Extract branch from: git checkout -b <branch> [origin/branch]
				if cmdStart+1 < len(args) && args[cmdStart+1] == "-b" && cmdStart+2 < len(args) {
					checkoutBranch = args[cmdStart+2]
				}
			}
		}
		return nil
	}

	gitLsRemoteHeads = func(workdir, pattern string) ([]string, error) {
		if pattern == "feature/auth-fix" {
			return []string{"refs/heads/feature/auth-fix"}, nil
		}
		return nil, nil
	}

	mp := &mockProvider{generateFunc: func(ctx context.Context, req *provider.CodeRequest) (*provider.CodeResponse, error) {
		return &provider.CodeResponse{Summary: "ok"}, nil
	}}
	ma := &mockAuthProvider{}
	ex := New(mp, ma)

	// Mock fetcher to return PR data with HeadRefName
	ex.fetcher = &mockFetcher{fetchFunc: func(ctx context.Context, gctx *github.Context) (*ghdata.FetchResult, error) {
		return &ghdata.FetchResult{
			ContextData: ghdata.PullRequest{
				Title:       "Test PR",
				Body:        "Test body",
				Author:      ghdata.Author{Login: "testuser"},
				BaseRefName: "main",
				HeadRefName: "feature/auth-fix", // This should be used for checkout
				State:       "OPEN",
			},
		}, nil
	}}

	// Create PR context but with empty PreparedBranch (simulating issue_comment webhook)
	ctx := buildTestCtx(true)
	ctx.PreparedBranch = "" // Explicitly empty - mode.Prepare() returned empty
	ctx.HeadBranch = ""     // Also empty in webhook context
	ctx.BaseBranch = "main" // Ensure base is set

	// Set PreparedPrompt to avoid hitting prompt builder
	ctx.PreparedPrompt = "test prompt"

	err := ex.Execute(context.Background(), ctx)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	// Verify git checkout was called with the PR's head branch from fetched data
	if !gitCheckoutCalled {
		t.Fatal("Expected git checkout to be called")
	}

	// The branch should be "feature/auth-fix" (from fetched PR data, not generated)
	if checkoutBranch != "feature/auth-fix" {
		t.Fatalf("git checkout branch = %q, want %q", checkoutBranch, "feature/auth-fix")
	}
}

func TestExecute_IssueContext_ReusesExistingBranch(t *testing.T) {
	origClone := cloneRepo
	origRun := runCmd
	origLsRemote := gitLsRemoteHeads
	defer func() {
		cloneRepo = origClone
		runCmd = origRun
		gitLsRemoteHeads = origLsRemote
	}()

	tempDir := t.TempDir()
	cloneRepo = func(repo, branch, token string) (string, func(), error) {
		return tempDir, func() {}, nil
	}

	var checkoutBranch string
	runCmd = func(name string, args ...string) error {
		if name == "git" && len(args) > 0 {
			idx := 0
			if len(args) > 2 && args[0] == "-C" {
				idx = 2
			}
			if args[idx] == "checkout" && idx+1 < len(args) && args[idx+1] == "-b" && idx+2 < len(args) {
				checkoutBranch = args[idx+2]
			}
		}
		return nil
	}

	gitLsRemoteHeads = func(workdir, pattern string) ([]string, error) {
		switch pattern {
		case "swe-agent/2461-*":
			return []string{
				"refs/heads/swe-agent/2461-111",
				"refs/heads/swe-agent/2461-222",
			}, nil
		case "swe-agent/2461-222":
			return []string{"refs/heads/swe-agent/2461-222"}, nil
		default:
			return nil, nil
		}
	}

	mp := &mockProvider{}
	ma := &mockAuthProvider{}
	ex := New(mp, ma)
	ex.fetcher = &mockFetcher{}

	ctx := buildTestCtx(false)
	ctx.IssueNumber = 2461
	ctx.PreparedBranch = ""
	ctx.PreparedPrompt = "prompt"

	if err := ex.Execute(context.Background(), ctx); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if checkoutBranch != "swe-agent/2461-222" {
		t.Fatalf("checkout branch = %q, want %q", checkoutBranch, "swe-agent/2461-222")
	}
	if ctx.PreparedBranch != "swe-agent/2461-222" {
		t.Fatalf("PreparedBranch = %q, want %q", ctx.PreparedBranch, "swe-agent/2461-222")
	}
}

func TestExecute_IssueContext_CreatesBranchWhenNoneExists(t *testing.T) {
	origClone := cloneRepo
	origRun := runCmd
	origLsRemote := gitLsRemoteHeads
	defer func() {
		cloneRepo = origClone
		runCmd = origRun
		gitLsRemoteHeads = origLsRemote
	}()

	tempDir := t.TempDir()
	cloneRepo = func(repo, branch, token string) (string, func(), error) {
		return tempDir, func() {}, nil
	}

	var checkoutBranch string
	runCmd = func(name string, args ...string) error {
		if name == "git" && len(args) > 0 {
			idx := 0
			if len(args) > 2 && args[0] == "-C" {
				idx = 2
			}
			if args[idx] == "checkout" && idx+1 < len(args) && args[idx+1] == "-b" && idx+2 < len(args) {
				checkoutBranch = args[idx+2]
			}
		}
		return nil
	}

	gitLsRemoteHeads = func(workdir, pattern string) ([]string, error) {
		return nil, nil
	}

	mp := &mockProvider{}
	ma := &mockAuthProvider{}
	ex := New(mp, ma)
	ex.fetcher = &mockFetcher{}

	ctx := buildTestCtx(false)
	ctx.IssueNumber = 2461
	ctx.PreparedBranch = ""
	ctx.PreparedPrompt = "prompt"

	if err := ex.Execute(context.Background(), ctx); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if !strings.HasPrefix(checkoutBranch, "swe-agent/2461-") {
		t.Fatalf("checkout branch = %q, want prefix swe-agent/2461-", checkoutBranch)
	}
	if ctx.PreparedBranch != checkoutBranch {
		t.Fatalf("PreparedBranch = %q, want %q", ctx.PreparedBranch, checkoutBranch)
	}
}

func TestExecute_IssueContext_CreatesBranchWhenLsRemoteFails(t *testing.T) {
	origClone := cloneRepo
	origRun := runCmd
	origLsRemote := gitLsRemoteHeads
	defer func() {
		cloneRepo = origClone
		runCmd = origRun
		gitLsRemoteHeads = origLsRemote
	}()

	tempDir := t.TempDir()
	cloneRepo = func(repo, branch, token string) (string, func(), error) {
		return tempDir, func() {}, nil
	}

	var checkoutBranch string
	runCmd = func(name string, args ...string) error {
		if name == "git" && len(args) > 0 {
			idx := 0
			if len(args) > 2 && args[0] == "-C" {
				idx = 2
			}
			if args[idx] == "checkout" && idx+1 < len(args) && args[idx+1] == "-b" && idx+2 < len(args) {
				checkoutBranch = args[idx+2]
			}
		}
		return nil
	}

	gitLsRemoteHeads = func(workdir, pattern string) ([]string, error) {
		return nil, errors.New("remote unavailable")
	}

	mp := &mockProvider{}
	ma := &mockAuthProvider{}
	ex := New(mp, ma)
	ex.fetcher = &mockFetcher{}

	ctx := buildTestCtx(false)
	ctx.IssueNumber = 2462
	ctx.PreparedPrompt = "prompt"

	if err := ex.Execute(context.Background(), ctx); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}

	if !strings.HasPrefix(checkoutBranch, "swe-agent/2462-") {
		t.Fatalf("checkout branch = %q, want prefix swe-agent/2462-", checkoutBranch)
	}
	if ctx.PreparedBranch != checkoutBranch {
		t.Fatalf("PreparedBranch = %q, want %q", ctx.PreparedBranch, checkoutBranch)
	}
}

func TestFindExistingIssueBranch_IgnoresInvalidRefs(t *testing.T) {
	origLsRemote := gitLsRemoteHeads
	defer func() { gitLsRemoteHeads = origLsRemote }()

	gitLsRemoteHeads = func(workdir, pattern string) ([]string, error) {
		return []string{
			"refs/heads/swe-agent/2461-invalid",
			"refs/heads/other-branch",
		}, nil
	}

	ctx := buildTestCtx(false)
	ctx.IssueNumber = 2461

	got, err := findExistingIssueBranch(ctx, "/tmp")
	if err != nil {
		t.Fatalf("findExistingIssueBranch err = %v, want nil", err)
	}
	if got != "" {
		t.Fatalf("findExistingIssueBranch = %q, want empty string", got)
	}
}

func TestFindExistingIssueBranch_ErrorPropagation(t *testing.T) {
	origLsRemote := gitLsRemoteHeads
	defer func() { gitLsRemoteHeads = origLsRemote }()

	gitLsRemoteHeads = func(workdir, pattern string) ([]string, error) {
		return nil, errors.New("boom")
	}

	ctx := buildTestCtx(false)
	ctx.IssueNumber = 2461

	got, err := findExistingIssueBranch(ctx, "/tmp")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got != "" {
		t.Fatalf("branch = %q, want empty on error", got)
	}
}

func TestCheckoutRemoteBranch_FallbackToFetchHead(t *testing.T) {
	origRun := runCmd
	defer func() { runCmd = origRun }()

	call := 0
	runCmd = func(name string, args ...string) error {
		call++
		if call == 1 {
			return fmt.Errorf("origin ref missing")
		}
		if call == 2 {
			if len(args) >= 6 && args[5] == "FETCH_HEAD" {
				return nil
			}
			t.Fatalf("unexpected args on fallback: %v", args)
		}
		return fmt.Errorf("unexpected call %d", call)
	}

	if err := checkoutRemoteBranch("/tmp", "feat/monorepo"); err != nil {
		t.Fatalf("checkoutRemoteBranch err = %v, want nil", err)
	}
	if call != 2 {
		t.Fatalf("expected 2 runCmd calls, got %d", call)
	}
}

func TestCheckoutRemoteBranch_FallbackFailure(t *testing.T) {
	origRun := runCmd
	defer func() { runCmd = origRun }()

	runCmd = func(name string, args ...string) error {
		return fmt.Errorf("fail")
	}

	err := checkoutRemoteBranch("/tmp", "feat/monorepo")
	if err == nil || !strings.Contains(err.Error(), "checkout remote branch") {
		t.Fatalf("expected checkout error, got %v", err)
	}
}
