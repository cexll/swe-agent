package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	ghdata "github.com/cexll/swe/internal/github/data"
)

type mockGitHubContext struct {
	eventName          string
	eventAction        string
	repositoryFullName string
	repositoryOwner    string
	repositoryName     string
	isPR               bool
	issueNumber        int
	prNumber           int
	baseBranch         string
	headBranch         string
	triggerUser        string
	actor              string
	triggerCommentBody string
}

func (m *mockGitHubContext) GetEventName() string          { return m.eventName }
func (m *mockGitHubContext) GetEventAction() string        { return m.eventAction }
func (m *mockGitHubContext) GetRepositoryFullName() string { return m.repositoryFullName }
func (m *mockGitHubContext) GetRepositoryOwner() string    { return m.repositoryOwner }
func (m *mockGitHubContext) GetRepositoryName() string     { return m.repositoryName }
func (m *mockGitHubContext) IsPRContext() bool             { return m.isPR }
func (m *mockGitHubContext) GetIssueNumber() int           { return m.issueNumber }
func (m *mockGitHubContext) GetPRNumber() int              { return m.prNumber }
func (m *mockGitHubContext) GetBaseBranch() string         { return m.baseBranch }
func (m *mockGitHubContext) GetHeadBranch() string         { return m.headBranch }
func (m *mockGitHubContext) GetTriggerUser() string        { return m.triggerUser }
func (m *mockGitHubContext) GetActor() string              { return m.actor }
func (m *mockGitHubContext) GetTriggerCommentBody() string { return m.triggerCommentBody }

func TestLoadSystemPrompt_FromCWD(t *testing.T) {
	tmpDir := t.TempDir()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}

	expectedContent := "Test system prompt from CWD"
	if err := os.WriteFile("system-prompt.md", []byte(expectedContent), 0644); err != nil {
		t.Fatalf("Failed to write system prompt: %v", err)
	}

	content, err := LoadSystemPrompt()
	if err != nil {
		t.Errorf("LoadSystemPrompt() returned unexpected error: %v", err)
	}
	if content != expectedContent {
		t.Errorf("LoadSystemPrompt() = %q, want %q", content, expectedContent)
	}
}

func TestLoadSystemPrompt_FromParentDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	expectedContent := "System prompt from parent directory"
	if err := os.WriteFile(filepath.Join(tmpDir, "system-prompt.md"), []byte(expectedContent), 0644); err != nil {
		t.Fatalf("Failed to write system prompt: %v", err)
	}

	subDir := filepath.Join(tmpDir, "subdir1", "subdir2")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	if err := os.Chdir(subDir); err != nil {
		t.Fatalf("Failed to change to subdir: %v", err)
	}

	content, err := LoadSystemPrompt()
	if err != nil {
		t.Errorf("LoadSystemPrompt() returned unexpected error: %v", err)
	}
	if content != expectedContent {
		t.Errorf("LoadSystemPrompt() = %q, want %q", content, expectedContent)
	}
}

func TestLoadSystemPrompt_FallbackWhenNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}

	content, err := LoadSystemPrompt()
	if err == nil {
		t.Error("LoadSystemPrompt() should return error when file not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error message should mention 'not found', got: %v", err)
	}
	if !strings.Contains(content, "AI assistant") {
		t.Errorf("Should return fallback prompt, got: %q", content)
	}
}

