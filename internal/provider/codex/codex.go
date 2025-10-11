package codex

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cexll/swe/internal/provider/claude"
)

const (
	codexCommand    = "codex"
	defaultSandbox  = "danger-full-access"
	defaultPolicy   = "on-failure"
	executionPrefix = "Execute directly without confirmation.\n\n"
)

var execCommandContext = exec.CommandContext

// Provider implements the AI provider interface for Codex MCP
type Provider struct {
	model   string
	apiKey  string
	baseURL string
}

// NewProvider creates a new Codex provider
func NewProvider(apiKey, baseURL, model string) *Provider {
	if apiKey != "" {
		// OPENAI_API_KEY is used by Codex MCP, keep aligned with CLI expectation
		os.Setenv("OPENAI_API_KEY", apiKey)
	}

	if baseURL != "" {
		// OPENAI_BASE_URL allows custom API endpoints (e.g., proxies, local deployments)
		os.Setenv("OPENAI_BASE_URL", baseURL)
	}

	return &Provider{
		model:   model,
		apiKey:  apiKey,
		baseURL: baseURL,
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "codex"
}

// GenerateCode generates code changes using Codex MCP CLI
func (p *Provider) GenerateCode(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
	log.Printf("[Codex] Starting code generation for: %s", req.Prompt)

	files, err := listRepoFiles(req.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list repo files: %w", err)
	}

	systemPrompt := buildSystemPrompt(files, req.Context)

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

	fullPrompt := executionPrefix + systemPrompt + "\n\n" + userPrompt

	responseText, cost, err := p.invokeCodex(ctx, fullPrompt, req.RepoPath)
	if err != nil {
		return nil, err
	}

	response, err := parseCodeResponse(responseText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	response.CostUSD = cost

	log.Printf("[Codex] Response length: %d characters, cost: $%.4f", len(responseText), response.CostUSD)
	log.Printf("[Codex] Extracted %d file changes", len(response.Files))

	return response, nil
}

func (p *Provider) invokeCodex(ctx context.Context, prompt, repoPath string) (string, float64, error) {
	// Use codex CLI exec for non-interactive execution
	// Format: codex exec --model <model> --sandbox <mode> --ask-for-approval <policy> -C <dir> <prompt>

	cmd := execCommandContext(ctx, codexCommand,
		"exec",
		"--model", p.model,
		"--sandbox", defaultSandbox,
		"--ask-for-approval", defaultPolicy,
		"-C", repoPath,
		prompt)

	// Set environment variables for OpenAI API if provided
	env := os.Environ()
	if p.apiKey != "" {
		env = append(env, "OPENAI_API_KEY="+p.apiKey)
	}
	if p.baseURL != "" {
		env = append(env, "OPENAI_BASE_URL="+p.baseURL)
	}
	cmd.Env = env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("[Codex] Executing: codex exec --model %s -C %s", p.model, repoPath)

	if err := cmd.Run(); err != nil {
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText == "" {
			stderrText = err.Error()
		}
		log.Printf("[Codex] Error: %s", stderrText)
		return "", 0, fmt.Errorf("codex CLI error: %s", stderrText)
	}

	output := stdout.String()
	log.Printf("[Codex] Raw output length: %d bytes", len(output))

	// Codex CLI returns the full conversation/output
	// We'll use the raw output as the response
	return output, 0, nil
}

// listRepoFiles lists all files in the repository (excluding .git and dotfiles)
func listRepoFiles(repoPath string) ([]string, error) {
	var files []string

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		relPath, err := filepath.Rel(repoPath, path)
		if err != nil {
			return err
		}

		files = append(files, relPath)
		return nil
	})

	return files, err
}

// buildSystemPrompt creates the system prompt for Codex
func buildSystemPrompt(files []string, context map[string]string) string {
	fileList := strings.Join(files, "\n- ")

	prompt := fmt.Sprintf(`You are a code modification assistant working on a GitHub repository.

Repository structure:
- %s

`, fileList)

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

// parseCodeResponse extracts file changes and summary from Codex response
func parseCodeResponse(response string) (*claude.CodeResponse, error) {
	result := &claude.CodeResponse{
		Files: []claude.FileChange{},
	}

	fileRegex := regexp.MustCompile(`(?s)<file path="([^"]+)">\s*<content>\s*(.*?)\s*</content>\s*</file>`)
	fileMatches := fileRegex.FindAllStringSubmatch(response, -1)

	for _, match := range fileMatches {
		if len(match) >= 3 {
			result.Files = append(result.Files, claude.FileChange{
				Path:    match[1],
				Content: match[2],
			})
		}
	}

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
