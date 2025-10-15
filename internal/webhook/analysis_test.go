package webhook

import "testing"

func TestIsAnalysisOnly(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"explicit-chinese", "/code 请只做分析，不要修改任何文件。", true},
		{"english-review", "/code please REVIEW this change only", true},
		{"question-no-action", "/code 这段代码是做什么的？可以解释一下实现吗", true},
		{"question-with-action", "/code 这段代码是做什么的？请修改 main.go 并解释原因", false},
		{"action-only", "/code 添加 Multiply 函数", false},
		{"empty", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isAnalysisOnly(tc.body)
			if got != tc.want {
				t.Fatalf("isAnalysisOnly(%q) = %v, want %v", tc.body, got, tc.want)
			}
		})
	}
}
