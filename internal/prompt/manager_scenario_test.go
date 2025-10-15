package prompt

import "testing"

func TestRenderUnderstandRequestSection_ScenarioClassification(t *testing.T) {
	// Comment event path
	data := promptTemplateData{
		IsCommentEvent: true,
		TriggerPhrase:  "@assistant",
	}
	out := renderUnderstandRequestSection(data)
	if want := "Extract the actual question or request from the <trigger_comment> tag"; !contains(out, want) {
		t.Fatalf("missing comment extraction hint: %q", want)
	}
	// Must include classification buckets
	for _, k := range []string{"REVIEW/QUESTION MODE", "IMPLEMENTATION MODE", "HYBRID MODE", "DEFAULT RULE"} {
		if !contains(out, k) {
			t.Fatalf("missing classification %q", k)
		}
	}

	// Non-comment path uses trigger phrase text
	data2 := promptTemplateData{IsCommentEvent: false, TriggerPhrase: "@code"}
	out2 := renderUnderstandRequestSection(data2)
	if want2 := "contains '@code'"; !contains(out2, want2) {
		t.Fatalf("missing trigger phrase guidance: %q in %q", want2, out2)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	// naive search to avoid importing strings and keep test focused
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
