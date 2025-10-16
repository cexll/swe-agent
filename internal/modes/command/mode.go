package command

import (
	"context"
	"fmt"
	"strings"

	ghpkg "github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/github/branch"
	"github.com/cexll/swe/internal/github/comment"
	"github.com/cexll/swe/internal/modes"
	"github.com/cexll/swe/internal/prompt"
)

// CommandMode 实现 Command 模式（/code 命令触发）
type CommandMode struct{}

// Name 返回模式名称
func (m *CommandMode) Name() string { return "command" }

// ShouldTrigger 检测是否包含 /code 命令
func (m *CommandMode) ShouldTrigger(ctx *ghpkg.Context) bool {
	return containsCommand(ctx.GetTriggerCommentBody(), "/code")
}

// Prepare 准备执行上下文
func (m *CommandMode) Prepare(ctx context.Context, ghCtx *ghpkg.Context) (*modes.PrepareResult, error) {
	// 1. 创建 GitHub 客户端
	client := ghCtx.NewGitHubClient()

	// 2. 创建初始协调评论
	tracker := comment.NewTracker(client, ghCtx.Repository.Owner, ghCtx.Repository.Name, ghCtx.IssueNumber)
	commentID, err := tracker.CreateInitial(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create initial comment: %w", err)
	}

	// 3. 创建分支（使用智能命名）
	branchMgr := branch.NewManager(client, ghCtx.Repository.Owner, ghCtx.Repository.Name)

	// 获取 Issue 标题（如果 Context 中没有，使用默认值）
	issueTitle := ghCtx.IssueTitle
	if strings.TrimSpace(issueTitle) == "" {
		issueTitle = fmt.Sprintf("issue-%d", ghCtx.IssueNumber)
	}

	base := ghCtx.GetBaseBranch()
	if strings.TrimSpace(base) == "" {
		base = ghCtx.GetRepositoryDefaultBranch()
	}
	branchName, err := branchMgr.CreateBranch(ctx, base, ghCtx.GetIssueNumber(), issueTitle)
	if err != nil {
		return nil, fmt.Errorf("failed to create branch: %w", err)
	}

	// 4. 构建完整 Prompt（传递 context 以支持图片下载）
	fullPrompt, err := prompt.BuildFullPrompt(ctx, ghCtx, commentID, branchName)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	return &modes.PrepareResult{
		CommentID:  commentID,
		Branch:     branchName,
		BaseBranch: base,
		Prompt:     fullPrompt,
	}, nil
}

// containsCommand 检查文本是否包含指定命令（忽略大小写）
func containsCommand(text, command string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(command))
}

// init 自动注册 Command 模式
func init() {
	modes.Register(&CommandMode{})
}
