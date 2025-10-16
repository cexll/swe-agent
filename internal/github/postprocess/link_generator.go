package postprocess

import (
	"fmt"
	"net/url"
)

// LinkGenerator 生成 GitHub 链接
type LinkGenerator struct {
	ServerURL string
	Owner     string
	Repo      string
}

// NewLinkGenerator 创建链接生成器
func NewLinkGenerator(owner, repo string) *LinkGenerator {
	return &LinkGenerator{
		ServerURL: "https://github.com",
		Owner:     owner,
		Repo:      repo,
	}
}

// GenerateBranchLink 生成分支链接（Markdown 格式）
func (lg *LinkGenerator) GenerateBranchLink(branch string) string {
	url := fmt.Sprintf("%s/%s/%s/tree/%s", lg.ServerURL, lg.Owner, lg.Repo, branch)
	return fmt.Sprintf("\n[View branch](%s)", url)
}

// GeneratePRLink 生成 PR 创建链接（Markdown 格式）
func (lg *LinkGenerator) GeneratePRLink(baseBranch, headBranch string, issueNumber int, isPR bool) string {
	// 确定实体类型
	entityType := "Issue"
	if isPR {
		entityType = "PR"
	}

	// 构建 PR 标题和描述
	title := fmt.Sprintf("%s #%d: Changes from SWE Agent", entityType, issueNumber)
	body := fmt.Sprintf("This PR addresses %s #%d\n\nGenerated with [SWE Agent](https://github.com/cexll/swe-agent)",
		entityType, issueNumber)

	// URL 编码
	encodedTitle := url.QueryEscape(title)
	encodedBody := url.QueryEscape(body)

	// 构建 PR 创建 URL（使用 GitHub 的 quick_pull 功能）
	prURL := fmt.Sprintf("%s/%s/%s/compare/%s...%s?quick_pull=1&title=%s&body=%s",
		lg.ServerURL,
		lg.Owner,
		lg.Repo,
		baseBranch,
		headBranch,
		encodedTitle,
		encodedBody,
	)

	return fmt.Sprintf("\n[Create a PR](%s)", prURL)
}

// GenerateJobRunLink 生成 Job Run 链接（Markdown 格式）
func (lg *LinkGenerator) GenerateJobRunLink(runID string) string {
	url := fmt.Sprintf("%s/%s/%s/actions/runs/%s", lg.ServerURL, lg.Owner, lg.Repo, runID)
	return fmt.Sprintf("[Job Run](%s)", url)
}
