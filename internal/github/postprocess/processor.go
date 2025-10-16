package postprocess

import (
	"context"
	"fmt"

	"github.com/google/go-github/v66/github"
)

// Processor 后处理器，执行完成后运行
type Processor struct {
	client      *github.Client
	owner       string
	repo        string
	commentID   int64
	branch      string
	baseBranch  string
	issueNumber int
	isPR        bool
}

// Allow tests to stub side-effectful helpers
var (
	checkBranchStatus      = CheckBranchStatus
	deleteBranch           = DeleteBranch
	updateCommentWithLinks = func(ctx context.Context, client *github.Client, owner, repo string, commentID int64, branchLink, prLink string) error {
		cu := NewCommentUpdater(client, owner, repo)
		return cu.UpdateCommentWithLinks(ctx, commentID, branchLink, prLink)
	}
)

// NewProcessor 创建处理器
func NewProcessor(client *github.Client, owner, repo string, commentID int64, branch, baseBranch string, issueNumber int, isPR bool) *Processor {
	return &Processor{
		client:      client,
		owner:       owner,
		repo:        repo,
		commentID:   commentID,
		branch:      branch,
		baseBranch:  baseBranch,
		issueNumber: issueNumber,
		isPR:        isPR,
	}
}

// Process 执行后处理流程
// 1. 检查分支状态
// 2. 生成分支链接（如果有提交）
// 3. 生成 PR 链接（如果有变更）
// 4. 删除空分支
// 5. 更新协调评论
func (p *Processor) Process(ctx context.Context) error {
	if p.client == nil {
		return fmt.Errorf("nil github client")
	}
	if p.owner == "" || p.repo == "" || p.branch == "" {
		return fmt.Errorf("missing owner/repo/branch")
	}
	// If no coordination comment is provided, we simply skip comment updates;
	// cleanup and other steps proceed as usual below.

	// 1) 检查分支状态
	status, err := checkBranchStatus(ctx, p.client, p.owner, p.repo, p.branch, p.baseBranch)
	if err != nil {
		return err
	}
	if !status.Exists {
		// Nothing to do
		return nil
	}

	// 2/3) 生成链接
	lg := NewLinkGenerator(p.owner, p.repo)
	branchLink := ""
	prLink := ""
	if status.HasCommits {
		branchLink = lg.GenerateBranchLink(p.branch)
		prLink = lg.GeneratePRLink(p.baseBranch, p.branch, p.issueNumber, p.isPR)
	}

	// 4) 删除空分支（0 commits 且 0 files changed）
	if !status.HasCommits {
		if err := deleteBranch(ctx, p.client, p.owner, p.repo, p.branch); err != nil {
			return err
		}
		// Nothing else to update
		return nil
	}

	// 5) 更新协调评论（若有 commentID 且有链接）
	if p.commentID > 0 && (branchLink != "" || prLink != "") {
		if err := updateCommentWithLinks(ctx, p.client, p.owner, p.repo, p.commentID, branchLink, prLink); err != nil {
			return err
		}
	}
	return nil
}
