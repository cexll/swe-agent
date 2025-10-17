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

	// Should include official Git MCP tools (with mcp__git__ prefix and double git_)
	// Note: git_push is not included because mcp-server-git doesn't support push
	gitTools := []string{"mcp__git__git_status", "mcp__git__git_diff_unstaged", "mcp__git__git_diff_staged", "mcp__git__git_commit", "mcp__git__git_add", "mcp__git__git_log"}
	for _, gt := range gitTools {
		if !contains(tools, gt) {
			t.Errorf("Expected git MCP tool %s not found in allowed tools", gt)
		}
	}

	// Should include GitHub MCP tools by default (with mcp__github__ prefix)
	// Both comment tools should be present with distinct purposes:
	// - update_claude_comment: Progress tracking (coordinating comment)
	// - add_issue_comment: New content (detailed analysis, code review)
	for _, ght := range []string{"mcp__comment_updater__update_claude_comment", "mcp__github__add_issue_comment", "mcp__github__create_pull_request"} {
		if !contains(tools, ght) {
			t.Errorf("Expected GitHub tool %s not found in allowed tools", ght)
		}
	}
	if !(contains(tools, "mcp__github__create_or_update_file") || contains(tools, "mcp__github__push_files")) {
		t.Errorf("Expected one of mcp__github__create_or_update_file or mcp__github__push_files in allowed tools")
	}
}

func TestBuildAllowedTools_CommitSigning_Toggles(t *testing.T) {
	opts := Options{UseCommitSigning: true}
	tools := BuildAllowedTools(opts)
	// Git MCP tools should be disabled when signing (note: git_push not in list because mcp-server-git doesn't support push)
	for _, gt := range []string{"mcp__git__git_status", "mcp__git__git_diff_unstaged", "mcp__git__git_diff_staged", "mcp__git__git_commit", "mcp__git__git_add", "mcp__git__git_log"} {
		if contains(tools, gt) {
			t.Errorf("Did not expect git tool %s when UseCommitSigning=true", gt)
		}
	}
	// API push tool should be enabled
	if !contains(tools, "mcp__github__push_files") {
		t.Errorf("Expected mcp__github__push_files when UseCommitSigning=true")
	}
}

func TestBuildAllowedTools_BothCommentToolsEnabled(t *testing.T) {
	// Both comment tools should be present with distinct purposes
	opts := Options{}
	tools := BuildAllowedTools(opts)
	
	// Check coordinating comment tool (progress tracking)
	if !contains(tools, "mcp__comment_updater__update_claude_comment") {
		t.Error("Expected mcp__comment_updater__update_claude_comment in allowed tools (for progress tracking)")
	}
	
	// Check add_issue_comment tool (new content)
	if !contains(tools, "mcp__github__add_issue_comment") {
		t.Error("Expected mcp__github__add_issue_comment in allowed tools (for new content like code reviews)")
	}
	
	// Verify create_issue_comment alias is NOT present (to avoid confusion)
	if contains(tools, "mcp__github__create_issue_comment") {
		t.Error("mcp__github__create_issue_comment should not be present (add_issue_comment is the canonical name)")
	}
}

func TestBuildAllowedTools_WithGitHubCIMCP(t *testing.T) {
	opts := Options{
		EnableGitHubCIMCP: true,
	}
	tools := BuildAllowedTools(opts)

	ciTools := []string{
		"mcp__github__get_workflow_runs",
		"mcp__github__get_workflow_run",
		"mcp__github__get_job_logs",
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

	// Should disallow WebFetch by default (security)
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

// TestAllMCPToolsHaveCorrectPrefix verifies that all MCP tools have the mcp__ prefix
// This test ensures we don't accidentally configure tools without the correct prefix
func TestAllMCPToolsHaveCorrectPrefix(t *testing.T) {
	testCases := []struct {
		name string
		opts Options
	}{
		{"Default configuration", Options{}},
		{"With commit signing", Options{UseCommitSigning: true}},
		{"With GitHub file ops", Options{EnableGitHubFileOpsMCP: true}},
		{"With CI MCP", Options{EnableGitHubCIMCP: true}},
		{"All options enabled", Options{
			UseCommitSigning:       true,
			EnableGitHubCommentMCP: true,
			EnableGitHubFileOpsMCP: true,
			EnableGitHubCIMCP:      true,
		}},
	}

	// List of tool names that should ALWAYS have mcp__ prefix when present
	mcpPrefixRequired := []string{
		"github__update_issue_comment",
		"github__add_issue_comment",
		"github__create_issue_comment",
		"github__create_pull_request",
		"github__push_files",
		"github__create_or_update_file",
		"git__status",
		"git__commit",
		"git__add",
		"git__push",
		"git__diff_unstaged",
		"git__diff_staged",
		"git__log",
		"git__show",
		"git__branch",
		"git__create_branch",
		"github__get_workflow_runs",
		"github__get_workflow_run",
		"github__get_job_logs",
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tools := BuildAllowedTools(tc.opts)

			for _, tool := range tools {
				// Check if this tool contains a pattern that should have mcp__ prefix
				for _, pattern := range mcpPrefixRequired {
					if tool == pattern {
						t.Errorf("Found tool '%s' without mcp__ prefix (should be 'mcp__%s')", tool, pattern)
					}
				}
			}
		})
	}
}

// TestNoLegacyToolNames ensures no legacy non-prefixed tool names are present
func TestNoLegacyToolNames(t *testing.T) {
	opts := Options{}
	tools := BuildAllowedTools(opts)

	// Legacy tool names that should NOT appear (they should have mcp__ prefix)
	legacyNames := []string{
		"github_update_issue_comment",
		"github_add_issue_comment",
		"github_create_or_update_file",
		"github_push_files",
		"git_status",
		"git_commit",
		"git_add",
		"git_push",
	}

	for _, legacy := range legacyNames {
		if contains(tools, legacy) {
			t.Errorf("Found legacy tool name '%s' without mcp__ prefix", legacy)
		}
	}
}

// TestMCPPrefixConsistency verifies all GitHub and Git tools have consistent prefixes
func TestMCPPrefixConsistency(t *testing.T) {
	opts := Options{}
	tools := BuildAllowedTools(opts)

	for _, tool := range tools {
		// Check GitHub MCP tools
		if hasSubstring(tool, "github") && hasSubstring(tool, "__") {
			if !hasPrefix(tool, "mcp__github__") {
				t.Errorf("GitHub tool '%s' has inconsistent prefix (should start with 'mcp__github__')", tool)
			}
		}

		// Check Git MCP tools
		if hasSubstring(tool, "git") && hasSubstring(tool, "__") && !hasSubstring(tool, "github") {
			if !hasPrefix(tool, "mcp__git__") {
				t.Errorf("Git tool '%s' has inconsistent prefix (should start with 'mcp__git__')", tool)
			}
		}
	}
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
