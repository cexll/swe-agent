package branch

import (
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Fix login bug", "fix-login-bug"},
		{"Add new feature!", "add-new-feature"},
		{"Update README.md", "update-readmemd"},
		{"Fix  multiple   spaces", "fix-multiple-spaces"},
		{"Test_underscore_naming", "test-underscore-naming"},
		{"Special chars: @#$%", "special-chars"},
		{"Trailing-", "trailing"},
		{"-Leading", "leading"},
		{"中文标题", ""}, // 非ASCII移除
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateBranchName(t *testing.T) {
	tests := []struct {
		issueNumber int
		issueTitle  string
		want        string
	}{
		{123, "Fix login bug", "swe/issue-123-fix-login-bug"},
		{1, "Add feature", "swe/issue-1-add-feature"},
		{999, "Update docs", "swe/issue-999-update-docs"},
		{456, "Very long title that exceeds the maximum length limit and should be truncated to fit", "swe/issue-456-very-long-title-that-exceeds-the-m"},
	}

	for _, tt := range tests {
		t.Run(tt.issueTitle, func(t *testing.T) {
			got := GenerateBranchName(tt.issueNumber, tt.issueTitle)
			if got != tt.want {
				t.Errorf("GenerateBranchName(%d, %q) = %q, want %q", tt.issueNumber, tt.issueTitle, got, tt.want)
			}
		})
	}
}

func TestValidateBranchName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"swe/issue-123-fix-bug", true},
		{"swe/issue-1-test", true},
		{"claude/issue-123-fix", false}, // 错误前缀
		{"swe/", false},                 // 太短
		{"swe/UPPERCASE", false},        // 大写字母
		{"swe/has space", false},        // 空格
		{"swe/has@special", false},      // 特殊字符
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateBranchName(tt.name)
			if got != tt.valid {
				t.Errorf("ValidateBranchName(%q) = %v, want %v", tt.name, got, tt.valid)
			}
		})
	}
}
