package branch

import (
	"context"
	"fmt"

	"github.com/google/go-github/v66/github"
)

// Manager 分支管理器
type Manager struct {
	client *github.Client
	owner  string
	repo   string
}

// NewManager 创建分支管理器
func NewManager(client *github.Client, owner, repo string) *Manager {
	return &Manager{
		client: client,
		owner:  owner,
		repo:   repo,
	}
}

// CreateBranch 创建分支
// baseBranch: 基础分支（如 "main"）
// issueNumber: Issue 编号
// issueTitle: Issue 标题
// 返回创建的分支名
func (m *Manager) CreateBranch(ctx context.Context, baseBranch string, issueNumber int, issueTitle string) (string, error) {
	// 1. 生成分支名
	branchName := GenerateBranchName(issueNumber, issueTitle)

	// 2. 验证分支名
	if !ValidateBranchName(branchName) {
		return "", fmt.Errorf("invalid branch name: %s", branchName)
	}

	// 3. 检查分支是否已存在
	if _, _, err := m.client.Git.GetRef(ctx, m.owner, m.repo, "refs/heads/"+branchName); err == nil {
		// 分支已存在，直接返回
		return branchName, nil
	}

	// 4. 获取基础分支的 SHA
	baseRef, _, err := m.client.Git.GetRef(ctx, m.owner, m.repo, "refs/heads/"+baseBranch)
	if err != nil {
		return "", fmt.Errorf("failed to get base branch: %w", err)
	}

	// 5. 创建新分支
	ref := &github.Reference{
		Ref: github.String("refs/heads/" + branchName),
		Object: &github.GitObject{
			SHA: baseRef.Object.SHA,
		},
	}

	if _, _, err = m.client.Git.CreateRef(ctx, m.owner, m.repo, ref); err != nil {
		return "", fmt.Errorf("failed to create branch: %w", err)
	}

	return branchName, nil
}

// BranchExists 检查分支是否存在
func (m *Manager) BranchExists(ctx context.Context, branchName string) (bool, error) {
	if _, _, err := m.client.Git.GetRef(ctx, m.owner, m.repo, "refs/heads/"+branchName); err != nil {
		// 404 表示不存在
		if _, ok := err.(*github.ErrorResponse); ok {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// DeleteBranch 删除分支
func (m *Manager) DeleteBranch(ctx context.Context, branchName string) error {
	_, err := m.client.Git.DeleteRef(ctx, m.owner, m.repo, "refs/heads/"+branchName)
	return err
}
