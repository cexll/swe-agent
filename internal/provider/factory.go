package provider

import (
	"fmt"

	"github.com/cexll/swe/internal/provider/claude"
)

// Config contains provider configuration
type Config struct {
	// Provider name: "claude", "codex", "gemini", "amp"
	Name string

	// Claude configuration
	ClaudeAPIKey string
	ClaudeModel  string

	// Future: Codex configuration
	// CodexAPIKey string
	// CodexModel  string

	// Future: Gemini configuration
	// GeminiAPIKey string
	// GeminiModel  string
}

// NewProvider creates a provider based on configuration
// This is a factory function that eliminates if-else branches
func NewProvider(cfg *Config) (Provider, error) {
	switch cfg.Name {
	case "claude":
		if cfg.ClaudeAPIKey == "" {
			return nil, fmt.Errorf("claude: ANTHROPIC_API_KEY is required")
		}
		model := cfg.ClaudeModel
		if model == "" {
			model = "claude-3-5-sonnet-20241022"
		}
		return claude.NewProvider(cfg.ClaudeAPIKey, model), nil

	// Future providers can be added here without modifying existing code
	// case "codex":
	//     return codex.NewProvider(cfg.CodexAPIKey, cfg.CodexModel), nil
	// case "gemini":
	//     return gemini.NewProvider(cfg.GeminiAPIKey, cfg.GeminiModel), nil
	// case "amp":
	//     return amp.NewProvider(cfg.AMPConfig), nil

	default:
		return nil, fmt.Errorf("unknown provider: %s (supported: claude)", cfg.Name)
	}
}
