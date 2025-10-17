package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cexll/swe/internal/provider"
	"github.com/cexll/swe/internal/provider/shared"
)

// CLIResult represents the result from Claude CLI
type CLIResult struct {
	Result  string  `json:"result"`
	IsError bool    `json:"isError"`
	CostUSD float64 `json:"costUSD"`
}

// Provider implements the AI provider interface for Claude
type Provider struct {
	model string
}

// NewProvider creates a new Claude provider
func NewProvider(apiKey, model string) *Provider {
	// Set environment variables for Claude Code CLI
	// Support both ANTHROPIC_API_KEY and ANTHROPIC_AUTH_TOKEN
	_ = os.Setenv("ANTHROPIC_API_KEY", apiKey)
	_ = os.Setenv("ANTHROPIC_AUTH_TOKEN", apiKey)

	// Preserve ANTHROPIC_BASE_URL if already set in environment
	// This allows using custom API endpoints
	if baseURL := os.Getenv("ANTHROPIC_BASE_URL"); baseURL != "" {
		log.Printf("[Claude] Using custom API endpoint: %s", baseURL)
	}

	return &Provider{
		model: model,
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "claude"
}

// buildMCPConfig dynamically generates MCP server configuration JSON with environment variables.
// This mirrors the approach to avoid conflicts with user's ~/.claude.json.
func buildMCPConfig(ctx map[string]string) (string, error) {
	type MCPServerConfig struct {
		Type    string            `json:"type,omitempty"`
		URL     string            `json:"url,omitempty"`
		Headers map[string]string `json:"headers,omitempty"`
		Command string            `json:"command,omitempty"`
		Args    []string          `json:"args,omitempty"`
		Env     map[string]string `json:"env,omitempty"`
	}

	type MCPConfig struct {
		MCPServers map[string]MCPServerConfig `json:"mcpServers"`
	}

	config := MCPConfig{
		MCPServers: make(map[string]MCPServerConfig),
	}

	// Add GitHub HTTP MCP server if token available
	// Uses GitHub Copilot's HTTP MCP endpoint (no Docker required)
	if githubToken := ctx["github_token"]; githubToken != "" {
		config.MCPServers["github"] = MCPServerConfig{
			Type: "http",
			URL:  "https://api.githubcopilot.com/mcp",
			Headers: map[string]string{
				"Authorization": "Bearer " + githubToken,
			},
		}
	}

	// Add Git MCP server (uvx mcp-server-git)
	if _, err := exec.LookPath("uvx"); err == nil {
		config.MCPServers["git"] = MCPServerConfig{
			Command: "uvx",
			Args:    []string{"mcp-server-git"},
		}
	}

	// Add Comment Updater MCP server if comment ID available and binary exists
	if commentID := ctx["comment_id"]; commentID != "" {
		owner := ctx["repo_owner"]
		repo := ctx["repo_name"]
		githubToken := ctx["github_token"]
		eventName := ctx["event_name"]

		if owner != "" && repo != "" && githubToken != "" {
			// Check if mcp-comment-server binary exists in PATH (防御性检查)
			if _, err := exec.LookPath("mcp-comment-server"); err == nil {
				config.MCPServers["comment_updater"] = MCPServerConfig{
					Command: "mcp-comment-server",
					Env: map[string]string{
						"GITHUB_TOKEN":      githubToken,
						"REPO_OWNER":        owner,
						"REPO_NAME":         repo,
						"CLAUDE_COMMENT_ID": commentID,
						"GITHUB_EVENT_NAME": eventName,
					},
				}
				log.Printf("[MCP Config] Added comment_updater server (comment ID: %s)", commentID)
			} else {
				log.Printf("[MCP Config] Warning: mcp-comment-server not found in PATH, comment updates via MCP will be unavailable")
			}
		}
	}

	// Add Sequential Thinking MCP server (npx @modelcontextprotocol/server-sequential-thinking)
	if _, err := exec.LookPath("npx"); err == nil {
		config.MCPServers["sequential-thinking"] = MCPServerConfig{
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-sequential-thinking"},
		}
		log.Printf("[MCP Config] Added sequential-thinking server")
	} else {
		log.Printf("[MCP Config] Warning: npx not found, sequential-thinking MCP will be unavailable")
	}

	// Add Fetch MCP server (uvx mcp-server-fetch)
	if _, err := exec.LookPath("uvx"); err == nil {
		config.MCPServers["fetch"] = MCPServerConfig{
			Command: "uvx",
			Args: []string{
				"--from",
				"git+https://github.com/cexll/mcp-server-fetch.git",
				"mcp-server-fetch",
			},
		}
		log.Printf("[MCP Config] Added fetch server")
	}

	// Log final MCP server configuration summary
	serverNames := make([]string, 0, len(config.MCPServers))
	for name := range config.MCPServers {
		serverNames = append(serverNames, name)
	}
	if len(serverNames) > 0 {
		log.Printf("[MCP Config] Total MCP servers configured: %d (%v)", len(serverNames), serverNames)
	} else {
		log.Printf("[MCP Config] Warning: No MCP servers configured")
	}

	// Marshal to JSON
	blob, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal MCP config: %w", err)
	}

	return string(blob), nil
}

