package command

import (
	"context"
	"fmt"
	"strings"

	ghpkg "github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/github/comment"
	"github.com/cexll/swe/internal/modes"
)

// Mode 实现 Command 模式（/code 命令触发）
type Mode struct{}

// Name 返回模式名称
func (m *Mode) Name() string { return "command" }

// ShouldTrigger 检测是否包含 /code 命令
func (m *Mode) ShouldTrigger(ctx *ghpkg.Context) bool {
	return containsCommand(ctx.GetTriggerCommentBody(), "/code")
}

// Prepare 准备执行上下文
func (m *Mode) Prepare(ctx context.Context, ghCtx *ghpkg.Context) (*modes.PrepareResult, error) {
	// 1. 创建 GitHub 客户端
	client := ghCtx.NewGitHubClient()

	// 2. 创建简单的初始协调评论（即时反馈）
	tracker := comment.NewTracker(client, ghCtx.Repository.Owner, ghCtx.Repository.Name, ghCtx.IssueNumber)
	commentID, err := tracker.CreateInitial(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create initial comment: %w", err)
	}

	// 3. 返回结果（不预先创建分支，让 AI 通过 MCP 自主创建）
	base := ghCtx.GetBaseBranch()
	if strings.TrimSpace(base) == "" {
		base = ghCtx.GetRepositoryDefaultBranch()
	}

	return &modes.PrepareResult{
		CommentID:  commentID,
		Branch:     "", // 留空，AI 会通过 MCP 自己创建分支
		BaseBranch: base,
		Prompt:     "", // 留空，Executor 会统一构建 Prompt
	}, nil
}

// containsCommand 检查文本是否包含指定命令（忽略大小写）
func containsCommand(text, command string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(command))
}

// init 自动注册 Command 模式
func init() {
	modes.Register(&Mode{})
}
