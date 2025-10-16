package comment

import "testing"

func TestFormatLinks(t *testing.T) {
	got := FormatLinks("https://job", "https://branch")
	if got == "" || got[0] != '\n' || got[1] != '-' {
		t.Fatalf("unexpected prefix: %q", got)
	}
	if !containsAll(got, []string{"[Job Run](https://job)", "[Branch](https://branch)"}) {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestChecklistAndSpinner(t *testing.T) {
	chk := FormatChecklist([]string{"A", "B"}, []bool{true, false})
	if !containsAll(chk, []string{"- [x] A", "- [ ] B"}) {
		t.Fatalf("checklist: %q", chk)
	}
	with := AddSpinner("Working")
	if with == "Working" || !containsAll(with, []string{"img", "Working"}) {
		t.Fatalf("AddSpinner: %q", with)
	}
	stripped := RemoveSpinner(with)
	if stripped != "Working" {
		t.Fatalf("RemoveSpinner => %q", stripped)
	}
}

func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(sub) > 0 && (indexOf(s, sub) >= 0)))
}

func indexOf(s, sub string) int {
	// simple substring search to avoid importing strings
	n, m := len(s), len(sub)
	for i := 0; i+m <= n; i++ {
		if s[i:i+m] == sub {
			return i
		}
	}
	return -1
}
