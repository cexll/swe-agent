package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/cexll/swe/internal/prompt"
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
	result := &CodeResponse{
		Files: []FileChange{},
	}

	// Debug logging if enabled
	if os.Getenv("DEBUG_CLAUDE_PARSING") == "true" {
		log.Printf("[Parse] Parsing response of %d characters", len(response))
		log.Printf("[Parse] Response preview: %s...", truncateString(response, 200))
	}

	// Primary parsing: XML-style file blocks
	result.Files = append(result.Files, parseXMLFileBlocks(response)...)

	// Fallback parsing: Markdown code blocks if no XML found
	if len(result.Files) == 0 {
		result.Files = append(result.Files, parseMarkdownCodeBlocks(response)...)
	}

	// Extract summary
	result.Summary = extractSummary(response, len(result.Files) > 0)

	// Debug results
	if os.Getenv("DEBUG_CLAUDE_PARSING") == "true" {
		log.Printf("[Parse] Found %d file changes", len(result.Files))
		log.Printf("[Parse] Summary: %s", truncateString(result.Summary, 100))
	}

	// Validation
	if len(result.Files) == 0 && strings.TrimSpace(result.Summary) == "" {
		return nil, fmt.Errorf("no content found in response")
	}

	return result, nil
}

// parseXMLFileBlocks extracts files from XML-style blocks
func parseXMLFileBlocks(response string) []FileChange {
	var files []FileChange

	// Enhanced regex for XML file blocks - more flexible with whitespace
	fileRegex := regexp.MustCompile(`(?s)<file\s+path=["']([^"']+)["']>\s*<content>\s*(.*?)\s*</content>\s*</file>`)
	fileMatches := fileRegex.FindAllStringSubmatch(response, -1)

	for _, match := range fileMatches {
		if len(match) >= 3 {
			path := strings.TrimSpace(match[1])
			content := match[2] // Don't trim content as it might be significant

			if path != "" {
				files = append(files, FileChange{
					Path:    path,
					Content: content,
				})
			}
		}
	}

	return files
}

// parseMarkdownCodeBlocks extracts files from markdown-style code blocks
func parseMarkdownCodeBlocks(response string) []FileChange {
	var files []FileChange

	// Look for patterns like:
	// ```go filename.go
	// code content
	// ```
	// or
	// **filename.go:**
	// ```go
	// code content
	// ```

	// Pattern 1: ```language filename (require language, more specific matching)
	// Only match if there's actually a filename (contains . or /) after language
	codeBlockRegex1 := regexp.MustCompile("```(\\w+)\\s+([^\\s\\n]*[./][^\\s\\n]*)\\s*\\n([\\s\\S]*?)\\n```")
	matches1 := codeBlockRegex1.FindAllStringSubmatch(response, -1)

	for _, match := range matches1 {
		if len(match) >= 4 {
			// match[1] = language, match[2] = path, match[3] = content
			path := strings.TrimSpace(match[2])
			content := match[3]

			// Regex already ensures path contains . or /, so no need to check again
			files = append(files, FileChange{
				Path:    path,
				Content: content,
			})
		}
	}

	// Pattern 2: **filename:** followed by code block
	headerRegex := regexp.MustCompile(`(?s)\*\*([^*]+)\*\*:?\s*\n` + "`" + `{3}\w*\s*\n(.*?)\n` + "`" + `{3}`)
	matches2 := headerRegex.FindAllStringSubmatch(response, -1)

	for _, match := range matches2 {
		if len(match) >= 3 {
			path := strings.TrimSpace(match[1])
			// Remove any trailing colon
			path = strings.TrimSuffix(path, ":")
			content := match[2]

			// Only consider it a file if path looks like a file path
			if strings.Contains(path, ".") || strings.Contains(path, "/") {
				files = append(files, FileChange{
					Path:    path,
					Content: content,
				})
			}
		}
	}

	return files
}

// extractSummary extracts summary from various formats
func extractSummary(response string, hasFiles bool) string {
	// Try <summary> tags first
	summaryRegex := regexp.MustCompile(`(?s)<summary>\s*(.*?)\s*</summary>`)
	summaryMatch := summaryRegex.FindStringSubmatch(response)
	if len(summaryMatch) >= 2 {
		return strings.TrimSpace(summaryMatch[1])
	}

	// Try ## Summary or ### Summary headers
	headerRegex := regexp.MustCompile(`(?s)#+\s*Summary\s*\n(.*?)(?:\n#+|$)`)
	headerMatch := headerRegex.FindStringSubmatch(response)
	if len(headerMatch) >= 2 {
		return strings.TrimSpace(headerMatch[1])
	}

	// If no files found, use entire response as summary
	if !hasFiles {
		return strings.TrimSpace(response)
	}

	// Default summary for file changes
	return "Code changes applied"
}

// truncateString truncates a string for logging
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
