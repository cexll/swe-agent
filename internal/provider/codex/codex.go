package codex

import (
	"bufio"
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
)

const (
	codexCommand    = "codex"
	executionPrefix = "Execute directly without confirmation.\n\n"
)

var execCommandContext = exec.CommandContext

// No prompt manager here; executor builds the full prompt already

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
		_ = os.Setenv("OPENAI_API_KEY", apiKey)
	}

	if baseURL != "" {
		// OPENAI_BASE_URL allows custom API endpoints (e.g., proxies, local deployments)
		_ = os.Setenv("OPENAI_BASE_URL", baseURL)
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
func (p *Provider) GenerateCode(ctx context.Context, req *provider.CodeRequest) (*provider.CodeResponse, error) {
	log.Printf("[Codex] Starting code generation (prompt length: %d chars)", len(req.Prompt))

	// Build dynamic MCP configuration (writes to ~/.codex/config.toml)
	if err := buildCodexMCPConfig(req.Context); err != nil {
		log.Printf("[Codex] Warning: failed to build MCP config: %v", err)
		// Continue without dynamic MCP config
	} else {
		log.Printf("[Codex] Dynamic MCP config written to ~/.codex/config.toml")
		if os.Getenv("DEBUG_MCP_CONFIG") == "true" {
			if home, err := os.UserHomeDir(); err == nil {
				configPath := home + string(os.PathSeparator) + ".codex" + string(os.PathSeparator) + "config.toml"
				if content, err := os.ReadFile(configPath); err == nil {
					log.Printf("[Codex] MCP config content:\n%s", string(content))
				}
			}
		}
	}

	// Provide GitHub token to MCP tools via env (backup method)
	if req.Context != nil {
		if tok, ok := req.Context["github_token"]; ok && tok != "" {
			_ = os.Setenv("GITHUB_TOKEN", tok)
			_ = os.Setenv("GH_TOKEN", tok)
		}
	}
	// Ensure sandbox runs with full access per instruction
	_ = os.Setenv("SANDBOX_MODE", "danger-full-access")

	// Executor already constructed the full prompt (system + user + GH XML)
	fullPrompt := executionPrefix + req.Prompt

	responseText, err := p.invokeCodex(ctx, fullPrompt, req.RepoPath)
	if err != nil {
		return nil, err
	}

	// We only need to return a summary for bookkeeping.
	log.Printf("[Codex] Response length: %d characters", len(responseText))
	return &provider.CodeResponse{Summary: truncateLogString(responseText, 2000)}, nil
}

func (p *Provider) invokeCodex(ctx context.Context, prompt, repoPath string) (string, error) {
	ctx, cancel := ensureCodexTimeout(ctx)
	defer cancel()

	cmd, stdout, stderr := p.buildCodexCommand(ctx, repoPath, prompt)

	log.Printf("[Codex] Executing: codex exec -m %s -c model_reasoning_effort=\"high\" --dangerously-bypass-approvals-and-sandbox -C %s (streaming output...)", p.model, repoPath)
	log.Printf("[Codex] Prompt length: %d characters", len(prompt))

	startTime := time.Now()
	if err := cmd.Run(); err != nil {
		duration := time.Since(startTime)
		log.Printf("[Codex] Command failed after %v", duration)

		stderrPreview := summarizeCodexError(err, stdout, stderr)
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("codex CLI timeout after %v: %s", duration, stderrPreview)
		}

		log.Printf("[Codex] Error: %s", stderrPreview)
		return "", fmt.Errorf("codex CLI error: %s", stderrPreview)
	}

	duration := time.Since(startTime)
	output := stdout.String()
	parsedOutput := aggregateCodexOutput(output)
	if parsedOutput == "" {
		parsedOutput = strings.TrimSpace(output)
	}

	log.Printf("[Codex] Command completed in %v, output length: %d bytes", duration, len(output))

	return parsedOutput, nil
}