// callClaudeCLIWithTools calls the Claude CLI with explicit allowed/disallowed tools.
// If lists are empty, flags are omitted to preserve CLI defaults.
func callClaudeCLIWithTools(workDir, prompt, model string, allowedTools, disallowedTools []string, mcpConfig string) (*CLIResult, error) {
	// Build command arguments
	args := []string{"-p", "--output-format", "json"}
	if model != "" {
		args = append(args, "--model", model)
	}
	if len(allowedTools) > 0 {
		allowedCSV := strings.Join(allowedTools, ",")
		args = append(args, "--allowedTools", allowedCSV)
		log.Printf("[Claude CLI] Allowed tools (%d): %s", len(allowedTools), allowedCSV)
	}
	if len(disallowedTools) > 0 {
		disallowedCSV := strings.Join(disallowedTools, ",")
		args = append(args, "--disallowedTools", disallowedCSV)
		log.Printf("[Claude CLI] Disallowed tools (%d): %s", len(disallowedTools), disallowedCSV)
	}
	// Add MCP config if provided (dynamically generated)
	if mcpConfig != "" {
		args = append(args, "--mcp-config", mcpConfig)
		log.Printf("[Claude CLI] Using dynamic MCP config (%d bytes)", len(mcpConfig))
	}

	// Create command
	cmd := exec.Command("claude", args...)
	cmd.Dir = workDir // Critical: set working directory to cloned repo
	cmd.Stdin = strings.NewReader(prompt)

	// Explicitly pass environment variables to ensure Claude CLI gets MCP config
	// Go's exec.Cmd inherits env by default if cmd.Env is nil, but we set it
	// explicitly to ensure CLAUDE_CONFIG is passed through
	cmd.Env = os.Environ()

	// Enable debug logging if requested
	if os.Getenv("DEBUG_CLAUDE_PARSING") == "true" {
		log.Printf("[Claude CLI] Working directory: %s", workDir)
		log.Printf("[Claude CLI] Command: claude %s", strings.Join(args, " "))
		log.Printf("[Claude CLI] Prompt length: %d chars", len(prompt))
	}

	// Create output buffer for later parsing
	var outputBuf bytes.Buffer

	// Enable real-time streaming: output to stdout + capture to buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &outputBuf)
	cmd.Stderr = os.Stderr

	log.Printf("[Claude CLI] Execution started, streaming output...")

	// Execute command (non-blocking for output)
	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)
	output := outputBuf.Bytes()

	if err != nil {
		outputPreview := truncateString(string(output), 1000)
		log.Printf("[Claude CLI] Command failed after %v: %v", duration, err)
		log.Printf("[Claude CLI] Output preview: %s", outputPreview)
		return nil, fmt.Errorf("claude CLI execution failed: %w (output preview: %s)", err, outputPreview)
	}

	log.Printf("[Claude CLI] Command completed in %v", duration)

	// Parse JSON response
	var result CLIResult
	if err := json.Unmarshal(output, &result); err != nil {
		outputPreview := truncateString(string(output), 1000)
		log.Printf("[Claude CLI] Failed to parse JSON response: %v", err)
		log.Printf("[Claude CLI] Raw output preview: %s", outputPreview)
		return nil, fmt.Errorf("failed to parse claude CLI JSON response: %w (output preview: %s)", err, outputPreview)
	}

	if result.IsError {
		return nil, fmt.Errorf("claude CLI error: %s", result.Result)
	}

	return &result, nil
}

