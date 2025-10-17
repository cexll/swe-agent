package github

import (
	"strings"
	"testing"
)

func TestStripHtmlComments(t *testing.T) {
	in := "a<!-- x -->b<!--y-->c"
	if got := StripHtmlComments(in); got != "abc" {
		t.Fatalf("got %q, want %q", got, "abc")
	}
}

func TestStripInvisibleCharacters(t *testing.T) {
	in := "a\u200Bb\u0007c\u00ADd\u202Ae"
	if got := StripInvisibleCharacters(in); got != "abcde" {
		t.Fatalf("got %q", got)
	}
}

func TestStripMarkdownImageAltText(t *testing.T) {
	in := "![alt text](url) and ![x](y)"
	want := "![](url) and ![](y)"
	if got := StripMarkdownImageAltText(in); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestStripMarkdownLinkTitles(t *testing.T) {
	in := `[t](u "title") and [x](y 't')`
	want := "[t](u) and [x](y)"
	if got := StripMarkdownLinkTitles(in); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestStripHiddenAttributes(t *testing.T) {
	in := `<img alt="a" title='b' aria-label=c data-x="1" data-y='2' data-z=3 placeholder="t" src=x>`
	got := StripHiddenAttributes(in)
	if got != `<img src=x>` { // attributes stripped
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeHtmlEntities(t *testing.T) {
	in := "Hello &#65; &#x42; &#9999;!"
	want := "Hello A B !"
	if got := NormalizeHtmlEntities(in); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRedactGitHubTokens(t *testing.T) {
	// construct various token patterns
	in := "ghp_abcdefghijklmnopqrstuvwxyzABCD123456 gho_abcdefghijklmnopqrstuvwxyzABCD123456 ghs_abcdefghijklmnopqrstuvwxyzABCD123456 ghr_abcdefghijklmnopqrstuvwxyzABCD123456 github_pat_ABCdef12345_ABCdef12345"
	got := RedactGitHubTokens(in)
	if c := strings.Count(got, "[REDACTED_GITHUB_TOKEN]"); c != 5 {
		t.Fatalf("expected 5 redactions, got %d in %q", c, got)
	}
}

func TestSanitizeContent(t *testing.T) {
	in := ` <!--c-->![alt](img) [t](u "x") <img alt=a title=b aria-label=c data-x=1 placeholder=p src=s> &#65; ghp_abcdefghijklmnopqrstuvwxyzABCD1234 `
	got := SanitizeContent(in)
	if got == "" || got[0] == ' ' || got[len(got)-1] == ' ' {
		t.Fatalf("should trim spaces, got %q", got)
	}
	if got == in {
		t.Fatalf("content not sanitized: %q", got)
	}
}