func truncateLogString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}

	if len(s) <= maxLen {
		return s
	}

	const marker = "\n... (truncated) ...\n"

	// For very small limits, prioritise exposing the tail without spending space on markers.
	if maxLen <= len(marker)+32 {
		return s[len(s)-maxLen:]
	}

	headLen := maxLen / 4
	tailLen := maxLen - headLen - len(marker)

	if tailLen <= 0 {
		// Prefer preserving the tail since it usually contains the actionable error.
		return marker + s[len(s)-(maxLen-len(marker)):]
	}

	head := ""
	if headLen > 0 {
		head = s[:headLen]
	}

	tail := s[len(s)-tailLen:]

	if head == "" {
		return marker + tail
	}

	return head + marker + tail
}

func aggregateCodexOutput(output string) string {
	s := strings.TrimSpace(output)
	if s == "" {
		return ""
	}

	scanner := bufio.NewScanner(strings.NewReader(s))
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 5*1024*1024)

	var sections []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if msg, handled := extractMessageFromJSONLine(line); handled {
			if msg != "" {
				sections = append(sections, msg)
			}
			continue
		}

		sections = append(sections, line)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[Codex] Warning: failed to scan JSON output: %v", err)
	}

	if len(sections) == 0 {
		return s
	}

	return strings.Join(sections, "\n\n")
}

func extractMessageFromJSONLine(line string) (string, bool) {
	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(line), &envelope); err != nil {
		return "", false
	}

	if msg, ok := getString(envelope, "message"); ok && msg != "" {
		return msg, true
	}

	if itemVal, ok := envelope["item"]; ok && itemVal != nil {
		if msg := extractTextFromItem(itemVal); msg != "" {
			return msg, true
		}
		return "", true
	}

	return "", true
}

func extractTextFromItem(item interface{}) string {
	itemMap, ok := item.(map[string]interface{})
	if !ok {
		return ""
	}

	if text, ok := getString(itemMap, "text"); ok && text != "" {
		return text
	}

	if contentVal, ok := itemMap["content"]; ok {
		switch content := contentVal.(type) {
		case []interface{}:
			var parts []string
			for _, raw := range content {
				if segmentMap, ok := raw.(map[string]interface{}); ok {
					if text, ok := getString(segmentMap, "text"); ok && text != "" {
						parts = append(parts, text)
					}
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "\n")
			}
		}
	}

	return ""
}

func getString(m map[string]interface{}, key string) (string, bool) {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str, true
		}
	}
	return "", false
}

func ensureCodexTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, 10*time.Minute)
}

func (p *Provider) buildCodexCommand(ctx context.Context, repoPath, prompt string) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer) {
	args := []string{
		"exec",
		"-m", p.model,
		"-c", `model_reasoning_effort="high"`,
		"--dangerously-bypass-approvals-and-sandbox",
		"--json",
		"-C", repoPath,
		prompt,
	}

	cmd := execCommandContext(ctx, codexCommand, args...)

	env := os.Environ()
	if p.apiKey != "" {
		env = append(env, "OPENAI_API_KEY="+p.apiKey)
	}
	if p.baseURL != "" {
		env = append(env, "OPENAI_BASE_URL="+p.baseURL)
	}
	// Pass through GitHub token for MCP tools
	if gh := os.Getenv("GITHUB_TOKEN"); gh != "" {
		env = append(env, "GITHUB_TOKEN="+gh, "GH_TOKEN="+gh)
	}
	// Prefer request-scoped token if provided in context
	// Note: executor should set this env before invoking provider, but we also
	// propagate if present in req.Context to be explicit.
	// (We cannot read req here, so ensure executor sets process env.)
	env = append(env, "SANDBOX_MODE=danger-full-access")
	cmd.Env = env

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Enable real-time streaming for stdout and stderr
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)

	return cmd, &stdout, &stderr
}

func summarizeCodexError(runErr error, stdout, stderr *bytes.Buffer) string {
	stderrText := strings.TrimSpace(stderr.String())
	stdoutText := strings.TrimSpace(stdout.String())

	if stderrText == "" {
		if parsed := aggregateCodexOutput(stdoutText); parsed != "" {
			stderrText = parsed
		} else if stdoutText != "" {
			stderrText = stdoutText
		}
	}

	if stderrText == "" && runErr != nil {
		stderrText = runErr.Error()
	}

	return truncateLogString(stderrText, 1000)
}

