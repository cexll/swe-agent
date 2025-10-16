package prompt

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ghctx "github.com/cexll/swe/internal/github"
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

	eventType, triggerCtx := eventTypeAndTriggerContext(ctx)

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

	eventType, triggerCtx := eventTypeAndTriggerContext(ctx)

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

	eventType, triggerCtx := eventTypeAndTriggerContext(ctx)

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

	eventType, triggerCtx := eventTypeAndTriggerContext(ctx)

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

	eventType, _ := eventTypeAndTriggerContext(ctx)

	if eventType != "ISSUE_LABELED" {
		t.Errorf("eventType = %q, want ISSUE_LABELED", eventType)
	}
}

func TestEventTypeAndTriggerContext_PullRequestOpened(t *testing.T) {
	ctx := &mockGitHubContext{
		eventName:   "pull_request",
		eventAction: "opened",
	}

	eventType, triggerCtx := eventTypeAndTriggerContext(ctx)

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

func TestEventTypeAndTriggerContext_UnknownEvent(t *testing.T) {
	ctx := &mockGitHubContext{eventName: "random_event"}
	et, tc := eventTypeAndTriggerContext(ctx)
	if et != strings.ToUpper("random_event") || !strings.Contains(tc, "generic") {
		t.Fatalf("unexpected mapping: %q / %q", et, tc)
	}
}

func TestBuildImageInfoAndFormatters(t *testing.T) {
	// buildImageInfo ordering and formatting
	m := map[string]string{
		"http://a/img.png": "/tmp/a.png",
		"http://b/img.jpg": "/tmp/b.jpg",
	}
	s := buildImageInfo(m)
	if !strings.Contains(s, "<images_info>") || !strings.Contains(s, "Image mappings:") {
		t.Fatalf("unexpected image info: %q", s)
	}

	// formatContext and formatComments pull fields from github.Context
	ctx := &ghctx.Context{
		Repository:  ghctx.Repository{Owner: "o", Name: "r"},
		IssueNumber: 7,
		Actor:       "alice",
		TriggerUser: "bob",
	}
	ctx.TriggerComment = &ghctx.Comment{Body: "hello"}
	fc := formatContext(ctx)
	if !strings.Contains(fc, "Repository: o/r") || !strings.Contains(fc, "#7") {
		t.Fatalf("formatContext missing fields: %q", fc)
	}
	cm := formatComments(ctx)
	if !strings.Contains(cm, "bob") || !strings.Contains(cm, "hello") {
		t.Fatalf("formatComments unexpected: %q", cm)
	}
}

func TestBuildFullPrompt_Minimal(t *testing.T) {
	// Ensure BuildFullPrompt renders template without images and with basic fields
	ctx := &ghctx.Context{
		Repository:  ghctx.Repository{Owner: "o", Name: "r"},
		IssueNumber: 9,
		BaseBranch:  "main",
		Actor:       "carol",
	}
	body, err := BuildFullPrompt(context.Background(), ctx, 1234, "feat-x")
	if err != nil {
		t.Fatalf("BuildFullPrompt error: %v", err)
	}
	if !strings.Contains(body, "<repository>o/r</repository>") || !strings.Contains(body, "<claude_comment_id>1234</claude_comment_id>") {
		t.Fatalf("BuildFullPrompt missing fields: %q", body)
	}
}

// TestTemplateContainsMCPPrefixedTools verifies that the prompt template uses correct MCP tool names
func TestTemplateContainsMCPPrefixedTools(t *testing.T) {
	ctx := &ghctx.Context{
		Repository:  ghctx.Repository{Owner: "owner", Name: "repo"},
		IssueNumber: 1,
	}

	prompt, err := BuildFullPrompt(context.Background(), ctx, 100, "branch")
	if err != nil {
		t.Fatalf("BuildFullPrompt failed: %v", err)
	}

	// Required MCP tool names that MUST be in the template
	// Note: We only check tools that are actually referenced in the template,
	// not all tools that might be available at runtime
	requiredMCPTools := []string{
		"mcp__github__add_issue_comment",
		"mcp__github__create_or_update_file",
		"mcp__github__push_files",
		"mcp__git__status",
		"mcp__git__commit",
		"mcp__git__diff_unstaged",
		"mcp__git__diff_staged",
		"mcp__git__log",
	}

	for _, tool := range requiredMCPTools {
		if !strings.Contains(prompt, tool) {
			t.Errorf("Template should contain MCP tool reference '%s' but it's missing", tool)
		}
	}
}

// TestTemplateNoLegacyToolNames ensures the template doesn't reference legacy tool names
func TestTemplateNoLegacyToolNames(t *testing.T) {
	ctx := &ghctx.Context{
		Repository:  ghctx.Repository{Owner: "owner", Name: "repo"},
		IssueNumber: 1,
	}

	prompt, err := BuildFullPrompt(context.Background(), ctx, 100, "branch")
	if err != nil {
		t.Fatalf("BuildFullPrompt failed: %v", err)
	}

	// Legacy tool names that should NOT appear (without mcp__ prefix)
	// Use specific patterns that wouldn't match valid English words
	legacyPatterns := []struct {
		pattern string
		reason  string
	}{
		{"github_add_issue_comment", "should use mcp__github__add_issue_comment"},
		{"github_create_or_update_file", "should use mcp__github__create_or_update_file"},
		{"github_push_files", "should use mcp__github__push_files"},
		{"git_status", "should use mcp__git__status"},
		{"git_commit", "should use mcp__git__commit"},
		{"git_add", "should use mcp__git__add"},
		{"git_push", "should use mcp__git__push"},
	}

	for _, lp := range legacyPatterns {
		// Check if the exact legacy pattern exists (not as part of the mcp__ version)
		if containsExactly(prompt, lp.pattern) {
			t.Errorf("Template contains legacy tool reference '%s' (%s)", lp.pattern, lp.reason)
		}
	}
}

// TestTemplateWarnsAgainstBashUsage verifies the template warns against using Bash for GitHub operations
func TestTemplateWarnsAgainstBashUsage(t *testing.T) {
	ctx := &ghctx.Context{
		Repository:  ghctx.Repository{Owner: "owner", Name: "repo"},
		IssueNumber: 1,
	}

	prompt, err := BuildFullPrompt(context.Background(), ctx, 100, "branch")
	if err != nil {
		t.Fatalf("BuildFullPrompt failed: %v", err)
	}

	// Should contain explicit warning about not using Bash/gh CLI
	warningPatterns := []string{
		"NEVER use Bash commands like 'gh api'",
		"always use the MCP tools",
	}

	for _, pattern := range warningPatterns {
		if !strings.Contains(prompt, pattern) {
			t.Errorf("Template should warn against Bash usage (missing: '%s')", pattern)
		}
	}
}

// Helper to check if string contains exact match (not as substring of another word)
func containsExactly(text, pattern string) bool {
	// Simple check: pattern exists but NOT as part of "mcp__" + pattern
	mcpVersion := "mcp__" + pattern
	if !strings.Contains(text, pattern) {
		return false
	}
	// If it exists, check it's not just the mcp__ version
	// Count both versions
	patternCount := countOccurrences(text, pattern)
	mcpCount := countOccurrences(text, mcpVersion)
	// If pattern appears more times than mcp__pattern, then we have exact matches
	return patternCount > mcpCount
}

func countOccurrences(text, pattern string) int {
	count := 0
	for i := 0; i <= len(text)-len(pattern); i++ {
		if text[i:i+len(pattern)] == pattern {
			count++
			i += len(pattern) - 1 // skip past this occurrence
		}
	}
	return count
}
