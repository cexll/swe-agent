package comment

import (
	"context"
	"fmt"

	"github.com/google/go-github/v66/github"
)

// Tracker 负责在一个 Issue/PR 上创建并维护协调用的评论。
type Tracker struct {
	client    *github.Client
	owner     string
	repo      string
	number    int
	commentID int64
}

// NewTracker 创建评论追踪器
func NewTracker(client *github.Client, owner, repo string, number int) *Tracker {
	return &Tracker{
		client: client,
		owner:  owner,
		repo:   repo,
		number: number,
	}
}

// CreateInitial 创建初始协调评论（带 spinner）
func (t *Tracker) CreateInitial(ctx context.Context) (int64, error) {
	if t == nil || t.client == nil {
		return 0, fmt.Errorf("nil tracker or client")
	}
	id, err := createInitialComment(ctx, t.client, t.owner, t.repo, t.number)
	if err != nil {
		return 0, err
	}
	t.commentID = id
	return id, nil
}

// Update 更新评论内容（主要由 AI 通过 MCP 调用，这里保留备用）
func (t *Tracker) Update(ctx context.Context, body string) error {
	if t == nil || t.client == nil {
		return fmt.Errorf("nil tracker or client")
	}
	if t.commentID == 0 {
		return fmt.Errorf("comment not created")
	}
	_, _, err := t.client.Issues.EditComment(ctx, t.owner, t.repo, t.commentID, &github.IssueComment{Body: &body})
	return err
}

// GetCommentID 获取当前评论 ID
func (t *Tracker) GetCommentID() int64 { return t.commentID }
