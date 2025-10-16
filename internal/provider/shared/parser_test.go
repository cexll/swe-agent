package shared

import (
	"testing"
)

func TestParseResponse_XMLBlocksAndSummary(t *testing.T) {
	input := `
<files>
  <file path="src/app.go"><content>package main\nfunc main() {}</content></file>
  <file path="path/to/file.ext"><content>... full file content here ...</content></file>
  <file path="lib/util.go"><content>package lib</content></file>
</files>
<summary>
Implemented feature X
</summary>`
	pr, err := ParseResponse("Claude", input)
	if err != nil {
		t.Fatalf("ParseResponse error: %v", err)
	}
	if len(pr.Files) != 2 {
		t.Fatalf("want 2 files after filtering, got %d", len(pr.Files))
	}
	if pr.Summary != "Implemented feature X" {
		t.Fatalf("unexpected summary: %q", pr.Summary)
	}
}

func TestParseResponse_MarkdownBlocks(t *testing.T) {
	input := "" +
		"```go src/handler.go\npackage handler\n```\n" +
		"**internal/util.go**\n```go\npackage util\n```\n" +
		"# Summary\nAdd endpoints\n"
	pr, err := ParseResponse("Codex", input)
	if err != nil {
		t.Fatalf("ParseResponse error: %v", err)
	}
	if len(pr.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(pr.Files))
	}
	if pr.Summary != "Add endpoints" {
		t.Fatalf("unexpected summary: %q", pr.Summary)
	}
}

func TestParseResponse_NoFilesRequiresNonPlaceholderSummary(t *testing.T) {
	// Placeholder summary without files should error
	if _, err := ParseResponse("Claude", "<summary>brief description of changes made</summary>"); err == nil {
		t.Fatalf("expected error for placeholder summary with no files")
	}

	// Non-empty non-placeholder OK
	pr, err := ParseResponse("Claude", "<summary>Real summary</summary>")
	if err != nil || pr.Summary == "" {
		t.Fatalf("unexpected err=%v pr=%+v", err, pr)
	}
}

func TestContainsPermissionRequest(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"Would you like me to proceed?", true},
		{"I can proceed once you approve", true},
		{"No permission request here", false},
	}
	for _, c := range cases {
		if got := ContainsPermissionRequest(c.in); got != c.want {
			t.Fatalf("ContainsPermissionRequest(%q)=%v want %v", c.in, got, c.want)
		}
	}
}