// GenerateCode generates code changes using Claude Code CLI
func (p *Provider) GenerateCode(ctx context.Context, req *provider.CodeRequest) (*provider.CodeResponse, error) {
	log.Printf("[Claude] Starting code generation (prompt length: %d chars)", len(req.Prompt))

	// Validate working directory
	if req.RepoPath == "" {
		return nil, fmt.Errorf("repository path is required")
	}
	if _, err := os.Stat(req.RepoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository path does not exist: %s", req.RepoPath)
	}

	// Executor already constructed the full prompt (system + user + GH XML)
	fullPrompt := req.Prompt

	log.Printf("[Claude] Calling Claude CLI with model: %s in directory: %s", p.model, req.RepoPath)

	// Gather tools configuration
	var allowed []string
	var disallowed []string
	if len(req.AllowedTools) > 0 {
		allowed = append(allowed, req.AllowedTools...)
	}
	if len(req.DisallowedTools) > 0 {
		disallowed = append(disallowed, req.DisallowedTools...)
	}
	// Back-compat: also allow context-based disallowed tools (comma-separated)
	if req.Context != nil {
		if s, ok := req.Context["disallowed_tools"]; ok && strings.TrimSpace(s) != "" {
			disallowed = append(disallowed, s)
		}
	}

	// Build dynamic MCP configuration with environment variables
	// This replaces the static ~/.claude.json approach to avoid conflicts with user config
	mcpConfig, err := buildMCPConfig(req.Context)
	if err != nil {
		log.Printf("[Claude] Warning: failed to build MCP config: %v", err)
		mcpConfig = "" // Continue without dynamic MCP config
	} else if mcpConfig != "" {
		log.Printf("[Claude] Dynamic MCP config generated: %d bytes", len(mcpConfig))
		if os.Getenv("DEBUG_MCP_CONFIG") == "true" {
			log.Printf("[Claude] MCP config content:\n%s", mcpConfig)
		}
	}

	// Call Claude CLI with correct working directory, tool configuration, and dynamic MCP config
	result, err := callClaudeCLIWithTools(req.RepoPath, fullPrompt, p.model, allowed, disallowed, mcpConfig)
	if err != nil {
		return nil, fmt.Errorf("claude CLI error: %w", err)
	}

	responseText := result.Result
	log.Printf("[Claude] Response length: %d characters, cost: $%.4f", len(responseText), result.CostUSD)

	// Debug logging if requested
	if os.Getenv("DEBUG_CLAUDE_PARSING") == "true" {
		log.Printf("[Claude] Raw response: %s", responseText)
	}

	// 5. Parse response
	parsed, err := parseCodeResponse(responseText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Return minimal response per new interface
	log.Printf("[Claude] Response length: %d characters", len(responseText))
	return &provider.CodeResponse{Summary: parsed.Summary}, nil
}

// parseCodeResponse extracts file changes and summary from Claude's response
// Enhanced with multiple format support and debugging
func parseCodeResponse(response string) (*provider.CodeResponse, error) {
	if os.Getenv("DEBUG_CLAUDE_PARSING") == "true" {
		log.Printf("[Parse] Parsing response of %d characters", len(response))
		log.Printf("[Parse] Response preview: %s...", truncateString(response, 200))
	}

	parsed, err := shared.ParseResponse("Claude", response)
	if err != nil {
		return nil, err
	}
	result := &provider.CodeResponse{Summary: parsed.Summary}
	if os.Getenv("DEBUG_CLAUDE_PARSING") == "true" {
		log.Printf("[Parse] Summary: %s", truncateString(result.Summary, 100))
	}
	return result, nil
}

// truncateString truncates a string for logging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
