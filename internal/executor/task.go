// New simplified executor
package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/cexll/swe/internal/github"
	ghdata "github.com/cexll/swe/internal/github/data"
	operations "github.com/cexll/swe/internal/github/operations/git"
	"github.com/cexll/swe/internal/prompt"
	"github.com/cexll/swe/internal/provider"
	"github.com/cexll/swe/internal/toolconfig"
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
var gitLsRemoteHeads = defaultLsRemoteHeads

func New(p provider.Provider, auth github.AuthProvider) *Executor {
	client := ghdata.NewClient(auth)
	return &Executor{
		provider: p,
		auth:     auth,
		fetcher:  ghdata.NewFetcher(client),
	}
}

func (e *Executor) Execute(ctx context.Context, webhookCtx *github.Context) error {
	// 0) Configure Git identity (best-effort)
	if err := operations.ConfigureGitForApp(0, "swe-agent"); err != nil {
		// non-fatal; downstream git commands may still work
		fmt.Printf("[Warn] Configure git failed: %v\n", err)
	}

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
	// Surface token in context for optional MCP clients
	webhookCtx.Token = token.Token

	// 2) Fetch GitHub data via data layer
	fetched, err := e.fetcher.Fetch(ctx, webhookCtx)
	if err != nil {
		return fmt.Errorf("fetch GitHub data: %w", err)
	}

	// 2.5) Fix PR context: If PreparedBranch is empty but we fetched PR data,
	//      extract head branch from GraphQL data (issue_comment webhooks don't provide it)
	if webhookCtx.IsPRContext() && webhookCtx.PreparedBranch == "" {
		if pr, ok := fetched.ContextData.(ghdata.PullRequest); ok {
			webhookCtx.PreparedBranch = pr.HeadRefName
			// Also update BaseBranch if not set
			if webhookCtx.PreparedBaseBranch == "" {
				webhookCtx.PreparedBaseBranch = pr.BaseRefName
			}
		}
	}

	// 3) Clone repository (prefer prepared base branch)
	base := webhookCtx.PreparedBaseBranch
	if base == "" {
		base = webhookCtx.GetBaseBranch()
	}
	if base == "" {
		base = "main"
	}
	workdir, cleanup, err := cloneRepo(repo, base, token.Token)
	if err != nil {
		return fmt.Errorf("clone repository: %w", err)
	}
	defer cleanup()

	// Configure git credential helper to use installation token for push authentication
	// This allows AI to execute "git push" without manual intervention
	remoteURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", token.Token, repo)
	if err := runCmd("git", "-C", workdir, "remote", "set-url", "origin", remoteURL); err != nil {
		return fmt.Errorf("configure git remote with token: %w", err)
	}

	// 4) Checkout task branch
	branch := webhookCtx.PreparedBranch
	if branch == "" && !webhookCtx.IsPRContext() {
		if existing, detectErr := findExistingIssueBranch(webhookCtx, workdir); detectErr != nil {
			fmt.Printf("[Warn] detect existing branch failed: %v\n", detectErr)
		} else if existing != "" {
			branch = existing
			webhookCtx.PreparedBranch = branch
		}
	}
	if branch == "" {
		// 生成新分支名
		branch = featureBranchName(webhookCtx)
		// 设置到 context 中，供 prompt builder 使用
		webhookCtx.PreparedBranch = branch
	}

	// 如果 branch == base，说明已经在目标分支上（clone 时已 checkout），跳过
	if branch != base {
		// 检查远程分支是否存在（PR 场景会存在）
		refs, lsErr := gitLsRemoteHeads(workdir, branch)
		// 如果 ls-remote 成功且有输出，说明远程分支存在（PR 场景）
		if lsErr == nil && len(refs) > 0 {
			// 远程分支存在：强制 fetch 该分支到本地 tracking ref
			refspec := fmt.Sprintf("refs/heads/%s:refs/remotes/origin/%s", branch, branch)
			if err := runCmd("git", "-C", workdir, "fetch", "origin", refspec); err != nil {
				return fmt.Errorf("fetch remote branch: %w", err)
			}
			if err := checkoutRemoteBranch(workdir, branch); err != nil {
				return err
			}
		} else {
			if lsErr != nil {
				fmt.Printf("[Warn] git ls-remote failed: %v\n", lsErr)
			}
			// 远程分支不存在或 ls-remote 失败：创建新分支（Issue 场景）
			if err := runCmd("git", "-C", workdir, "checkout", "-b", branch); err != nil {
				return fmt.Errorf("create feature branch: %w", err)
			}
		}
	}

	// 5) Build or use prepared prompt (system + GitHub XML)
	fullPrompt := webhookCtx.PreparedPrompt
	if fullPrompt == "" {
		fullPrompt = prompt.BuildPrompt(webhookCtx, fetched)
	}

	// 6) Call provider.GenerateCode (pass token via context + env for MCP)
	// 6) Inject MCP-friendly environment variables
	// Set env for child tools (best-effort; provider also sets from req.Context)
	_ = os.Setenv("GITHUB_PERSONAL_ACCESS_TOKEN", token.Token)
	_ = os.Setenv("GITHUB_TOKEN", token.Token)
	_ = os.Setenv("GH_TOKEN", token.Token)
	_ = os.Setenv("REPO_DIR", workdir)

	// Build context map for provider (including MCP config data)
	ctxMap := map[string]string{
		"github_token": token.Token,
		"repository":   repo,
		"base_branch":  base,
		"head_branch":  webhookCtx.GetHeadBranch(),
	}

	// Add MCP comment server context if available
	if webhookCtx.PreparedCommentID > 0 {
		ctxMap["comment_id"] = fmt.Sprintf("%d", webhookCtx.PreparedCommentID)
		ctxMap["repo_owner"] = webhookCtx.GetRepositoryOwner()
		ctxMap["repo_name"] = webhookCtx.GetRepositoryName()
		if webhookCtx.EventName != "" {
			ctxMap["event_name"] = string(webhookCtx.EventName)
		}
	}
	if webhookCtx.IsPRContext() {
		if n := webhookCtx.GetPRNumber(); n != 0 {
			ctxMap["pr_number"] = fmt.Sprintf("%d", n)
		}
	} else if n := webhookCtx.GetIssueNumber(); n != 0 {
		ctxMap["issue_number"] = fmt.Sprintf("%d", n)
	}

	// Build tool configuration
	toolOpts := toolconfig.Options{
		UseCommitSigning:       getEnvBool("USE_COMMIT_SIGNING", false),
		EnableGitHubCommentMCP: true, // default enable comment MCP for coordinator
		EnableGitHubFileOpsMCP: getEnvBool("ENABLE_GITHUB_MCP_FILES", false),
		EnableGitHubCIMCP:      getEnvBool("ENABLE_GITHUB_MCP_CI", false),
	}
	allowedTools := toolconfig.BuildAllowedTools(toolOpts)
	disallowedTools := toolconfig.BuildDisallowedTools(toolOpts)

	// Log tool configuration for debugging
	if len(allowedTools) > 0 {
		fmt.Printf("[Tools] Allowed (%d): %s\n", len(allowedTools), joinCSV(allowedTools))
	}
	if len(disallowedTools) > 0 {
		fmt.Printf("[Tools] Disallowed (%d): %s\n", len(disallowedTools), joinCSV(disallowedTools))
	}

	_, err = e.provider.GenerateCode(ctx, &provider.CodeRequest{
		Prompt:          fullPrompt,
		RepoPath:        workdir,
		Context:         ctxMap,
		AllowedTools:    allowedTools,
		DisallowedTools: disallowedTools,
	})
	if err != nil {
		return fmt.Errorf("provider %s: %w", e.provider.Name(), err)
	}

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

func checkoutRemoteBranch(workdir, branch string) error {
	remoteBranch := fmt.Sprintf("origin/%s", branch)
	if err := runCmd("git", "-C", workdir, "checkout", "-b", branch, remoteBranch); err == nil {
		return nil
	}
	// fallback: use FETCH_HEAD when remote tracking 未创建（单分支 clone 常见）
	if err := runCmd("git", "-C", workdir, "checkout", "-b", branch, "FETCH_HEAD"); err != nil {
		return fmt.Errorf("checkout remote branch: %w", err)
	}
	return nil
}

func defaultLsRemoteHeads(workdir, pattern string) ([]string, error) {
	args := []string{"-C", workdir, "ls-remote", "--heads", "origin"}
	if pattern != "" {
		args = append(args, pattern)
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git ls-remote: %w\n%s", err, string(out))
	}
	outStr := strings.TrimSpace(string(out))
	if outStr == "" {
		return nil, nil
	}
	lines := strings.Split(outStr, "\n")
	refs := make([]string, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			refs = append(refs, fields[1])
		}
	}
	return refs, nil
}

