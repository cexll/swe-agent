package postprocess

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v66/github"
)

// CommentUpdater 评论更新器
type CommentUpdater struct {
	client *github.Client
	owner  string
	repo   string
}

// NewCommentUpdater 创建评论更新器
func NewCommentUpdater(client *github.Client, owner, repo string) *CommentUpdater {
	return &CommentUpdater{
		client: client,
		owner:  owner,
		repo:   repo,
	}
}

// UpdateCommentWithLinks 更新评论，添加分支链接和 PR 链接
func (cu *CommentUpdater) UpdateCommentWithLinks(
	ctx context.Context,
	commentID int64,
	branchLink, prLink string,
) error {
	// 1. 获取当前评论内容
	comment, _, err := cu.client.Issues.GetComment(ctx, cu.owner, cu.repo, commentID)
	if err != nil {
		return fmt.Errorf("failed to get comment: %w", err)
	}

	currentBody := comment.GetBody()

	// 2. 检查是否已包含链接（避免重复添加）
	if strings.Contains(currentBody, "[View branch]") || strings.Contains(currentBody, "[Create a PR]") {
		// 已包含链接，不重复添加
		return nil
	}

	// 3. 构建新的评论内容
	newBody := currentBody
	if branchLink != "" {
		newBody += branchLink
	}
	if prLink != "" {
		newBody += prLink
	}

	// 4. 更新评论
	updateReq := &github.IssueComment{
		Body: &newBody,
	}

	_, _, err = cu.client.Issues.EditComment(ctx, cu.owner, cu.repo, commentID, updateReq)
	if err != nil {
		return fmt.Errorf("failed to update comment: %w", err)
	}

	return nil
}
