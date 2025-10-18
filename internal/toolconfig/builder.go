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
	base := []string{"Edit", "MultiEdit", "Glob", "Grep", "LS", "Read", "Write", "WebSearch", "Bash"}

	// Git CLI commands (safe operations via Bash tool)
	base = append(base,
		"Bash(git status)",
		"Bash(git diff)",
		"Bash(git log)",
		"Bash(git show)",
		"Bash(git branch)",
		"Bash(git checkout)",
		"Bash(git add)",
		"Bash(git commit)",
		"Bash(git push)",
		"Bash(git pull)",
		"Bash(git fetch)",
		"Bash(git clone)",
		"Bash(git remote)",
	)

	// GitHub CLI commands (safe operations via Bash tool)
	base = append(base,
		"Bash(gh pr create)",
		"Bash(gh pr list)",
		"Bash(gh pr view)",
		"Bash(gh pr comment)",
		"Bash(gh pr merge)",
		"Bash(gh pr close)",
		"Bash(gh pr checkout)",
		"Bash(gh issue create)",
		"Bash(gh issue list)",
		"Bash(gh issue view)",
		"Bash(gh issue comment)",
		"Bash(gh issue close)",
		"Bash(gh repo clone)",
		"Bash(gh repo view)",
		"Bash(gh api)",
	)

	// Custom MCP tools (minimal set)
	base = append(base,
		"mcp__sequential-thinking__sequentialthinking", // Deep reasoning
		"mcp__fetch__fetch",                            // Web content fetching
		"mcp__comment_updater__update_claude_comment",  // Progress tracking (coordinating comment)
	)

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
	// Default disallowed tools for safety
	disallowed := []string{
		"WebFetch", // Prefer mcp__fetch__fetch for web content

		// Dangerous git operations (prevent data loss)
		"Bash(git push --force)",
		"Bash(git push -f)",
		"Bash(git push --force-with-lease)",
		"Bash(git reset --hard)",
		"Bash(git clean -fd)",
		"Bash(git clean -f)",
		"Bash(git branch -D)",
		"Bash(git tag -d)",
		"Bash(git rebase -i)", // Interactive mode not supported
		"Bash(git add -i)",    // Interactive mode not supported
		"Bash(rm -rf .git)",
		"Bash(rm -rf *)",

		// Dangerous gh CLI operations
		"Bash(gh repo delete)",
		"Bash(gh api -X DELETE)",
		"Bash(gh api --method DELETE)",
	}

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
