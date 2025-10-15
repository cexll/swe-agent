package data

import (
	"strings"
	"testing"
)

func TestFormatContext_PR(t *testing.T) {
	pr := PullRequest{
		Title:       "Add feature X",
		Author:      Author{Login: "alice"},
		HeadRefName: "feature/x",
		BaseRefName: "main",
		State:       "OPEN",
		Additions:   12,
		Deletions:   3,
	}
	pr.Commits.TotalCount = 2
	pr.Files.Nodes = []File{{Path: "a.go"}, {Path: "b.go"}}

	out := formatContext(pr, true)
	if !strings.Contains(out, "PR Title: Add feature X") {
		t.Fatalf("missing title: %q", out)
	}
	if !strings.Contains(out, "PR Author: alice") {
		t.Fatalf("missing author: %q", out)
	}
	if !strings.Contains(out, "feature/x -> main") {
		t.Fatalf("missing branches: %q", out)
	}
	if !strings.Contains(out, "Changed Files: 2 files") {
		t.Fatalf("missing changed files: %q", out)
	}
}

func TestFormatContext_Issue(t *testing.T) {
	is := Issue{Title: "Bug Y", Author: Author{Login: "bob"}, State: "OPEN"}
	out := formatContext(is, false)
	if !strings.Contains(out, "Issue Title: Bug Y") || !strings.Contains(out, "Issue Author: bob") {
		t.Fatalf("unexpected: %q", out)
	}
}

func TestFormatBody_NoMap(t *testing.T) {
	raw := "Hello <!-- c -->world ![alt](http://x/img.png) <div title=\"a\"></div>"
	out := formatBody(raw, nil)
	if strings.Contains(out, "<!--") || strings.Contains(out, "alt]") || strings.Contains(out, "title=") {
		t.Fatalf("content not sanitized: %q", out)
	}
}

func TestFormatBody_WithImageMap(t *testing.T) {
	raw := "image ![](http://cdn/image.png)"
	mapped := formatBody(raw, map[string]string{"http://cdn/image.png": "/local/image.png"})
	if !strings.Contains(mapped, "/local/image.png") || strings.Contains(mapped, "http://cdn/image.png") {
		t.Fatalf("image url not replaced: %q", mapped)
	}
}

func TestFormatComments_Cases(t *testing.T) {
	// empty
	if s := formatComments(nil, nil); s != "" {
		t.Fatalf("expected empty, got %q", s)
	}
	// multiple + skip minimized + sanitize + replacements
	comments := []Comment{
		{Body: "hi <!--x--> ![](http://u/img.png)", Author: Author{Login: "u1"}, CreatedAt: "t1"},
		{Body: "skip", Author: Author{Login: "u2"}, CreatedAt: "t2", IsMinimized: true},
		{Body: "next", Author: Author{Login: "u3"}, CreatedAt: "t3"},
	}
	s := formatComments(comments, map[string]string{"http://u/img.png": "/l/i.png"})
	if strings.Contains(s, "<!--") {
		t.Fatalf("not sanitized: %q", s)
	}
	if !strings.Contains(s, "/l/i.png") || strings.Contains(s, "http://u/img.png") {
		t.Fatalf("image not replaced: %q", s)
	}
	if strings.Contains(s, "skip") {
		t.Fatalf("minimized not skipped: %q", s)
	}
	if !strings.Contains(s, "[u1 at t1]: hi") || !strings.Contains(s, "[u3 at t3]: next") {
		t.Fatalf("bad format: %q", s)
	}
}

type ReviewsWrap struct{ Nodes []Review }
type ReviewCommentsWrap struct{ Nodes []ReviewComment }

func TestFormatReviewComments_Variants(t *testing.T) {
	if s := formatReviewComments(nil, nil); s != "" {
		t.Fatalf("nil reviews should be empty")
	}
	empty := &ReviewsWrap{Nodes: nil}
	if s := formatReviewComments((*struct{ Nodes []Review })(empty), nil); s != "" {
		t.Fatalf("empty nodes should be empty")
	}

	lineNum := 42
	rv := Review{
		Author:      Author{Login: "rv1"},
		State:       "APPROVED",
		SubmittedAt: "t0",
		Body:        "Body <!--c--> ![](http://u/i.png)",
	}
	rv.Comments.Nodes = []ReviewComment{
		{Comment: Comment{Body: "inline1", IsMinimized: false}, Path: "a.go", Line: &lineNum},
		{Comment: Comment{Body: "hide", IsMinimized: true}, Path: "b.go"},
	}
	reviews := &ReviewsWrap{Nodes: []Review{rv}}

	s := formatReviewComments((*struct{ Nodes []Review })(reviews), map[string]string{"http://u/i.png": "/l/i.png"})
	if !strings.Contains(s, "[Review by rv1 at t0]: APPROVED") {
		t.Fatalf("missing header: %q", s)
	}
	if strings.Contains(s, "<!--") {
		t.Fatalf("not sanitized: %q", s)
	}
	if !strings.Contains(s, "/l/i.png") || strings.Contains(s, "http://u/i.png") {
		t.Fatalf("image not replaced: %q", s)
	}
	if strings.Contains(s, "hide") {
		t.Fatalf("minimized inline not skipped: %q", s)
	}
	if !strings.Contains(s, "  [Comment on a.go:42]: inline1") {
		t.Fatalf("missing inline: %q", s)
	}
}

