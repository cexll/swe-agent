package toolconfig

import (
	"os"
	"testing"
)

func TestBuildAllowedTools_Defaults(t *testing.T) {
	opts := Options{}
	tools := BuildAllowedTools(opts)

	// Should include base tools
	baseTools := []string{"Read", "Write", "Edit", "MultiEdit", "Glob", "Grep", "LS"}
	for _, bt := range baseTools {
		if !contains(tools, bt) {
			t.Errorf("Expected base tool %s not found in allowed tools", bt)
		}
	}

	// Should include official Git MCP tools
	gitTools := []string{"git_status", "git_diff_unstaged", "git_diff_staged", "git_commit", "git_log"}
	for _, gt := range gitTools {
		if !contains(tools, gt) {
			t.Errorf("Expected git MCP tool %s not found in allowed tools", gt)
		}
	}

	// Should include GitHub MCP tools by default (no mcp__ prefix). One of file op tools must be present.
	for _, ght := range []string{"github_update_issue_comment", "github_create_issue_comment", "github_create_pull_request"} {
		if !contains(tools, ght) {
			t.Errorf("Expected GitHub tool %s not found in allowed tools", ght)
		}
	}
	if !(contains(tools, "github_create_or_update_file") || contains(tools, "github_push_files")) {
		t.Errorf("Expected one of github_create_or_update_file or github_push_files in allowed tools")
	}
}

func TestBuildAllowedTools_CommitSigning_Toggles(t *testing.T) {
	opts := Options{UseCommitSigning: true}
	tools := BuildAllowedTools(opts)
	// Git MCP tools should be disabled when signing
	for _, gt := range []string{"git_status", "git_diff_unstaged", "git_diff_staged", "git_commit", "git_log"} {
		if contains(tools, gt) {
			t.Errorf("Did not expect git tool %s when UseCommitSigning=true", gt)
		}
	}
	// API push tool should be enabled
	if !contains(tools, "github_push_files") {
		t.Errorf("Expected github_push_files when UseCommitSigning=true")
	}
}

func TestBuildAllowedTools_GitHubCommentAlwaysEnabled(t *testing.T) {
	// github_update_issue_comment should always be present
	opts := Options{}
	tools := BuildAllowedTools(opts)
	if !contains(tools, "github_update_issue_comment") {
		t.Error("Expected github_update_issue_comment in allowed tools by default")
	}
}

func TestBuildAllowedTools_WithGitHubCIMCP(t *testing.T) {
	opts := Options{
		EnableGitHubCIMCP: true,
	}
	tools := BuildAllowedTools(opts)

	ciTools := []string{
		"github_get_workflow_runs",
		"github_get_workflow_run",
		"github_get_job_logs",
	}
	for _, ct := range ciTools {
		if !contains(tools, ct) {
			t.Errorf("Expected CI tool %s when EnableGitHubCIMCP is true", ct)
		}
	}
}

func TestBuildAllowedTools_WithCustomTools(t *testing.T) {
	opts := Options{
		CustomAllowedTools: []string{"CustomTool1", "CustomTool2"},
	}
	tools := BuildAllowedTools(opts)

	if !contains(tools, "CustomTool1") {
		t.Error("Expected CustomTool1 in allowed tools")
	}
	if !contains(tools, "CustomTool2") {
		t.Error("Expected CustomTool2 in allowed tools")
	}
}

func TestBuildDisallowedTools_Defaults(t *testing.T) {
	opts := Options{}
	tools := BuildDisallowedTools(opts)

	// Should disallow WebSearch and WebFetch by default (security)
	if !contains(tools, "WebSearch") {
		t.Error("Expected WebSearch in default disallowed tools")
	}
	if !contains(tools, "WebFetch") {
		t.Error("Expected WebFetch in default disallowed tools")
	}
}

func TestBuildDisallowedTools_WithExplicitAllow(t *testing.T) {
	opts := Options{
		CustomAllowedTools: []string{"WebSearch"}, // Explicitly allow WebSearch
	}
	tools := BuildDisallowedTools(opts)

	// WebSearch should be removed from disallowed since it's explicitly allowed
	if contains(tools, "WebSearch") {
		t.Error("WebSearch should not be disallowed when explicitly allowed")
	}

	// WebFetch should still be disallowed
	if !contains(tools, "WebFetch") {
		t.Error("WebFetch should remain disallowed")
	}
}

func TestBuildDisallowedTools_WithCustomDisallowed(t *testing.T) {
	opts := Options{
		CustomDisallowedTools: []string{"DangerousTool", "AnotherBadTool"},
	}
	tools := BuildDisallowedTools(opts)

	if !contains(tools, "DangerousTool") {
		t.Error("Expected DangerousTool in disallowed tools")
	}
	if !contains(tools, "AnotherBadTool") {
		t.Error("Expected AnotherBadTool in disallowed tools")
	}
}

func TestBuildDisallowedTools_FromEnv(t *testing.T) {
	// Set environment variable
	os.Setenv("DISALLOWED_TOOLS", "EnvTool1,EnvTool2")
	defer os.Unsetenv("DISALLOWED_TOOLS")

	opts := Options{}
	tools := BuildDisallowedTools(opts)

	if !contains(tools, "EnvTool1") {
		t.Error("Expected EnvTool1 from DISALLOWED_TOOLS env")
	}
	if !contains(tools, "EnvTool2") {
		t.Error("Expected EnvTool2 from DISALLOWED_TOOLS env")
	}
}

func TestBuildAllowedTools_NoDuplicates(t *testing.T) {
	opts := Options{
		CustomAllowedTools: []string{"Read", "Write"}, // Duplicates of base tools
	}
	tools := BuildAllowedTools(opts)

	// Count occurrences of "Read"
	count := 0
	for _, tool := range tools {
		if tool == "Read" {
			count++
		}
	}

	if count > 1 {
		t.Errorf("Found %d instances of 'Read', expected 1 (unique() should deduplicate)", count)
	}
}

func TestBuildAllowedTools_Sorted(t *testing.T) {
	opts := Options{
		CustomAllowedTools: []string{"ZZZ", "AAA"},
	}
	tools := BuildAllowedTools(opts)

	// Verify sorted order
	for i := 1; i < len(tools); i++ {
		if tools[i-1] > tools[i] {
			t.Errorf("Tools not sorted: %s comes after %s", tools[i-1], tools[i])
		}
	}
}

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
