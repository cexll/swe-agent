package claude

import (
	"os"
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
