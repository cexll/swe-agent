package webhook

import "strings"

// isAnalysisOnly returns true when the comment clearly requests analysis/review-only
// (no code changes). The rule is intentionally simple and explainable:
//  1. If it contains "只做分析" or case-insensitive "review" → true
//  2. Else, if it looks like a question (contains ？/?/解释/说明) and
//     does not contain obvious action verbs (修改/创建/添加/实现/修复) → true
//
// Otherwise → false.
func isAnalysisOnly(body string) bool {
	b := strings.TrimSpace(body)
	if b == "" {
		return false
	}
	bl := strings.ToLower(b)
	if strings.Contains(b, "只做分析") || strings.Contains(bl, "review") {
		return true
	}
	hasQuestion := strings.Contains(b, "？") || strings.Contains(b, "?") || strings.Contains(b, "解释") || strings.Contains(b, "说明")
	// 注意："实现"在中文里常作名词（implementation），容易误判；此处不将其视为动作词
	hasAction := strings.Contains(b, "修改") || strings.Contains(b, "创建") || strings.Contains(b, "添加") || strings.Contains(b, "修复")
	return hasQuestion && !hasAction
}
