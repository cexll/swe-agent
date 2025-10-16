package comment

import (
	"context"

	"github.com/google/go-github/v66/github"
)

// createInitialComment 创建初始评论（内部函数）
// 返回评论 ID
func createInitialComment(ctx context.Context, client *github.Client, owner, repo string, number int) (int64, error) {
	// 1. 生成初始 body（带 spinner + checklist）
	body := formatInitialBody()

	// 2. 调用 GitHub API 创建评论
	comment, _, err := client.Issues.CreateComment(ctx, owner, repo, number, &github.IssueComment{
		Body: &body,
	})
	if err != nil {
		return 0, err
	}
	if comment == nil || comment.ID == nil {
		return 0, github.CheckResponse(nil)
	}
	return *comment.ID, nil
}

// formatInitialBody 格式化初始评论内容
func formatInitialBody() string {
	return `<img src="https://github.com/user-attachments/assets/5ac382c7-e004-429b-8e35-7feb3e8f9c6f" width="14px" /> Working on your request...

### Tasks
- [ ] Analyzing request
- [ ] Making changes
- [ ] Testing

---
[Job Run](#) | [Branch](#)`
}
