package postprocess

import (
	"strings"
	"testing"
)

func TestLinkGenerator(t *testing.T) {
	lg := NewLinkGenerator("o", "r")
	b := lg.GenerateBranchLink("feat-x")
	if b == "" || b[0] != '\n' || !strings.Contains(b, "/o/r/tree/feat-x") {
		t.Fatalf("unexpected branch link: %q", b)
	}
	p := lg.GeneratePRLink("main", "feat-x", 42, true)
	if p == "" || !strings.Contains(p, "/o/r/compare/main...feat-x") || !strings.Contains(p, "quick_pull=1") {
		t.Fatalf("unexpected PR link: %q", p)
	}
	// When not PR context, link title/body uses Issue
	p2 := lg.GeneratePRLink("main", "feat-x", 7, false)
	if !(strings.Contains(p2, "Issue%20%237") || strings.Contains(p2, "Issue+%237")) {
		t.Fatalf("expected Issue title in URL: %q", p2)
	}
	j := lg.GenerateJobRunLink("123")
	if j == "" || !strings.Contains(j, "/o/r/actions/runs/123") {
		t.Fatalf("unexpected job run link: %q", j)
	}
}
