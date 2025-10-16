package config

import (
	"testing"
)

type namedProvider interface{ Name() string }

func TestNewProvider_Claude(t *testing.T) {
	cfg := &Config{Provider: "claude", ClaudeAPIKey: "k", ClaudeModel: ""}
	p, err := cfg.NewProvider()
	if err != nil {
		t.Fatalf("NewProvider error: %v", err)
	}
	if np, ok := p.(namedProvider); !ok || np.Name() != "claude" {
		t.Fatalf("expected claude provider, got %T", p)
	}
}

func TestNewProvider_Claude_NoKey(t *testing.T) {
	cfg := &Config{Provider: "claude", ClaudeAPIKey: ""}
	if _, err := cfg.NewProvider(); err == nil {
		t.Fatalf("expected error for missing ANTHROPIC_API_KEY")
	}
}

func TestNewProvider_Codex(t *testing.T) {
	cfg := &Config{Provider: "codex", OpenAIAPIKey: "x", CodexModel: ""}
	p, err := cfg.NewProvider()
	if err != nil {
		t.Fatalf("NewProvider error: %v", err)
	}
	if np, ok := p.(namedProvider); !ok || np.Name() != "codex" {
		t.Fatalf("expected codex provider, got %T", p)
	}
}

func TestNewProvider_Unknown(t *testing.T) {
	cfg := &Config{Provider: "foo"}
	if _, err := cfg.NewProvider(); err == nil {
		t.Fatalf("expected error for unknown provider")
	}
}
