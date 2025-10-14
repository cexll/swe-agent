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

	"github.com/cexll/swe/internal/prompt"
	"github.com/cexll/swe/internal/provider/shared"
)

// FileChange represents a file modification
type FileChange struct {
	Path    string
	Content string
}

// CodeRequest contains input for code generation
type CodeRequest struct {
	Prompt   string            // User instruction
	RepoPath string            // Repository path
	Context  map[string]string // Additional context
}

// CodeResponse contains the AI-generated code changes
type CodeResponse struct {
	Files   []FileChange // Modified files
	Summary string       // Summary of changes
	CostUSD float64      // Cost in USD
}

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

var promptManager = prompt.NewManager()

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
func callClaudeCLI(workDir, prompt, model, disallowedTools string) (*CLIResult, error) {
	// Build command arguments
	args := []string{"-p", "--output-format", "json"}
	if model != "" {
		args = append(args, "--model", model)
	}
	// Add disallowed tools if specified
	if disallowedTools != "" {
		args = append(args, "--disallowedTools", disallowedTools)
		log.Printf("[Claude CLI] Disallowed tools: %s", disallowedTools)
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
func (p *Provider) GenerateCode(ctx context.Context, req *CodeRequest) (*CodeResponse, error) {
	log.Printf("[Claude] Starting code generation (prompt length: %d chars)", len(req.Prompt))

	// Validate working directory
	if req.RepoPath == "" {
		return nil, fmt.Errorf("repository path is required")
	}
	if _, err := os.Stat(req.RepoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository path does not exist: %s", req.RepoPath)
	}

	// 1. List repository files
	files, err := promptManager.ListRepoFiles(req.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list repo files: %w", err)
	}

	// 2. Build system prompt
	systemPrompt := promptManager.BuildDefaultSystemPrompt(files, req.Context)

	// 3. Build full prompt with system and user content
	fullPrompt := fmt.Sprintf("System: %s\n\nUser: %s", systemPrompt, promptManager.BuildUserPrompt(req.Prompt))

	log.Printf("[Claude] Calling Claude CLI with model: %s in directory: %s", p.model, req.RepoPath)

	// 4. Get disallowed tools from context
	disallowedTools := ""
	if req.Context != nil {
		if tools, ok := req.Context["disallowed_tools"]; ok {
			disallowedTools = tools
		}
	}

	// 5. Call Claude CLI with correct working directory
	result, err := callClaudeCLI(req.RepoPath, fullPrompt, p.model, disallowedTools)
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
	response, err := parseCodeResponse(responseText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Set cost
	response.CostUSD = result.CostUSD

	log.Printf("[Claude] Extracted %d file changes", len(response.Files))
	return response, nil
}

// parseCodeResponse extracts file changes and summary from Claude's response
// Enhanced with multiple format support and debugging
func parseCodeResponse(response string) (*CodeResponse, error) {
	if os.Getenv("DEBUG_CLAUDE_PARSING") == "true" {
		log.Printf("[Parse] Parsing response of %d characters", len(response))
		log.Printf("[Parse] Response preview: %s...", truncateString(response, 200))
	}

	parsed, err := shared.ParseResponse("Claude", response)
	if err != nil {
		return nil, err
	}

	result := &CodeResponse{
		Summary: parsed.Summary,
		Files:   make([]FileChange, 0, len(parsed.Files)),
	}

	for _, file := range parsed.Files {
		result.Files = append(result.Files, FileChange{
			Path:    file.Path,
			Content: file.Content,
		})
	}

	if os.Getenv("DEBUG_CLAUDE_PARSING") == "true" {
		log.Printf("[Parse] Found %d file changes", len(result.Files))
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
