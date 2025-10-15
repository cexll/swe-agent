package github

import (
	"encoding/json"
	"testing"
)

// helper to build common repo/sender blocks
func basePayload() map[string]interface{} {
	return map[string]interface{}{
		"repository": map[string]interface{}{
			"name":      "swe-agent",
			"full_name": "cexll/swe-agent",
			"owner": map[string]interface{}{
				"login": "cexll",
			},
		},
		"sender": map[string]interface{}{
			"login": "octocat",
		},
	}
}

func mustJSON(t *testing.T, m map[string]interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestParseWebhookEvent_IssueComment(t *testing.T) {
	// complete payload, issue comment on PR (issue has pull_request field)
	p := basePayload()
	p["action"] = "created"
	p["issue"] = map[string]interface{}{
		"number":       float64(11),
		"pull_request": map[string]interface{}{},
	}
	p["comment"] = map[string]interface{}{
		"id":         float64(101),
		"body":       "/code do it",
		"user":       map[string]interface{}{"login": "octocat"},
		"created_at": "2024-01-01T00:00:00Z",
		"updated_at": "2024-01-01T00:00:10Z",
	}

	ctx, err := ParseWebhookEvent("issue_comment", mustJSON(t, p))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctx.EventName != EventIssueComment || ctx.EventAction != ActionCreated {
		t.Fatalf("event parsed wrong: %v %v", ctx.EventName, ctx.EventAction)
	}
	if !ctx.IsPR || ctx.PRNumber != 11 || ctx.IssueNumber != 11 {
		t.Fatalf("PR detection failed: isPR=%v pr=%d issue=%d", ctx.IsPR, ctx.PRNumber, ctx.IssueNumber)
	}
	if ctx.TriggerComment == nil || ctx.TriggerComment.ID != 101 || ctx.TriggerComment.Body != "/code do it" {
		t.Fatalf("comment parsed wrong: %+v", ctx.TriggerComment)
	}
	if ctx.Repository.FullName != "cexll/swe-agent" || ctx.Actor != "octocat" || ctx.TriggerUser != "octocat" {
		t.Fatalf("repo/sender wrong: repo=%+v actor=%s trig=%s", ctx.Repository, ctx.Actor, ctx.TriggerUser)
	}
}

func TestParseWebhookEvent_Issues(t *testing.T) {
	p := basePayload()
	p["action"] = "opened"
	p["issue"] = map[string]interface{}{
		"number": float64(7),
	}

	ctx, err := ParseWebhookEvent("issues", mustJSON(t, p))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctx.EventName != EventIssues || ctx.EventAction != ActionOpened {
		t.Fatalf("wrong event: %v %v", ctx.EventName, ctx.EventAction)
	}
	if ctx.IsPR {
		t.Fatalf("issues should not be PR context")
	}
	if ctx.IssueNumber != 7 {
		t.Fatalf("issue number = %d, want 7", ctx.IssueNumber)
	}

	// closed action
	p["action"] = "closed"
	ctx2, err := ParseWebhookEvent("issues", mustJSON(t, p))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx2.EventAction != ActionClosed {
		t.Fatalf("action = %v, want closed", ctx2.EventAction)
	}
}

func TestParseWebhookEvent_PullRequest(t *testing.T) {
	// complete PR payload
	p := basePayload()
	p["action"] = "opened"
	p["pull_request"] = map[string]interface{}{
		"number": float64(3),
		"base":   map[string]interface{}{"ref": "main"},
		"head":   map[string]interface{}{"ref": "feature"},
	}

	ctx, err := ParseWebhookEvent("pull_request", mustJSON(t, p))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.IsPR || ctx.PRNumber != 3 || ctx.BaseBranch != "main" || ctx.HeadBranch != "feature" {
		t.Fatalf("PR fields wrong: %+v", ctx)
	}

	// minimal PR payload
	p2 := basePayload()
	p2["action"] = "edited"
	p2["pull_request"] = map[string]interface{}{
		"number": float64(5),
	}
	ctx2, err := ParseWebhookEvent("pull_request", mustJSON(t, p2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx2.PRNumber != 5 || ctx2.BaseBranch != "" || ctx2.HeadBranch != "" {
		t.Fatalf("minimal PR parse wrong: %+v", ctx2)
	}
}

func TestParseWebhookEvent_PullRequestReview(t *testing.T) {
	p := basePayload()
	p["action"] = "submitted"
	p["pull_request"] = map[string]interface{}{
		"number": float64(9),
		"base":   map[string]interface{}{"ref": "develop"},
		"head":   map[string]interface{}{"ref": "topic"},
	}
	p["review"] = map[string]interface{}{
		"id":           float64(2001),
		"body":         "/code please fix",
		"user":         map[string]interface{}{"login": "alice"},
		"submitted_at": "2024-01-02T12:00:00Z",
	}

	ctx, err := ParseWebhookEvent("pull_request_review", mustJSON(t, p))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.IsPR || ctx.PRNumber != 9 {
		t.Fatalf("PR number missing: %+v", ctx)
	}
	if ctx.TriggerComment == nil || ctx.TriggerComment.ID != 2001 || ctx.TriggerComment.User != "alice" {
		t.Fatalf("review parsed wrong: %+v", ctx.TriggerComment)
	}
}

func TestParseWebhookEvent_PullRequestReviewComment(t *testing.T) {
	p := basePayload()
	p["action"] = "created"
	p["pull_request"] = map[string]interface{}{"number": float64(10)}
	p["comment"] = map[string]interface{}{
		"id":         float64(3001),
		"body":       "LGTM /code run",
		"user":       map[string]interface{}{"login": "bob"},
		"created_at": "2024-01-03T00:00:00Z",
		"updated_at": "2024-01-03T00:00:01Z",
	}

	ctx, err := ParseWebhookEvent("pull_request_review_comment", mustJSON(t, p))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.TriggerComment == nil || ctx.TriggerComment.Body == "" || ctx.TriggerComment.User != "bob" {
		t.Fatalf("review comment parsed wrong: %+v", ctx.TriggerComment)
	}
}

func TestParseWebhookEvent_UnsupportedAndInvalid(t *testing.T) {
	// unsupported event
	if _, err := ParseWebhookEvent("project_card", []byte("{}")); err == nil {
		t.Fatal("expected error for unsupported event type")
	}
	// invalid json
	if _, err := ParseWebhookEvent("issues", []byte("{")); err == nil {
		t.Fatal("expected JSON parse error")
	}
}

func TestParseIssueComment_Variants(t *testing.T) {
	// complete
	p := basePayload()
	p["action"] = "edited"
	p["issue"] = map[string]interface{}{"number": float64(1)}
	p["comment"] = map[string]interface{}{
		"id":         float64(42),
		"body":       "hello",
		"user":       map[string]interface{}{"login": "eve"},
		"created_at": "t1",
		"updated_at": "t2",
	}
	ctx, err := ParseWebhookEvent("issue_comment", mustJSON(t, p))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if ctx.TriggerComment == nil || ctx.TriggerComment.ID != 42 || ctx.IssueNumber != 1 {
		t.Fatalf("parseIssueComment complete failed: %+v", ctx)
	}

	// minimal: no comment block
	p2 := basePayload()
	p2["action"] = "created"
	p2["issue"] = map[string]interface{}{"number": float64(2)}
	ctx2, err := ParseWebhookEvent("issue_comment", mustJSON(t, p2))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if ctx2.TriggerComment != nil || ctx2.IssueNumber != 2 {
		t.Fatalf("parseIssueComment minimal failed: %+v", ctx2)
	}
}

func TestParsePullRequest_MinimalAndComplete(t *testing.T) {
	// covered in TestParseWebhookEvent_PullRequest; kept for explicit coverage
	p := basePayload()
	p["action"] = "opened"
	p["pull_request"] = map[string]interface{}{"number": float64(123)}
	ctx, err := ParseWebhookEvent("pull_request", mustJSON(t, p))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !ctx.IsPR || ctx.PRNumber != 123 {
		t.Fatalf("pr parse failed: %+v", ctx)
	}
}

func TestShouldTrigger(t *testing.T) {
	p := basePayload()
	p["action"] = "created"
	p["issue"] = map[string]interface{}{"number": float64(1)}
	p["comment"] = map[string]interface{}{
		"id":   float64(1),
		"body": "/code run analysis",
		"user": map[string]interface{}{"login": "dev"},
	}
	ctx, _ := ParseWebhookEvent("issue_comment", mustJSON(t, p))

	if !ctx.ShouldTrigger("/code") {
		t.Fatalf("ShouldTrigger expected true")
	}
	if ctx.ShouldTrigger("/run") {
		t.Fatalf("ShouldTrigger expected false for other phrase")
	}
	// different phrase works as exact match when present
	p2 := p
	p2["comment"].(map[string]interface{})["body"] = "please /run now"
	ctx2, _ := ParseWebhookEvent("issue_comment", mustJSON(t, p2))
	if !ctx2.ShouldTrigger("/run") {
		t.Fatalf("ShouldTrigger with different phrase failed")
	}
}

func TestExtractPrompt(t *testing.T) {
	p := basePayload()
	p["action"] = "created"
	p["issue"] = map[string]interface{}{"number": float64(2)}
	p["comment"] = map[string]interface{}{
		"id":   float64(2),
		"body": "/code   fix bug please",
		"user": map[string]interface{}{"login": "dev"},
	}
	ctx, _ := ParseWebhookEvent("issue_comment", mustJSON(t, p))
	if got := ctx.ExtractPrompt("/code"); got != "fix bug please" {
		t.Fatalf("prompt = %q, want %q", got, "fix bug please")
	}

	// no trigger
	p2 := basePayload()
	p2["action"] = "created"
	p2["issue"] = map[string]interface{}{"number": float64(3)}
	p2["comment"] = map[string]interface{}{
		"id":   float64(3),
		"body": "no trigger here",
		"user": map[string]interface{}{"login": "dev"},
	}
	ctx2, _ := ParseWebhookEvent("issue_comment", mustJSON(t, p2))
	if got := ctx2.ExtractPrompt("/code"); got != "" {
		t.Fatalf("prompt = %q, want empty", got)
	}
}

func TestShouldTrigger_NoCommentAndExtract_NoComment(t *testing.T) {
	// pull_request events have no TriggerComment
	p := basePayload()
	p["action"] = "opened"
	p["pull_request"] = map[string]interface{}{"number": float64(1)}
	ctx, _ := ParseWebhookEvent("pull_request", mustJSON(t, p))

	if ctx.ShouldTrigger("/code") {
		t.Fatalf("expected ShouldTrigger false when no comment")
	}
	if got := ctx.ExtractPrompt("/code"); got != "" {
		t.Fatalf("expected empty prompt when no comment, got %q", got)
	}
}

func TestGetterMethods(t *testing.T) {
	p := basePayload()
	p["action"] = "created"
	p["pull_request"] = map[string]interface{}{
		"number": float64(99),
		"base":   map[string]interface{}{"ref": "stable"},
		"head":   map[string]interface{}{"ref": "topic/x"},
	}
	ctx, _ := ParseWebhookEvent("pull_request", mustJSON(t, p))

	if ctx.GetEventName() != string(EventPullRequest) || ctx.GetEventAction() != string(ActionCreated) {
		t.Fatalf("getters event/action wrong: %s %s", ctx.GetEventName(), ctx.GetEventAction())
	}
	if ctx.GetRepositoryFullName() != "cexll/swe-agent" || ctx.GetRepositoryOwner() != "cexll" || ctx.GetRepositoryName() != "swe-agent" {
		t.Fatalf("repo getters wrong: %s %s %s", ctx.GetRepositoryFullName(), ctx.GetRepositoryOwner(), ctx.GetRepositoryName())
	}
	if !ctx.IsPRContext() || ctx.GetIssueNumber() != 99 || ctx.GetPRNumber() != 99 {
		t.Fatalf("number getters wrong: isPR=%v issue=%d pr=%d", ctx.IsPRContext(), ctx.GetIssueNumber(), ctx.GetPRNumber())
	}
	if ctx.GetBaseBranch() != "stable" || ctx.GetHeadBranch() != "topic/x" {
		t.Fatalf("branch getters wrong: %s %s", ctx.GetBaseBranch(), ctx.GetHeadBranch())
	}
	if ctx.GetTriggerUser() != "octocat" || ctx.GetActor() != "octocat" {
		t.Fatalf("actor getters wrong: %s %s", ctx.GetTriggerUser(), ctx.GetActor())
	}
	if ctx.GetTriggerCommentBody() != "" { // no comment for pull_request event
		t.Fatalf("trigger comment body should be empty: %q", ctx.GetTriggerCommentBody())
	}
}

func TestRepositoryParsing_MissingNestedOwner(t *testing.T) {
	// Exercise getStringField nested traversal failure path
	p := basePayload()
	// override owner to a non-map so nested access fails
	p["repository"].(map[string]interface{})["owner"] = "not-a-map"
	p["action"] = "opened"
	p["issues"] = map[string]interface{}{}

	ctx, err := ParseWebhookEvent("issues", mustJSON(t, p))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if ctx.GetRepositoryOwner() != "" {
		t.Fatalf("owner should be empty when nested map missing, got %q", ctx.GetRepositoryOwner())
	}
}

func TestHelperAccessors(t *testing.T) {
	m := map[string]interface{}{
		"a": map[string]interface{}{"b": "x"},
		"n": float64(2),
		"s": "not-a-map",
	}
	if got := getStringField(m, "a", "b"); got != "x" {
		t.Fatalf("getStringField nested = %q, want x", got)
	}
	if got := getStringField(m, "a", "missing"); got != "" {
		t.Fatalf("getStringField missing final = %q, want empty", got)
	}
	if got := getStringField(m, "s", "k"); got != "" {
		t.Fatalf("getStringField non-map intermediate = %q, want empty", got)
	}
	if n := getNumberField(m, "n"); n != 2 {
		t.Fatalf("getNumberField = %v, want 2", n)
	}
	if n := getNumberField(m, "a", "b"); n != 0 { // final type mismatch (string)
		t.Fatalf("getNumberField type mismatch = %v, want 0", n)
	}
	if n := getNumberField(m, "s", "k"); n != 0 { // intermediate not a map
		t.Fatalf("getNumberField non-map intermediate = %v, want 0", n)
	}
}