func findExistingIssueBranch(ctx *github.Context, workdir string) (string, error) {
	issueNumber := ctx.GetIssueNumber()
	if issueNumber <= 0 {
		return "", nil
	}
	pattern := fmt.Sprintf("swe-agent/%d-*", issueNumber)
	refs, err := gitLsRemoteHeads(workdir, pattern)
	if err != nil {
		return "", err
	}
	if len(refs) == 0 {
		return "", nil
	}

	prefix := fmt.Sprintf("swe-agent/%d-", issueNumber)
	var latestBranch string
	var latestTs int64
	for _, ref := range refs {
		name := strings.TrimPrefix(ref, "refs/heads/")
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		tsPart := strings.TrimPrefix(name, prefix)
		ts, parseErr := strconv.ParseInt(tsPart, 10, 64)
		if parseErr != nil {
			continue
		}
		if ts > latestTs {
			latestTs = ts
			latestBranch = name
		}
	}
	return latestBranch, nil
}

// helpers local to executor to avoid importing config here
func getEnvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "TRUE", "True", "yes", "Y", "y":
		return true
	case "0", "false", "FALSE", "False", "no", "N", "n":
		return false
	default:
		return def
	}
}

func joinCSV(items []string) string {
	if len(items) == 0 {
		return ""
	}
	// Simple join without pulling in additional helpers
	b := make([]byte, 0, 64)
	for i, s := range items {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, s...)
	}
	return string(b)
}
