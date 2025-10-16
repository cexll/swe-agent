package branch

import (
	"context"
	"strings"
	"time"

	"github.com/google/go-github/v66/github"
)

// CleanupOptions 清理选项
type CleanupOptions struct {
	// MaxAge 分支最大保留时间
	MaxAge time.Duration

	// DryRun 是否只模拟，不实际删除
	DryRun bool

	// Prefix 分支前缀过滤（默认 "swe/")
	Prefix string
}

// CleanupOldBranches 清理旧的 swe/* 分支
// 返回删除的分支列表
func (m *Manager) CleanupOldBranches(ctx context.Context, opts CleanupOptions) ([]string, error) {
	// 设置默认值
	if opts.Prefix == "" {
		opts.Prefix = "swe/"
	}
	if opts.MaxAge == 0 {
		opts.MaxAge = 30 * 24 * time.Hour // 默认 30 天
	}

	// 1. 列出所有匹配前缀的分支
	branches, err := m.listBranchesByPrefix(ctx, opts.Prefix)
	if err != nil {
		return nil, err
	}

	deleted := []string{}
	now := time.Now()

	// 2. 检查每个分支的最后提交时间
	for _, branch := range branches {
		// 获取分支最后提交
		commit, _, err := m.client.Repositories.GetCommit(ctx, m.owner, m.repo, branch.GetObject().GetSHA(), nil)
		if err != nil {
			continue
		}

		// 计算分支年龄
		commitDate := commit.GetCommit().GetAuthor().GetDate().Time
		age := now.Sub(commitDate)

		// 超过最大保留时间则删除
		if age > opts.MaxAge {
			branchName := strings.TrimPrefix(branch.GetRef(), "refs/heads/")

			if !opts.DryRun {
				if err := m.DeleteBranch(ctx, branchName); err != nil {
					continue
				}
			}

			deleted = append(deleted, branchName)
		}
	}

	return deleted, nil
}

// listBranchesByPrefix 列出指定前缀的所有分支
func (m *Manager) listBranchesByPrefix(ctx context.Context, prefix string) ([]*github.Reference, error) {
	opts := &github.ReferenceListOptions{
		Ref: "heads/" + prefix,
	}

	refs, _, err := m.client.Git.ListMatchingRefs(ctx, m.owner, m.repo, opts)
	if err != nil {
		return nil, err
	}

	return refs, nil
}
