package claude

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	claudecli "github.com/lancekrogers/claude-code-go/pkg/claude"
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

// Provider implements the AI provider interface for Claude
type Provider struct {
	claudeClient *claudecli.ClaudeClient
	model        string
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

	// Create Claude Code client
	claudeClient := &claudecli.ClaudeClient{
		BinPath: "claude", // Uses claude from PATH
		DefaultOptions: &claudecli.RunOptions{
			Format: claudecli.JSONOutput,
			Model:  model,
		},
	}

	return &Provider{
		claudeClient: claudeClient,
		model:        model,
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "claude"
}

// GenerateCode generates code changes using Claude Code CLI
func (p *Provider) GenerateCode(ctx context.Context, req *CodeRequest) (*CodeResponse, error) {
	log.Printf("[Claude] Starting code generation for: %s", req.Prompt)

	// 1. List repository files
	files, err := listRepoFiles(req.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list repo files: %w", err)
	}

	// 2. Build system prompt
	systemPrompt := buildSystemPrompt(files, req.Context)

	// 3. Build user prompt
	userPrompt := fmt.Sprintf(`Task: %s

You can choose to either:

1. Provide code changes (if modifications are needed):
<file path="path/to/file.ext">
<content>
... full file content here ...
</content>
</file>

<summary>
Brief description of changes made
</summary>

2. Provide analysis/answer only (if no code changes needed):
<summary>
Your analysis, recommendations, or answer here.
You can include explanations, task lists, or any helpful information.
</summary>

Make sure to include the COMPLETE file content when providing code changes, not just the changes.`, req.Prompt)

	log.Printf("[Claude] Calling Claude Code CLI with model: %s", p.model)

	// 4. Call Claude Code CLI
	result, err := p.claudeClient.RunPromptCtx(ctx, userPrompt, &claudecli.RunOptions{
		Format:       claudecli.JSONOutput,
		Model:        p.model,
		SystemPrompt: systemPrompt,
	})

	if err != nil {
		return nil, fmt.Errorf("Claude Code CLI error: %w", err)
	}

	if result.IsError {
		return nil, fmt.Errorf("Claude Code error: %s", result.Result)
	}

	responseText := result.Result
	log.Printf("[Claude] Response length: %d characters, cost: $%.4f", len(responseText), result.CostUSD)

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

// listRepoFiles lists all files in the repository (excluding .git)
func listRepoFiles(repoPath string) ([]string, error) {
	var files []string

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Skip directories and hidden files
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(repoPath, path)
		if err != nil {
			return err
		}

		files = append(files, relPath)
		return nil
	})

	return files, err
}

// buildSystemPrompt creates the system prompt for Claude
func buildSystemPrompt(files []string, context map[string]string) string {
	fileList := strings.Join(files, "\n- ")

	prompt := fmt.Sprintf(`You are a code modification assistant working on a GitHub repository.

Repository structure:
- %s

`, fileList)

	// Add context if available
	if len(context) > 0 {
		prompt += "\nAdditional Context:\n"
		for key, value := range context {
			if value != "" {
				prompt += fmt.Sprintf("- %s: %s\n", key, value)
			}
		}
	}

	prompt += `
When making changes:
1. Understand the task thoroughly
2. Make minimal, focused changes
3. Preserve existing code style
4. Include complete file content in your response

Return your changes in this exact format:
<file path="path/to/file">
<content>
... complete file content ...
</content>
</file>

<summary>
Brief description of what was changed
</summary>`

	return prompt
}

// parseCodeResponse extracts file changes and summary from Claude's response
func parseCodeResponse(response string) (*CodeResponse, error) {
	result := &CodeResponse{
		Files: []FileChange{},
	}

	// Extract file blocks: <file path="..."><content>...</content></file>
	fileRegex := regexp.MustCompile(`(?s)<file path="([^"]+)">\s*<content>\s*(.*?)\s*</content>\s*</file>`)
	fileMatches := fileRegex.FindAllStringSubmatch(response, -1)

	for _, match := range fileMatches {
		if len(match) >= 3 {
			result.Files = append(result.Files, FileChange{
				Path:    match[1],
				Content: match[2],
			})
		}
	}

	// Extract summary: <summary>...</summary>
	summaryRegex := regexp.MustCompile(`(?s)<summary>\s*(.*?)\s*</summary>`)
	summaryMatch := summaryRegex.FindStringSubmatch(response)
	if len(summaryMatch) >= 2 {
		result.Summary = summaryMatch[1]
	} else if len(result.Files) == 0 {
		// No files and no <summary> tag, use raw response as content
		result.Summary = strings.TrimSpace(response)
	} else {
		result.Summary = "Code changes applied"
	}

	// Allow responses without file changes (analysis/Q&A/recommendations)
	// As long as there's meaningful content in the summary
	if len(result.Files) == 0 && strings.TrimSpace(result.Summary) == "" {
		return nil, fmt.Errorf("no content found in response")
	}

	return result, nil
}