func TestBuildPrompt_IssueComment(t *testing.T) {
	ctx := &mockGitHubContext{
		eventName:          "issue_comment",
		eventAction:        "created",
		repositoryFullName: "owner/repo",
		repositoryOwner:    "owner",
		repositoryName:     "repo",
		isPR:               false,
		issueNumber:        42,
		baseBranch:         "main",
		triggerUser:        "testuser",
		triggerCommentBody: "/code fix this bug",
	}

	fetched := &ghdata.FetchResult{
		ContextData: ghdata.Issue{
			Title:  "Test Issue",
			Body:   "Issue description",
			Author: ghdata.Author{Login: "testuser"},
			State:  "open",
		},
		Comments: []ghdata.Comment{
			{Body: "/code fix this bug", Author: ghdata.Author{Login: "testuser"}},
		},
	}

	prompt := BuildPrompt(ctx, fetched)

	if !strings.Contains(prompt, "---") {
		t.Error("Prompt should contain separator '---'")
	}

	if !strings.Contains(prompt, "<repository>owner/repo</repository>") {
		t.Error("Prompt should contain repository tag")
	}

	if !strings.Contains(prompt, "<issue_number>42</issue_number>") {
		t.Error("Prompt should contain issue_number tag")
	}

	if !strings.Contains(prompt, "<event_type>GENERAL_COMMENT</event_type>") {
		t.Error("Prompt should contain event_type tag")
	}

	if !strings.Contains(prompt, "<trigger_username>testuser</trigger_username>") {
		t.Error("Prompt should contain trigger_username tag")
	}
}

func TestBuildPrompt_PullRequest(t *testing.T) {
	ctx := &mockGitHubContext{
		eventName:          "pull_request",
		eventAction:        "opened",
		repositoryFullName: "owner/repo",
		repositoryOwner:    "owner",
		repositoryName:     "repo",
		isPR:               true,
		prNumber:           123,
		baseBranch:         "main",
		headBranch:         "feature",
		triggerUser:        "contributor",
		triggerCommentBody: "",
	}

	fetched := &ghdata.FetchResult{
		ContextData: ghdata.PullRequest{
			Title:       "Test PR",
			Body:        "PR description",
			Author:      ghdata.Author{Login: "contributor"},
			BaseRefName: "main",
			HeadRefName: "feature",
			State:       "open",
		},
	}

	prompt := BuildPrompt(ctx, fetched)

	if !strings.Contains(prompt, "<pr_number>123</pr_number>") {
		t.Error("Prompt should contain pr_number tag")
	}

	if !strings.Contains(prompt, "<event_type>PULL_REQUEST</event_type>") {
		t.Error("Prompt should contain PR event_type")
	}

	if !strings.Contains(prompt, "main") {
		t.Error("Prompt should contain base branch in PR context")
	}
}

func TestBuildPrompt_WithNilFetchResult(t *testing.T) {
	ctx := &mockGitHubContext{
		eventName:          "issue_comment",
		repositoryFullName: "owner/repo",
		issueNumber:        1,
		triggerUser:        "user",
	}

	fetched := &ghdata.FetchResult{
		ContextData: ghdata.Issue{
			Title:  "Test",
			Author: ghdata.Author{Login: "user"},
			State:  "open",
		},
	}

	prompt := BuildPrompt(ctx, fetched)

	if prompt == "" {
		t.Error("BuildPrompt() should not return empty string")
	}

	if !strings.Contains(prompt, "---") {
		t.Error("Prompt should still contain separator")
	}
}

func TestEventTypeAndTriggerContext_ReviewComment(t *testing.T) {
	ctx := &mockGitHubContext{
		eventName: "pull_request_review_comment",
	}

	eventType, triggerCtx := eventTypeAndTriggerContext(ctx, "@assistant")

	if eventType != "REVIEW_COMMENT" {
		t.Errorf("eventType = %q, want REVIEW_COMMENT", eventType)
	}
	if !strings.Contains(triggerCtx, "PR review comment") {
		t.Errorf("triggerCtx should mention PR review comment, got: %q", triggerCtx)
	}
}

func TestEventTypeAndTriggerContext_PRReview(t *testing.T) {
	ctx := &mockGitHubContext{
		eventName: "pull_request_review",
	}

	eventType, triggerCtx := eventTypeAndTriggerContext(ctx, "@assistant")

	if eventType != "PR_REVIEW" {
		t.Errorf("eventType = %q, want PR_REVIEW", eventType)
	}
	if !strings.Contains(triggerCtx, "PR review") {
		t.Errorf("triggerCtx should mention PR review, got: %q", triggerCtx)
	}
}