func TestFormatChangedFiles(t *testing.T) {
	if s := formatChangedFiles(nil); s != "" {
		t.Fatalf("expected empty string")
	}
	files := []File{
		{Path: "a.go", ChangeType: "ADDED", Additions: 10, Deletions: 0},
		{Path: "b.go", ChangeType: "MODIFIED", Additions: 2, Deletions: 1},
	}
	s := formatChangedFiles(files)
	if !strings.Contains(s, "- a.go (ADDED) +10/-0") || !strings.Contains(s, "- b.go (MODIFIED) +2/-1") {
		t.Fatalf("bad formatting: %q", s)
	}
}

func TestFormatChangedFilesWithSHA_List(t *testing.T) {
	files := []GitHubFileWithSHA{{File: File{Path: "x", ChangeType: "M", Additions: 1, Deletions: 2}, SHA: "abc"}}
	s := formatChangedFilesWithSHA(files)
	if !strings.Contains(s, "SHA: abc") {
		t.Fatalf("missing sha: %q", s)
	}
}

func TestGenerateXML_IssueAndPR(t *testing.T) {
	// Issue
	is := Issue{Title: "B", Body: "Hello <!--x-->", Author: Author{Login: "bob"}, State: "OPEN"}
	xml := GenerateXML(GenerateXMLParams{
		Repository:         "o/r",
		IsPR:               false,
		Number:             7,
		EventType:          "issue_comment",
		TriggerContext:     "ctx",
		TriggerUsername:    "bob",
		TriggerDisplayName: "",
		TriggerPhrase:      "run",
		TriggerComment:     "Hi <!--c--> ",
		ContextData:        is,
		Comments:           []Comment{},
	})
	mustContain(t, xml, "<formatted_context>")
	mustContain(t, xml, "<pr_or_issue_body>")
	mustContain(t, xml, "<comments>")
	mustContain(t, xml, "<event_type>issue_comment</event_type>")
	mustContain(t, xml, "<is_pr>false</is_pr>")
	mustContain(t, xml, "<issue_number>7</issue_number>")
	// trigger comment sanitized
	if strings.Contains(xml, "<!--") {
		t.Fatalf("trigger comment not sanitized: %q", xml)
	}
	// display name fallback to username
	mustContain(t, xml, "<trigger_display_name>bob</trigger_display_name>")

	// PR
	pr := PullRequest{Title: "P", Body: "Body", Author: Author{Login: "alice"}, BaseRefName: "main", HeadRefName: "f", State: "OPEN"}
	pr.Files.Nodes = []File{{Path: "f.go", ChangeType: "MODIFIED", Additions: 1, Deletions: 1}}
	xml = GenerateXML(GenerateXMLParams{
		Repository:          "o/r",
		IsPR:                true,
		Number:              9,
		EventType:           "pull_request",
		TriggerContext:      "ctx",
		TriggerUsername:     "alice",
		TriggerDisplayName:  "Alice Doe",
		TriggerPhrase:       "run",
		ContextData:         pr,
		Comments:            []Comment{{Body: "ok", Author: Author{Login: "c"}, CreatedAt: "t"}},
		ReviewData:          &struct{ Nodes []Review }{Nodes: []Review{{Author: Author{Login: "r"}, State: "APPROVED", SubmittedAt: "t"}}},
		ChangedFilesWithSHA: []GitHubFileWithSHA{{File: File{Path: "f.go", ChangeType: "MODIFIED", Additions: 1, Deletions: 1}, SHA: "abc"}},
	})
	mustContain(t, xml, "<review_comments>")
	mustContain(t, xml, "<changed_files>")
	mustContain(t, xml, "<is_pr>true</is_pr>")
	mustContain(t, xml, "<pr_number>9</pr_number>")
}

func TestGenerateXML_EmptyTriggerComment(t *testing.T) {
	is := Issue{Title: "B", Author: Author{Login: "bob"}, State: "OPEN"}
	xml := GenerateXML(GenerateXMLParams{Repository: "o/r", IsPR: false, Number: 1, EventType: "issue_comment", TriggerContext: "ctx", TriggerUsername: "bob", TriggerDisplayName: "Bob", TriggerPhrase: "go", ContextData: is})
	if strings.Contains(xml, "<trigger_comment>") {
		t.Fatalf("should not include trigger_comment when empty: %q", xml)
	}
}

func mustContain(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Fatalf("missing %q in %q", sub, s)
	}
}
