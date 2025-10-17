package modes

import (
	"context"

	"github.com/cexll/swe/internal/github"
)

// Mode 定义执行模式接口
type Mode interface {
	// Name 返回模式名称
	Name() string

	// ShouldTrigger 判断是否应该触发此模式
	ShouldTrigger(ctx *github.Context) bool

	// Prepare 准备执行上下文（创建评论、分支等）
	Prepare(ctx context.Context, ghCtx *github.Context) (*PrepareResult, error)
}

// PrepareResult 准备阶段的结果
type PrepareResult struct {
	CommentID  int64  // 创建的协调评论 ID
	Branch     string // 创建的分支名
	BaseBranch string // 基础分支
	Prompt     string // 构建的完整 prompt
}
