package postprocess

import (
	"context"
	"fmt"

	"github.com/google/go-github/v66/github"
)

// BranchStatus 表示分支状态
type BranchStatus struct {
	Exists       bool
	HasCommits   bool
	TotalCommits int
	FilesChanged int
	BranchURL    string
}

// CheckBranchStatus 检查分支状态
func CheckBranchStatus(
	ctx context.Context,
	client *github.Client,
	owner, repo, branch, baseBranch string,
) (*BranchStatus, error) {
	status := &BranchStatus{
		Exists:     false,
		HasCommits: false,
	}

	// 1. 检查分支是否存在
	_, resp, err := client.Repositories.GetBranch(ctx, owner, repo, branch, 0)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			// 分支不存在
			return status, nil
		}
		return nil, fmt.Errorf("failed to get branch: %w", err)
	}

	status.Exists = true
	status.BranchURL = fmt.Sprintf("https://github.com/%s/%s/tree/%s", owner, repo, branch)

	// 2. 比较分支与 base 分支的差异
	comparison, _, err := client.Repositories.CompareCommits(
		ctx,
		owner,
		repo,
		baseBranch,
		branch,
		&github.ListOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to compare branches: %w", err)
	}

	status.TotalCommits = comparison.GetTotalCommits()
	if comparison.Files != nil {
		status.FilesChanged = len(comparison.Files)
	}

	// 如果有提交或文件变更，标记为有内容
	status.HasCommits = status.TotalCommits > 0 || status.FilesChanged > 0

	return status, nil
}

// DeleteBranch 删除远程分支
func DeleteBranch(
	ctx context.Context,
	client *github.Client,
	owner, repo, branch string,
) error {
	ref := fmt.Sprintf("heads/%s", branch)
	_, err := client.Git.DeleteRef(ctx, owner, repo, ref)
	if err != nil {
		return fmt.Errorf("failed to delete branch %s: %w", branch, err)
	}
	return nil
}
