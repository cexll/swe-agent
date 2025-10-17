package toolconfig

import (
	"os"
	"sort"
)

// BuildAllowedTools returns the list of tools that should be allowed for the
// provider CLI.
// buildAllowedToolsString() while keeping the return type as []string to allow
// callers to format/stringify as needed.
func BuildAllowedTools(opts Options) []string {
	// Base essential tools
	base := []string{"Edit", "MultiEdit", "Glob", "Grep", "LS", "Read", "Write", "WebSearch", "WebFetch", "fetch__fetch"}

	// GitHub MCP tools - Full capability matrix (requires mcp__github__ prefix for MCP server tools)
	base = append(base,
		// Comment operations
		"mcp__comment_updater__update_claude_comment", // PRIMARY for progress tracking
		"mcp__github__add_issue_comment",              // Secondary for detailed analysis
		"mcp__github__get_issue_comments",

		// Issue management
		"mcp__github__create_issue",      // Task decomposition
		"mcp__github__update_issue",      // Modify issue content
		"mcp__github__close_issue",       // Close completed issues
		"mcp__github__reopen_issue",      // Reopen closed issues
		"mcp__github__list_issues",       // Query issues
		"mcp__github__assign_issue",      // Assign issues to users
		"mcp__github__create_issue_comment", // Alternative name for add_issue_comment

		// Pull request management
		"mcp__github__create_pull_request",
		"mcp__github__merge_pull_request",  // Merge approved PRs
		"mcp__github__close_pull_request",  // Close PR without merging
		"mcp__github__request_reviewers",   // Request specific reviewers
		"mcp__github__create_and_submit_pull_request_review", // Submit code review
		"mcp__github__add_comment_to_pending_review",
		"mcp__github__create_pending_pull_request_review",

		// Label & milestone management
		"mcp__github__add_labels",      // Add labels to issues/PRs
		"mcp__github__remove_labels",   // Remove labels
		"mcp__github__list_labels",     // List available labels
		"mcp__github__create_label",    // Create new label
		"mcp__github__create_milestone", // Create project milestones
		"mcp__github__update_milestone", // Update milestones

		// Branch management
		"mcp__github__create_branch",
		"mcp__github__list_branches",

		// Organization & projects
		"mcp__github__add_to_project", // Add issues/PRs to project boards
		"mcp__github__assign_copilot_to_issue", // Assign Copilot

		// Repository management
		"mcp__github__list_repositories",
		"mcp__github__get_repository",
		"mcp__github__create_discussion",

		// Search
		"mcp__github__search_code",
		"mcp__github__search_issues",
		"mcp__github__search_repositories",
	)
	// File ops: choose between git bash vs API push tool
	if opts.UseCommitSigning || opts.EnableGitHubFileOpsMCP {
		// Enable API-based push that supports signing on server side (requires mcp__ prefix)
		base = append(base, "mcp__github__push_files")
	} else {
		// Allow local file create/update when not using signing
		base = append(base, "mcp__github__create_or_update_file")
	}

	// Git MCP tools (requires mcp__git__ prefix for MCP server tools)
	if !opts.UseCommitSigning {
		base = append(base,
			"mcp__git__status",
			"mcp__git__diff_unstaged",
			"mcp__git__diff_staged",
			"mcp__git__commit",
			"mcp__git__add",
			"mcp__git__push",
			"mcp__git__branch",
			"mcp__git__log",
			"mcp__git__show",
			"mcp__git__create_branch",
		)
	}

	// GitHub CI MCP (optional, requires mcp__github__ prefix)
	if opts.EnableGitHubCIMCP {
		base = append(base,
			"mcp__github__get_workflow_runs",
			"mcp__github__get_workflow_run",
			"mcp__github__get_job_logs",
		)
	}

	// Append any custom tools last
	if len(opts.CustomAllowedTools) > 0 {
		base = append(base, opts.CustomAllowedTools...)
	}

	sort.Strings(base)
	return unique(base)
}

// BuildDisallowedTools returns a default-restrictive set and merges any custom
// entries. It also removes tools from the default blocklist if they are
// explicitly allowed, mirroring the behavior.
func BuildDisallowedTools(opts Options) []string {
	// Default disallowed tools for safety (match reference implementation)
	// WebSearch and WebFetch are disallowed by default for security
	disallowed := []string{"WebSearch", "WebFetch"}

	// Remove from defaults if explicitly allowed in CustomAllowedTools
	customAllowedSet := toSet(opts.CustomAllowedTools)
	tmp := disallowed[:0]
	for _, t := range disallowed {
		if !customAllowedSet[t] {
			tmp = append(tmp, t)
		}
	}
	disallowed = tmp

	// Merge custom disallowed
	if len(opts.CustomDisallowedTools) > 0 {
		disallowed = append(disallowed, opts.CustomDisallowedTools...)
	}

	// Allow overrides via env DISALLOWED_TOOLS for convenience/back-compat
	if extra := os.Getenv("DISALLOWED_TOOLS"); extra != "" {
		// The executor may already parse this, but we also honor it here for
		// library usage outside the executor.
		for _, part := range splitCSV(extra) {
			if part != "" {
				disallowed = append(disallowed, part)
			}
		}
	}

	sort.Strings(disallowed)
	return unique(disallowed)
}

func toSet(list []string) map[string]bool {
	m := make(map[string]bool, len(list))
	for _, v := range list {
		m[v] = true
	}
	return m
}

func unique(list []string) []string {
	if len(list) < 2 {
		return list
	}
	out := make([]string, 0, len(list))
	seen := make(map[string]struct{}, len(list))
	for _, s := range list {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func splitCSV(s string) []string {
	var res []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if i > start {
				res = append(res, s[start:i])
			} else {
				res = append(res, "")
			}
			start = i + 1
		}
	}
	return res
}
