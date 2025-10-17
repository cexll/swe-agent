package provider

import "context"

// Provider is the interface that all AI providers must implement
type Provider interface {
	// GenerateCode generates code changes based on the request
	GenerateCode(ctx context.Context, req *CodeRequest) (*CodeResponse, error)

	// Name returns the provider name
	Name() string
}

// CodeRequest is the unified provider request payload
type CodeRequest struct {
	// Full prompt (already built: system + user + GitHub XML)
	Prompt string
	// Repo working directory for the provider to operate in
	RepoPath string
	// Context for providers/tools (e.g., github_token, repository, issue_number, pr_number, base/head branches)
	Context map[string]string

	// Tools configuration (passed to CLI). When empty, providers should fall
	// back to their defaults to preserve backwards compatibility.
	AllowedTools    []string
	DisallowedTools []string
}

// CodeResponse is the minimal response; AI handles changes via MCP
type CodeResponse struct {
	Summary string
}
