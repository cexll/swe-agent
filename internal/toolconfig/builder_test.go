package toolconfig

import (
	"os"
	"testing"
)

func TestBuildAllowedTools_Defaults(t *testing.T) {
	opts := Options{}
	tools := BuildAllowedTools(opts)

	// Should include base tools
	baseTools := []string{"Read", "Write", "Edit", "MultiEdit", "Glob", "Grep", "LS", "Bash"}
	for _, bt := range baseTools {
		if !contains(tools, bt) {
			t.Errorf("Expected base tool %s not found in allowed tools", bt)
		}
	}

	// Should include safe git CLI commands via Bash
	gitCommands := []string{
		"Bash(git status)",
		"Bash(git diff)",
		"Bash(git add)",
		"Bash(git commit)",
		"Bash(git push)",
	}
	for _, gc := range gitCommands {
		if !contains(tools, gc) {
			t.Errorf("Expected git command %s not found in allowed tools", gc)
		}
	}

	// Should include safe gh CLI commands via Bash
	ghCommands := []string{
		"Bash(gh pr create)",
		"Bash(gh issue create)",
		"Bash(gh api)",
	}
	for _, ghc := range ghCommands {
		if !contains(tools, ghc) {
			t.Errorf("Expected gh command %s not found in allowed tools", ghc)
		}
	}

	// Should include essential MCP tools
	mcpTools := []string{
		"mcp__sequential-thinking__sequentialthinking",
		"mcp__fetch__fetch",
		"mcp__comment_updater__update_claude_comment",
	}
	for _, mcp := range mcpTools {
		if !contains(tools, mcp) {
			t.Errorf("Expected MCP tool %s not found in allowed tools", mcp)
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

	// Should disallow dangerous git operations
	dangerousGit := []string{
		"Bash(git push --force)",
		"Bash(git push -f)",
		"Bash(git reset --hard)",
		"Bash(git clean -fd)",
		"Bash(git branch -D)",
	}
	for _, dg := range dangerousGit {
		if !contains(tools, dg) {
			t.Errorf("Expected dangerous git operation %s in disallowed tools", dg)
		}
	}

	// Should disallow dangerous gh operations
	dangerousGh := []string{
		"Bash(gh repo delete)",
		"Bash(gh api -X DELETE)",
	}
	for _, dgh := range dangerousGh {
		if !contains(tools, dgh) {
			t.Errorf("Expected dangerous gh operation %s in disallowed tools", dgh)
		}
	}
}

func TestBuildDisallowedTools_WithExplicitAllow(t *testing.T) {
	opts := Options{
		CustomAllowedTools: []string{"WebFetch"}, // Explicitly allow WebFetch
	}
	tools := BuildDisallowedTools(opts)

	// WebFetch should be removed from disallowed since it's explicitly allowed
	if contains(tools, "WebFetch") {
		t.Error("WebFetch should not be disallowed when explicitly allowed")
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

// TestBashCommandsUsedForGitGh verifies we use Bash(git/gh) format instead of MCP
func TestBashCommandsUsedForGitGh(t *testing.T) {
	opts := Options{}
	tools := BuildAllowedTools(opts)

	// Should NOT have any mcp__git__ or mcp__github__ tools (except comment_updater)
	for _, tool := range tools {
		if hasPrefix(tool, "mcp__git__") {
			t.Errorf("Found MCP git tool '%s', should use Bash(git ...) instead", tool)
		}
		if hasPrefix(tool, "mcp__github__") && tool != "mcp__comment_updater__update_claude_comment" {
			t.Errorf("Found MCP GitHub tool '%s', should use Bash(gh ...) instead", tool)
		}
	}

	// Should have Bash(git ...) commands
	gitBashFound := false
	for _, tool := range tools {
		if hasPrefix(tool, "Bash(git ") {
			gitBashFound = true
			break
		}
	}
	if !gitBashFound {
		t.Error("Expected at least one Bash(git ...) command in allowed tools")
	}

	// Should have Bash(gh ...) commands
	ghBashFound := false
	for _, tool := range tools {
		if hasPrefix(tool, "Bash(gh ") {
			ghBashFound = true
			break
		}
	}
	if !ghBashFound {
		t.Error("Expected at least one Bash(gh ...) command in allowed tools")
	}
}

// TestInteractiveCommandsBlocked verifies interactive commands are blocked
func TestInteractiveCommandsBlocked(t *testing.T) {
	opts := Options{}
	disallowed := BuildDisallowedTools(opts)

	// Interactive git commands should be disallowed
	if !contains(disallowed, "Bash(git rebase -i)") {
		t.Error("Expected 'Bash(git rebase -i)' to be disallowed")
	}
	if !contains(disallowed, "Bash(git add -i)") {
		t.Error("Expected 'Bash(git add -i)' to be disallowed")
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
