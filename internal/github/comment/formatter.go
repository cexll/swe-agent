package comment

import (
	"fmt"
	"strings"
)

// FormatChecklist 格式化 checklist
// 输入：["Task 1", "Task 2"], completed: [true, false]
// 输出："- [x] Task 1\n- [ ] Task 2"
func FormatChecklist(tasks []string, completed []bool) string {
	if len(tasks) == 0 {
		return ""
	}
	var b strings.Builder
	for i, t := range tasks {
		if i > 0 {
			b.WriteByte('\n')
		}
		done := false
		if i < len(completed) {
			done = completed[i]
		}
		if done {
			b.WriteString("- [x] ")
		} else {
			b.WriteString("- [ ] ")
		}
		b.WriteString(t)
	}
	return b.String()
}

// AddSpinner 添加 spinner 到文本
func AddSpinner(text string) string {
	return `<img src="https://github.com/user-attachments/assets/5ac382c7-e004-429b-8e35-7feb3e8f9c6f" width="14px" /> ` + text
}

// RemoveSpinner 移除 spinner
func RemoveSpinner(text string) string {
	// 移除我们添加的标准 spinner 片段（包含或不包含后置空格）
	spinner := `<img src="https://github.com/user-attachments/assets/5ac382c7-e004-429b-8e35-7feb3e8f9c6f" width="14px" /> `
	text = strings.ReplaceAll(text, spinner, "")
	spinnerNoSpace := strings.TrimRight(spinner, " ")
	text = strings.ReplaceAll(text, spinnerNoSpace, "")
	return text
}

// FormatLinks 格式化底部链接
func FormatLinks(jobRunURL, branchURL string) string {
	return fmt.Sprintf("\n---\n[Job Run](%s) | [Branch](%s)", jobRunURL, branchURL)
}