func TestEventTypeAndTriggerContext_IssueComment(t *testing.T) {
	ctx := &mockGitHubContext{
		eventName: "issue_comment",
	}

	eventType, triggerCtx := eventTypeAndTriggerContext(ctx, "@assistant")

	if eventType != "GENERAL_COMMENT" {
		t.Errorf("eventType = %q, want GENERAL_COMMENT", eventType)
	}
	if !strings.Contains(triggerCtx, "issue comment") {
		t.Errorf("triggerCtx should mention issue comment, got: %q", triggerCtx)
	}
}

func TestEventTypeAndTriggerContext_IssueOpened(t *testing.T) {
	ctx := &mockGitHubContext{
		eventName:   "issues",
		eventAction: "opened",
	}

	eventType, triggerCtx := eventTypeAndTriggerContext(ctx, "@assistant")

	if eventType != "ISSUE_CREATED" {
		t.Errorf("eventType = %q, want ISSUE_CREATED", eventType)
	}
	if !strings.Contains(triggerCtx, "new issue") {
		t.Errorf("triggerCtx should mention new issue, got: %q", triggerCtx)
	}
}

func TestEventTypeAndTriggerContext_IssueLabeled(t *testing.T) {
	ctx := &mockGitHubContext{
		eventName:   "issues",
		eventAction: "labeled",
	}

	eventType, _ := eventTypeAndTriggerContext(ctx, "@assistant")

	if eventType != "ISSUE_LABELED" {
		t.Errorf("eventType = %q, want ISSUE_LABELED", eventType)
	}
}

func TestEventTypeAndTriggerContext_PullRequestOpened(t *testing.T) {
	ctx := &mockGitHubContext{
		eventName:   "pull_request",
		eventAction: "opened",
	}

	eventType, triggerCtx := eventTypeAndTriggerContext(ctx, "@assistant")

	if eventType != "PULL_REQUEST" {
		t.Errorf("eventType = %q, want PULL_REQUEST", eventType)
	}
	if !strings.Contains(triggerCtx, "opened") {
		t.Errorf("triggerCtx should mention action, got: %q", triggerCtx)
	}
}

func TestBuildPrompt_RepositoryNameFallback(t *testing.T) {
	ctx := &mockGitHubContext{
		eventName:          "issue_comment",
		repositoryFullName: "",
		repositoryOwner:    "testowner",
		repositoryName:     "testrepo",
		issueNumber:        1,
		triggerUser:        "user",
	}

	fetched := &ghdata.FetchResult{
		ContextData: ghdata.Issue{
			Title:  "Test",
			Author: ghdata.Author{Login: "user"},
			State:  "open",
		},
	}

	prompt := BuildPrompt(ctx, fetched)

	if !strings.Contains(prompt, "<repository>testowner/testrepo</repository>") {
		t.Error("Prompt should construct repository name from owner and name")
	}
}

func TestBuildPrompt_TriggerUserFallback(t *testing.T) {
	ctx := &mockGitHubContext{
		eventName:          "issue_comment",
		repositoryFullName: "owner/repo",
		issueNumber:        1,
		triggerUser:        "",
		actor:              "actor-user",
	}

	fetched := &ghdata.FetchResult{
		ContextData: ghdata.Issue{
			Title:  "Test",
			Author: ghdata.Author{Login: "actor-user"},
			State:  "open",
		},
	}

	prompt := BuildPrompt(ctx, fetched)

	if !strings.Contains(prompt, "<trigger_username>actor-user</trigger_username>") {
		t.Error("Prompt should fall back to actor when trigger_user is empty")
	}
}

func TestBuildPrompt_PRNumberOverridesIssueNumber(t *testing.T) {
	ctx := &mockGitHubContext{
		eventName:          "pull_request",
		repositoryFullName: "owner/repo",
		isPR:               true,
		issueNumber:        50,
		prNumber:           123,
		triggerUser:        "user",
	}

	fetched := &ghdata.FetchResult{
		ContextData: ghdata.PullRequest{
			Title:       "Test PR",
			Author:      ghdata.Author{Login: "user"},
			BaseRefName: "main",
			HeadRefName: "feature",
			State:       "open",
		},
	}

	prompt := BuildPrompt(ctx, fetched)

	if !strings.Contains(prompt, "<pr_number>123</pr_number>") {
		t.Error("Prompt should use PR number when both are present")
	}
}
