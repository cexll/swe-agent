package webhook

import (
	"strings"
	"testing"
	"time"
)

func TestSummarizeInstruction(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty returns empty",
			in:   "   \n ",
			want: "",
		},
		{
			name: "multiline collapses whitespace",
			in:   "line one\n\n  line two \n\tline three",
			want: "line one line two line three",
		},
		{
			name: "truncates long text",
			in:   strings.Repeat("a", 300),
			want: strings.Repeat("a", 180) + "…",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := summarizeInstruction(tc.in, 180)
			if got != tc.want {
				t.Fatalf("summarizeInstruction(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	t.Run("custom limit shorter than text", func(t *testing.T) {
		got := summarizeInstruction("keep this short please", 4)
		if got != "keep…" {
			t.Fatalf("limited summarizeInstruction = %q, want keep…", got)
		}
	})
}

func TestCommentDeduperLifecycle(t *testing.T) {
	d := newCommentDeduper(10 * time.Millisecond)

	if !d.markIfNew(1) {
		t.Fatal("first markIfNew should return true")
	}
	if d.markIfNew(1) {
		t.Fatal("second markIfNew should return false before expiry")
	}

	time.Sleep(15 * time.Millisecond)

	if !d.markIfNew(1) {
		t.Fatal("markIfNew should return true after expiry")
	}
}

func TestCommentDeduperDefaultTTL(t *testing.T) {
	d := newCommentDeduper(0)
	if d.ttl != time.Hour {
		t.Fatalf("default TTL = %s, want 1h", d.ttl)
	}
}
