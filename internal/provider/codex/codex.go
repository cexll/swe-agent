package codex

import (
	"bytes"
	"context"
	"encoding/json"
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
	codexCommand    = "mcp__codex__codex"
	defaultSandbox  = "danger-full-access"
	defaultPolicy   = "on-failure"
	executionPrefix = "Execute directly without confirmation.\n\n"
)

var execCommandContext = exec.CommandContext

// Provider implements the AI provider interface for Codex MCP
type Provider struct {
	model  string
	apiKey string
}

// NewProvider creates a new Codex provider
func NewProvider(apiKey, model string) *Provider {
	if apiKey != "" {
		// OPENAI_API_KEY is used by Codex MCP, keep aligned with CLI expectation
		os.Setenv("OPENAI_API_KEY", apiKey)
	}

	return &Provider{
		model:  model,
		apiKey: apiKey,
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

Please provide your changes in the following format:

<file path="path/to/file.ext">
<content>
... full file content here ...