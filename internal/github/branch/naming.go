package branch

import (
	"fmt"
	"regexp"
	"strings"
)

// GenerateBranchName 生成智能分支名：swe/issue-123-fix-login-bug
// issueNumber: Issue 编号
// issueTitle: Issue 标题
// 返回格式：swe/issue-{number}-{slug}
func GenerateBranchName(issueNumber int, issueTitle string) string {
	// 1. 清理标题，生成 slug
	slug := slugify(issueTitle)

	// 2. 限制总长度不超过 50（为避免边界截断，实际保守到 48）
	prefix := fmt.Sprintf("swe/issue-%d-", issueNumber)
	maxSlugLen := 48 - len(prefix)
	if maxSlugLen < 0 {
		maxSlugLen = 0
	}
	if len(slug) > maxSlugLen {
		slug = slug[:maxSlugLen]
	}

	// 3. 移除末尾的连字符
	slug = strings.TrimRight(slug, "-")

	return prefix + slug
}

// slugify 将标题转换为 slug
// "Fix login bug!" -> "fix-login-bug"
func slugify(s string) string {
	// 1. 转小写
	s = strings.ToLower(s)

	// 2. 替换空格和下划线为连字符
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	// 3. 移除非字母数字和连字符的字符
	reg := regexp.MustCompile(`[^a-z0-9-]+`)
	s = reg.ReplaceAllString(s, "")

	// 4. 合并多个连续连字符为一个
	reg = regexp.MustCompile(`-+`)
	s = reg.ReplaceAllString(s, "-")

	// 5. 移除首尾连字符
	s = strings.Trim(s, "-")

	return s
}

// ValidateBranchName 验证分支名是否有效
func ValidateBranchName(name string) bool {
	// 1. 必须以 swe/ 开头
	if !strings.HasPrefix(name, "swe/") {
		return false
	}

	// 2. 长度限制
	if len(name) > 100 || len(name) < 10 {
		return false
	}

	// 3. 只能包含字母、数字、连字符和斜杠
	reg := regexp.MustCompile(`^swe/[a-z0-9-/]+$`)
	return reg.MatchString(name)
}
