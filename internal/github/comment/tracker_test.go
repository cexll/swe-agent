package comment

import (
	"context"
	"strings"
	"testing"
)

func TestFormatInitialBody(t *testing.T) {
	body := formatInitialBody()

	// 检查是否包含 spinner
	if !strings.Contains(body, "img src=") {
		t.Error("Initial body should contain spinner")
	}

	// 检查是否包含 Tasks 标题
	if !strings.Contains(body, "### Tasks") {
		t.Error("Initial body should contain Tasks section")
	}

	// 检查是否包含 checklist
	if !strings.Contains(body, "- [ ]") {
		t.Error("Initial body should contain unchecked items")
	}
}

func TestFormatChecklist(t *testing.T) {
	tests := []struct {
		name      string
		tasks     []string
		completed []bool
		want      string
	}{
		{
			name:      "empty",
			tasks:     []string{},
			completed: []bool{},
			want:      "",
		},
		{
			name:      "single incomplete",
			tasks:     []string{"Task 1"},
			completed: []bool{false},
			want:      "- [ ] Task 1",
		},
		{
			name:      "single complete",
			tasks:     []string{"Task 1"},
			completed: []bool{true},
			want:      "- [x] Task 1",
		},
		{
			name:      "multiple mixed",
			tasks:     []string{"Task 1", "Task 2", "Task 3"},
			completed: []bool{true, false, true},
			want:      "- [x] Task 1\n- [ ] Task 2\n- [x] Task 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatChecklist(tt.tasks, tt.completed)
			if got != tt.want {
				t.Errorf("FormatChecklist() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAddSpinner(t *testing.T) {
	text := "Working..."
	result := AddSpinner(text)

	if !strings.Contains(result, "img src=") {
		t.Error("AddSpinner should add spinner image")
	}
	if !strings.Contains(result, text) {
		t.Error("AddSpinner should preserve original text")
	}
}

func TestRemoveSpinner(t *testing.T) {
	text := AddSpinner("Working...")
	result := RemoveSpinner(text)

	if strings.Contains(result, "img src=") {
		t.Error("RemoveSpinner should remove spinner image")
	}
	if !strings.Contains(result, "Working...") {
		t.Error("RemoveSpinner should preserve text")
	}
}

// compile-time usage of context so linter doesn't complain about the import when running focus tests
var _ = context.TODO