// buildCodexMCPConfig dynamically generates Codex MCP configuration TOML file.
// This writes to ~/.codex/config.toml to configure MCP servers with runtime context.
func buildCodexMCPConfig(ctx map[string]string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	configDir := home + string(os.PathSeparator) + ".codex"
	configPath := configDir + string(os.PathSeparator) + "config.toml"

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("create codex config dir: %w", err)
	}

	// Build TOML configuration
	var sb strings.Builder
	sb.WriteString("# Dynamically generated Codex configuration\n")
	sb.WriteString("model = \"gpt-5-codex\"\n")
	sb.WriteString("model_reasoning_effort = \"high\"\n")
	sb.WriteString("model_reasoning_summary = \"detailed\"\n")
	sb.WriteString("approval_policy = \"never\"\n")
	sb.WriteString("sandbox_mode = \"danger-full-access\"\n")
	sb.WriteString("disable_response_storage = true\n")
	sb.WriteString("network_access = true\n\n")

	// Add GitHub HTTP MCP server if token available
	if githubToken := ctx["github_token"]; githubToken != "" {
		sb.WriteString("[mcp_servers.github]\n")
		sb.WriteString("type = \"http\"\n")
		sb.WriteString("url = \"https://api.githubcopilot.com/mcp\"\n\n")
		sb.WriteString("[mcp_servers.github.headers]\n")
		sb.WriteString(fmt.Sprintf("Authorization = \"Bearer %s\"\n\n", githubToken))
	}

	// Add Git MCP server (uvx mcp-server-git)
	if _, err := exec.LookPath("uvx"); err == nil {
		sb.WriteString("[mcp_servers.git]\n")
		sb.WriteString("command = \"uvx\"\n")
		sb.WriteString("args = [\"mcp-server-git\"]\n\n")
	}

	// Add Comment Updater MCP server if comment ID available
	if commentID := ctx["comment_id"]; commentID != "" {
		owner := ctx["repo_owner"]
		repo := ctx["repo_name"]
		githubToken := ctx["github_token"]
		eventName := ctx["event_name"]

		if owner != "" && repo != "" && githubToken != "" {
			sb.WriteString("[mcp_servers.comment_updater]\n")
			sb.WriteString("command = \"mcp-comment-server\"\n\n")
			sb.WriteString("[mcp_servers.comment_updater.env]\n")
			sb.WriteString(fmt.Sprintf("GITHUB_TOKEN = \"%s\"\n", githubToken))
			sb.WriteString(fmt.Sprintf("REPO_OWNER = \"%s\"\n", owner))
			sb.WriteString(fmt.Sprintf("REPO_NAME = \"%s\"\n", repo))
			sb.WriteString(fmt.Sprintf("CLAUDE_COMMENT_ID = \"%s\"\n", commentID))
			if eventName != "" {
				sb.WriteString(fmt.Sprintf("GITHUB_EVENT_NAME = \"%s\"\n", eventName))
			}
			sb.WriteString("\n")
		}
	}

	// Add Sequential Thinking MCP server (npx @modelcontextprotocol/server-sequential-thinking)
	if _, err := exec.LookPath("npx"); err == nil {
		sb.WriteString("[mcp_servers.sequential_thinking]\n")
		sb.WriteString("command = \"npx\"\n")
		sb.WriteString("args = [\"-y\", \"@modelcontextprotocol/server-sequential-thinking\"]\n\n")
		log.Printf("[Codex MCP] Added sequential-thinking server")
	}

	// Add Fetch MCP server (uvx mcp-server-fetch)
	if _, err := exec.LookPath("uvx"); err == nil {
		sb.WriteString("[mcp_servers.fetch]\n")
		sb.WriteString("command = \"uvx\"\n")
		sb.WriteString("args = [\"--from\", \"git+https://github.com/cexll/mcp-server-fetch.git\", \"mcp-server-fetch\"]\n\n")
		log.Printf("[Codex MCP] Added fetch server")
	}

	// Write configuration file
	if err := os.WriteFile(configPath, []byte(sb.String()), 0o600); err != nil {
		return fmt.Errorf("write codex config: %w", err)
	}

	log.Printf("[Codex] MCP config written to: %s", configPath)
	return nil
}
