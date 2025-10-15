// New simplified executor
package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/cexll/swe/internal/github"
	ghdata "github.com/cexll/swe/internal/github/data"
	"github.com/cexll/swe/internal/prompt"
	"github.com/cexll/swe/internal/provider"
)

type fetcherIface interface {
	Fetch(ctx context.Context, gctx *github.Context) (*ghdata.FetchResult, error)
}

type Executor struct {
	provider provider.Provider
	auth     github.AuthProvider
	fetcher  fetcherIface
}

// allow tests to stub cloning and command execution
var cloneRepo = github.Clone
var runCmd = run

func New(p provider.Provider, auth github.AuthProvider) *Executor {
	client := ghdata.NewClient(auth)
	return &Executor{
		provider: p,
		auth:     auth,
		fetcher:  ghdata.NewFetcher(client),
	}
}

func (e *Executor) Execute(ctx context.Context, webhookCtx *github.Context) error {
	// 1) Authenticate (GitHub App → installation token)
	repo := webhookCtx.GetRepositoryFullName()
	if repo == "" {
		// fall back to owner/name if needed
		repo = fmt.Sprintf("%s/%s", webhookCtx.GetRepositoryOwner(), webhookCtx.GetRepositoryName())
	}
	token, err := e.auth.GetInstallationToken(repo)
	if err != nil {
		return fmt.Errorf("authenticate GitHub app: %w", err)
	}

	// 2) Fetch GitHub data via data layer
	fetched, err := e.fetcher.Fetch(ctx, webhookCtx)
	if err != nil {
		return fmt.Errorf("fetch GitHub data: %w", err)
	}

	// 3) Clone repository (base branch when available)
	base := webhookCtx.GetBaseBranch()
	if base == "" {
		base = "main"
	}
	workdir, cleanup, err := cloneRepo(repo, base, token.Token)
	if err != nil {
		return fmt.Errorf("clone repository: %w", err)
	}
	defer cleanup()

	// 4) Create feature branch
	branch := featureBranchName(webhookCtx)
	if err := runCmd("git", "-C", workdir, "checkout", "-b", branch); err != nil {
		return fmt.Errorf("create feature branch: %w", err)
	}

	// 5) Build prompt (system + GitHub XML)
	fullPrompt := prompt.BuildPrompt(webhookCtx, fetched)

	// 6) Call provider.GenerateCode (pass token via context + env for MCP)
	// Set env for child tools (best-effort; provider also sets from req.Context)
	os.Setenv("GITHUB_TOKEN", token.Token)
	os.Setenv("GH_TOKEN", token.Token)

	ctxMap := map[string]string{
		"github_token": token.Token,
		"repository":   repo,
		"base_branch":  base,
		"head_branch":  webhookCtx.GetHeadBranch(),
	}
	if webhookCtx.IsPRContext() {
		if n := webhookCtx.GetPRNumber(); n != 0 {
			ctxMap["pr_number"] = fmt.Sprintf("%d", n)
		}
	} else if n := webhookCtx.GetIssueNumber(); n != 0 {
		ctxMap["issue_number"] = fmt.Sprintf("%d", n)
	}

	_, err = e.provider.GenerateCode(ctx, &provider.CodeRequest{
		Prompt:   fullPrompt,
		RepoPath: workdir,
		Context:  ctxMap,
	})
	if err != nil {
		return fmt.Errorf("provider %s: %w", e.provider.Name(), err)
	}

	// 7) Done! (AI handles everything via MCP)
	return nil
}

func featureBranchName(ctx *github.Context) string {
	id := ctx.GetIssueNumber()
	if ctx.IsPRContext() && ctx.GetPRNumber() != 0 {
		id = ctx.GetPRNumber()
	}
	if id <= 0 {
		id = int(time.Now().Unix())
	}
	return fmt.Sprintf("swe-agent/%d-%d", id, time.Now().Unix())
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v failed: %v\n%s", name, args, err, string(out))
	}
	return nil
}
