package claude

import (
	"context"
	"encoding/json"
	"fmt"
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
	os.Setenv("ANTHROPIC_API_KEY", apiKey)
	os.Setenv("ANTHROPIC_AUTH_TOKEN", apiKey)

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

// callClaudeCLI calls the Claude CLI directly with proper working directory
// Backwards-compatible wrapper retained for tests. Prefer callClaudeCLIWithTools.
func callClaudeCLI(workDir, prompt, model string) (*CLIResult, error) {
    return callClaudeCLIWithTools(workDir, prompt, model, nil, nil)
}

// callClaudeCLIWithTools calls the Claude CLI with explicit allowed/disallowed tools.
// If lists are empty, flags are omitted to preserve CLI defaults.
func callClaudeCLIWithTools(workDir, prompt, model string, allowedTools, disallowedTools []string) (*CLIResult, error) {
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

	// Create command
	cmd := exec.Command("claude", args...)
	cmd.Dir = workDir // Critical: set working directory to cloned repo
	cmd.Stdin = strings.NewReader(prompt)

	// Enable debug logging if requested
	if os.Getenv("DEBUG_CLAUDE_PARSING") == "true" {
		log.Printf("[Claude CLI] Working directory: %s", workDir)
		log.Printf("[Claude CLI] Command: claude %s", strings.Join(args, " "))
		log.Printf("[Claude CLI] Prompt length: %d chars", len(prompt))
	}

	// Execute command
	start := time.Now()
	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

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

	// Call Claude CLI with correct working directory and tool configuration
	result, err := callClaudeCLIWithTools(req.RepoPath, fullPrompt, p.model, allowed, disallowed)
	if err != nil {
		return nil, fmt.Errorf("Claude CLI error: %w", err)
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
