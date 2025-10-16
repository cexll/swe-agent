package toolconfig

import (
	"os"
	"sort"
)

// BuildAllowedTools returns the list of tools that should be allowed for the
// provider CLI. This mirrors the logic of Claude Code Action's
// buildAllowedToolsString() while keeping the return type as []string to allow
// callers to format/stringify as needed.
func BuildAllowedTools(opts Options) []string {
	// Base essential tools
	base := []string{"Edit", "MultiEdit", "Glob", "Grep", "LS", "Read", "Write"}

	// GitHub MCP tools (official tool names, no mcp__ prefix)
	base = append(base,
		// Comment + PR
		"github_update_issue_comment",
		"github_create_issue_comment",
		"github_create_pull_request",
	)
	// File ops: choose between git bash vs API push tool
	if opts.UseCommitSigning || opts.EnableGitHubFileOpsMCP {
		// Enable API-based push that supports signing on server side
		base = append(base, "github_push_files")
	} else {
		// Allow local file create/update when not using signing
		base = append(base, "github_create_or_update_file")
	}

	// Git MCP tools
	if !opts.UseCommitSigning {
		base = append(base,
			"git_status",
			"git_diff_unstaged",
			"git_diff_staged",
			"git_commit",
			"git_log",
		)
	}

	// GitHub CI MCP (optional)
	if opts.EnableGitHubCIMCP {
		base = append(base,
			"github_get_workflow_runs",
			"github_get_workflow_run",
			"github_get_job_logs",
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
// explicitly allowed, mirroring the behavior in claude-code-action.
func BuildDisallowedTools(opts Options) []string {
	// Default disallowed tools for safety (match reference implementation)
	disallowed := []string{"WebSearch", "WebFetch"}

	// Remove from defaults if explicitly allowed
	allowedSet := toSet(BuildAllowedTools(opts))
	tmp := disallowed[:0]
	for _, t := range disallowed {
		if !allowedSet[t] {
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
