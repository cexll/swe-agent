package toolconfig

// Options controls how allowed/disallowed tool lists are built.
//
// (buildAllowedToolsString / buildDisallowedToolsString) so the
// swe-agent CLI passes a known tool surface to the provider.
type Options struct {
	// If true, prefer MCP file ops for committing/deleting files.
	// Otherwise allow specific git Bash commands.
	UseCommitSigning bool

	// Enable MCP tools for updating Claude comments on GitHub.
	EnableGitHubCommentMCP bool

	// Enable MCP tools for file operations (commit/delete) on GitHub.
	EnableGitHubFileOpsMCP bool

	// Enable MCP tools for GitHub Actions / CI inspection.
	EnableGitHubCIMCP bool

	// Additional tools to allow (verbatim names)
	CustomAllowedTools []string

	// Additional tools to disallow (verbatim names)
	CustomDisallowedTools []string
}
