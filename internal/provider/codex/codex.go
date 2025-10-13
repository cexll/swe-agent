package codex

import (
    "bytes"
    "context"
    "fmt"
    "log"
    "os"
    "os/exec"
    "regexp"
    "strings"
    "time"

    "github.com/cexll/swe/internal/provider/claude"
    "github.com/cexll/swe/internal/prompt"
)

const (
    codexCommand = "codex"
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
    log.Printf("[Codex] Starting code generation (prompt length: %d chars)", len(req.Prompt))

    files, err := prompt.ListRepoFiles(req.RepoPath)
    if err != nil {
        return nil, fmt.Errorf("failed to list repo files: %w", err)
    }

    // Use the shared prompt builder without provider-specific prefixes to ensure
    // identical prompts across providers (unified prompt management).
    fullPrompt := prompt.BuildFullPrompt(req.Prompt, files, req.Context, "")

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
	// Format: codex exec [OPTIONS] [PROMPT]
	// Reference: codex exec -h for all options

	// Add timeout to prevent hanging (default 10 minutes if no context deadline)
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
	}

    args := []string{
        "exec",
        "-m", p.model, // Model selection
        "--dangerously-bypass-approvals-and-sandbox", // Skip all confirmation prompts
        "-C", repoPath, // Working directory
        prompt, // Initial instructions
    }

	cmd := execCommandContext(ctx, codexCommand, args...)

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

    log.Printf("[Codex] Executing: codex exec -m %s --dangerously-bypass-approvals-and-sandbox -C %s", p.model, repoPath)
	log.Printf("[Codex] Prompt length: %d characters", len(prompt))

	startTime := time.Now()
	if err := cmd.Run(); err != nil {
		duration := time.Since(startTime)
		log.Printf("[Codex] Command failed after %v", duration)

		stderrText := strings.TrimSpace(stderr.String())
		if stderrText == "" {
			stderrText = err.Error()
		}

		// Check if it was a timeout
		if ctx.Err() == context.DeadlineExceeded {
			return "", 0, fmt.Errorf("codex CLI timeout after %v: %s", duration, stderrText)
		}

		log.Printf("[Codex] Error: %s", stderrText)
		return "", 0, fmt.Errorf("codex CLI error: %s", stderrText)
	}

	duration := time.Since(startTime)
	output := stdout.String()
	log.Printf("[Codex] Command completed in %v, output length: %d bytes", duration, len(output))

	// Codex CLI returns the full conversation/output
	// We'll use the raw output as the response
	return output, 0, nil
}

// listRepoFiles lists all files in the repository (excluding .git and dotfiles)
// Shared prompt construction now lives in internal/prompt.

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

// --- Backward-compatible shim for tests ---
// listRepoFiles is kept for tests; delegates to prompt package implementation.
func listRepoFiles(repoPath string) ([]string, error) {
    return prompt.ListRepoFiles(repoPath)
}
