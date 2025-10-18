package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewProvider(t *testing.T) {
	apiKey := "sk-ant-test-key"
	model := "claude-3-opus-20240229"

	provider := NewProvider(apiKey, model)

	if provider == nil {
		t.Fatal("NewProvider() returned nil")
	}

	if provider.Name() != "claude" {
		t.Errorf("Name() = %s, want claude", provider.Name())
	}

	if provider.model != model {
		t.Errorf("model = %s, want %s", provider.model, model)
	}

	envKey := os.Getenv("ANTHROPIC_API_KEY")
	if envKey != apiKey {
		t.Errorf("ANTHROPIC_API_KEY = %s, want %s", envKey, apiKey)
	}
}

func TestProvider_Name(t *testing.T) {
	provider := NewProvider("test-key", "test-model")

	if got := provider.Name(); got != "claude" {
		t.Errorf("Name() = %s, want claude", got)
	}
}

type testMCPServer struct {
	Type    string            `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

type testMCPConfig struct {
	MCPServers map[string]testMCPServer `json:"mcpServers"`
}

func setUVXAvailability(t *testing.T, available bool) {
	t.Helper()

	originalPath := os.Getenv("PATH")
	t.Cleanup(func() {
		if err := os.Setenv("PATH", originalPath); err != nil {
			t.Fatalf("restore PATH: %v", err)
		}
	})

	if available {
		dir := t.TempDir()
		uvxPath := filepath.Join(dir, "uvx")
		if err := os.WriteFile(uvxPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("create uvx stub: %v", err)
		}
		newPath := dir
		if originalPath != "" {
			newPath = dir + string(os.PathListSeparator) + originalPath
		}
		if err := os.Setenv("PATH", newPath); err != nil {
			t.Fatalf("set PATH: %v", err)
		}
		return
	}

	emptyDir := t.TempDir()
	if err := os.Setenv("PATH", emptyDir); err != nil {
		t.Fatalf("set PATH without uvx: %v", err)
	}
}

// setMCPCommentServerAvailability sets or unsets mcp-comment-server in PATH for testing
func setMCPCommentServerAvailability(t *testing.T, available bool) {
	t.Helper()

	originalPath := os.Getenv("PATH")
	t.Cleanup(func() {
		if err := os.Setenv("PATH", originalPath); err != nil {
			t.Fatalf("restore PATH: %v", err)
		}
	})

	if available {
		dir := t.TempDir()
		mcpPath := filepath.Join(dir, "mcp-comment-server")
		if err := os.WriteFile(mcpPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("create mcp-comment-server stub: %v", err)
		}
		newPath := dir
		if originalPath != "" {
			newPath = dir + string(os.PathListSeparator) + originalPath
		}
		if err := os.Setenv("PATH", newPath); err != nil {
			t.Fatalf("set PATH: %v", err)
		}
	}
}

func decodeMCPConfig(t *testing.T, raw string) testMCPConfig {
	t.Helper()

	var cfg testMCPConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal MCP config: %v\nraw: %s", err, raw)
	}
	return cfg
}

func TestBuildMCPConfig_FullContext(t *testing.T) {
	cases := []struct {
		name string
		ctx  map[string]string
	}{
		{
			name: "uvxAvailable",
			ctx: map[string]string{
				"github_token": "ghs_full",
				"comment_id":   "1234",
				"repo_owner":   "octocat",
				"repo_name":    "hello-world",
				"event_name":   "issue_comment",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setUVXAvailability(t, true)
			setMCPCommentServerAvailability(t, true)

			raw, err := buildMCPConfig(tc.ctx)
			if err != nil {
				t.Fatalf("buildMCPConfig error: %v", err)
			}

			cfg := decodeMCPConfig(t, raw)

			if len(cfg.MCPServers) != 3 {
				t.Fatalf("expected 3 MCP servers (comment_updater, sequential-thinking, fetch), got %d", len(cfg.MCPServers))
			}

			comment, ok := cfg.MCPServers["comment_updater"]
			if !ok {
				t.Fatalf("comment_updater MCP server missing")
			}
			if comment.Command != "mcp-comment-server" {
				t.Fatalf("comment_updater command mismatch: %s", comment.Command)
			}
			env := comment.Env
			wantEnv := map[string]string{
				"GITHUB_TOKEN":      tc.ctx["github_token"],
				"REPO_OWNER":        tc.ctx["repo_owner"],
				"REPO_NAME":         tc.ctx["repo_name"],
				"CLAUDE_COMMENT_ID": tc.ctx["comment_id"],
				"GITHUB_EVENT_NAME": tc.ctx["event_name"],
			}
			for key, want := range wantEnv {
				if got := env[key]; got != want {
					t.Fatalf("comment_updater env[%s] = %q, want %q", key, got, want)
				}
			}
		})
	}
}

func TestBuildMCPConfig_GitHubOnly(t *testing.T) {
	cases := []struct {
		name string
		uvx  bool
	}{
		{name: "uvxAvailable", uvx: true},
		{name: "uvxMissing", uvx: false},
	}

	ctx := map[string]string{
		"github_token": "ghs_only",
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setUVXAvailability(t, tc.uvx)

			raw, err := buildMCPConfig(ctx)
			if err != nil {
				t.Fatalf("buildMCPConfig error: %v", err)
			}

			cfg := decodeMCPConfig(t, raw)

			// Should have sequential-thinking and fetch (if uvx available)
			if tc.uvx {
				if _, ok := cfg.MCPServers["fetch"]; !ok {
					t.Fatalf("expected fetch MCP server when uvx available")
				}
			}

			// Should NOT have comment_updater (missing required context)
			if _, ok := cfg.MCPServers["comment_updater"]; ok {
				t.Fatalf("comment_updater MCP server should not be present")
			}
		})
	}
}

func TestBuildMCPConfig_EmptyContext(t *testing.T) {
	cases := []struct {
		name string
		uvx  bool
	}{
		{name: "uvxAvailable", uvx: true},
		{name: "uvxMissing", uvx: false},
	}

	ctx := map[string]string{}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setUVXAvailability(t, tc.uvx)

			raw, err := buildMCPConfig(ctx)
			if err != nil {
				t.Fatalf("buildMCPConfig error: %v", err)
			}

			cfg := decodeMCPConfig(t, raw)

			// When uvx available, should have fetch
			if tc.uvx {
				if _, ok := cfg.MCPServers["fetch"]; !ok {
					t.Fatalf("expected fetch MCP server when uvx available: %+v", cfg.MCPServers)
				}
			}

			// Should NOT have comment_updater (empty context)
			if _, ok := cfg.MCPServers["comment_updater"]; ok {
				t.Fatalf("unexpected comment_updater MCP server with empty context")
			}
		})
	}
}

func TestBuildMCPConfig_PartialCommentContext(t *testing.T) {
	cases := []struct {
		name string
		uvx  bool
	}{
		{name: "uvxAvailable", uvx: true},
		{name: "uvxMissing", uvx: false},
	}

	ctx := map[string]string{
		"github_token": "ghs_partial",
		"comment_id":   "42",
		"repo_name":    "missing-owner",
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setUVXAvailability(t, tc.uvx)

			raw, err := buildMCPConfig(ctx)
			if err != nil {
				t.Fatalf("buildMCPConfig error: %v", err)
			}

			cfg := decodeMCPConfig(t, raw)

			// Should NOT have comment_updater (partial context, missing repo_owner)
			if _, ok := cfg.MCPServers["comment_updater"]; ok {
				t.Fatalf("comment_updater should not be present with partial context")
			}

			// Should have fetch if uvx available
			if tc.uvx {
				if _, ok := cfg.MCPServers["fetch"]; !ok {
					t.Fatalf("expected fetch MCP server when uvx available")
				}
			}
		})
	}
}

func TestBuildMCPConfig_JSONFormat(t *testing.T) {
	cases := []struct {
		name string
		ctx  map[string]string
		uvx  bool
	}{
		{
			name: "fullContext",
			ctx: map[string]string{
				"github_token": "ghs_json",
				"comment_id":   "999",
				"repo_owner":   "owner",
				"repo_name":    "repo",
			},
			uvx: true,
		},
		{
			name: "emptyContext",
			ctx:  map[string]string{},
			uvx:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setUVXAvailability(t, tc.uvx)

			raw, err := buildMCPConfig(tc.ctx)
			if err != nil {
				t.Fatalf("buildMCPConfig error: %v", err)
			}

			if !json.Valid([]byte(raw)) {
				t.Fatalf("returned config is not valid JSON: %s", raw)
			}

			var parsed map[string]any
			if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
				t.Fatalf("failed to parse JSON: %v", err)
			}

			value, ok := parsed["mcpServers"]
			if !ok {
				t.Fatalf("mcpServers field missing: %v", parsed)
			}
			if _, ok := value.(map[string]any); !ok {
				t.Fatalf("mcpServers field has unexpected type: %T", value)
			}
		})
	}
}
