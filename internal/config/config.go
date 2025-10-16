package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cexll/swe/internal/provider"
	"github.com/cexll/swe/internal/provider/claude"
	"github.com/cexll/swe/internal/provider/codex"
)

// Config holds all configuration for the swe-agent service
type Config struct {
	// Server settings
	Port int

	// GitHub App settings
	GitHubAppID         string
	GitHubPrivateKey    string
	GitHubWebhookSecret string

	// AI Provider selection
	Provider string // "claude" or "codex"

	// Claude settings
	ClaudeAPIKey string
	ClaudeModel  string

	// Codex settings (uses OpenAI-compatible environment variables)
	OpenAIAPIKey  string
	OpenAIBaseURL string // Optional: custom API endpoint
	CodexModel    string

	// Trigger settings
	TriggerKeyword string

	// Security settings
	DisallowedTools string

	// Tooling/MCP toggles
	EnableGitHubCommentMCP bool
	EnableGitHubFileOpsMCP bool
	EnableGitHubCIMCP      bool
	UseCommitSigning       bool

	// Dispatcher settings
	DispatcherWorkers           int
	DispatcherQueueSize         int
	DispatcherMaxAttempts       int
	DispatcherRetryInitial      time.Duration
	DispatcherRetryMax          time.Duration
	DispatcherBackoffMultiplier float64
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	privateKey := normalizePrivateKey(os.Getenv("GITHUB_PRIVATE_KEY"))

	cfg := &Config{
		Port:                        getEnvInt("PORT", 8000),
		GitHubAppID:                 os.Getenv("GITHUB_APP_ID"),
		GitHubPrivateKey:            privateKey,
		GitHubWebhookSecret:         os.Getenv("GITHUB_WEBHOOK_SECRET"),
		Provider:                    getEnv("PROVIDER", "claude"),
		ClaudeAPIKey:                os.Getenv("ANTHROPIC_API_KEY"),
		ClaudeModel:                 getEnv("CLAUDE_MODEL", "claude-sonnet-4-5-20250929"),
		OpenAIAPIKey:                os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL:               os.Getenv("OPENAI_BASE_URL"),
		CodexModel:                  getEnv("CODEX_MODEL", "gpt-5-codex"),
		TriggerKeyword:              getEnv("TRIGGER_KEYWORD", "/code"),
		DisallowedTools:             getEnv("DISALLOWED_TOOLS", ""),
    EnableGitHubCommentMCP:      getEnvBool("ENABLE_GITHUB_MCP_COMMENT"),
    EnableGitHubFileOpsMCP:      getEnvBool("ENABLE_GITHUB_MCP_FILES"),
    EnableGitHubCIMCP:           getEnvBool("ENABLE_GITHUB_MCP_CI"),
    UseCommitSigning:            getEnvBool("USE_COMMIT_SIGNING"),
		DispatcherWorkers:           getEnvInt("DISPATCHER_WORKERS", 4),
		DispatcherQueueSize:         getEnvInt("DISPATCHER_QUEUE_SIZE", 16),
		DispatcherMaxAttempts:       getEnvInt("DISPATCHER_MAX_ATTEMPTS", 3),
		DispatcherRetryInitial:      time.Duration(getEnvInt("DISPATCHER_RETRY_SECONDS", 15)) * time.Second,
		DispatcherRetryMax:          time.Duration(getEnvInt("DISPATCHER_RETRY_MAX_SECONDS", 300)) * time.Second,
		DispatcherBackoffMultiplier: getEnvFloat("DISPATCHER_BACKOFF_MULTIPLIER", 2.0),
	}

	// Validate required fields
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func normalizePrivateKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"") {
		trimmed = strings.TrimPrefix(trimmed, "\"")
		trimmed = strings.TrimSuffix(trimmed, "\"")
	}
	if strings.HasPrefix(trimmed, "'") && strings.HasSuffix(trimmed, "'") {
		trimmed = strings.TrimPrefix(trimmed, "'")
		trimmed = strings.TrimSuffix(trimmed, "'")
	}

	trimmed = strings.ReplaceAll(trimmed, "\r\n", "\n")
	trimmed = strings.ReplaceAll(trimmed, "\r", "\n")
	if strings.Contains(trimmed, "\\n") {
		trimmed = strings.ReplaceAll(trimmed, "\\r", "")
		trimmed = strings.ReplaceAll(trimmed, "\\n", "\n")
	}

	return trimmed
}

// validate checks that all required configuration is present
func (c *Config) validate() error {
	if err := c.validateGitHubCredentials(); err != nil {
		return err
	}

	if err := c.validateProviderConfig(); err != nil {
		return err
	}

	c.applyDispatcherDefaults()
	return c.validateDispatcherConfig()
}

func (c *Config) validateGitHubCredentials() error {
	if c.GitHubAppID == "" {
		return fmt.Errorf("GITHUB_APP_ID is required")
	}
	if c.GitHubPrivateKey == "" {
		return fmt.Errorf("GITHUB_PRIVATE_KEY is required")
	}
	if c.GitHubWebhookSecret == "" {
		return fmt.Errorf("GITHUB_WEBHOOK_SECRET is required")
	}
	return nil
}

func (c *Config) validateProviderConfig() error {
	switch c.Provider {
	case "claude":
		if c.ClaudeAPIKey == "" {
			return fmt.Errorf("ANTHROPIC_API_KEY is required for claude provider")
		}
	case "codex":
		if c.OpenAIAPIKey == "" {
			log.Printf("Warning: OPENAI_API_KEY not set, using default OpenAI credentials")
		}
	default:
		return fmt.Errorf("invalid provider: %s (must be 'claude' or 'codex')", c.Provider)
	}
	return nil
}

func (c *Config) applyDispatcherDefaults() {
	if c.DispatcherWorkers <= 0 {
		c.DispatcherWorkers = 4
	}
	if c.DispatcherQueueSize <= 0 {
		c.DispatcherQueueSize = 16
	}
	if c.DispatcherMaxAttempts <= 0 {
		c.DispatcherMaxAttempts = 3
	}
	if c.DispatcherRetryInitial <= 0 {
		c.DispatcherRetryInitial = 15 * time.Second
	}
	if c.DispatcherRetryMax <= 0 {
		c.DispatcherRetryMax = 5 * time.Minute
	}
	if c.DispatcherBackoffMultiplier < 1 {
		c.DispatcherBackoffMultiplier = 2
	}
}

func (c *Config) validateDispatcherConfig() error {
	if c.DispatcherWorkers <= 0 {
		return fmt.Errorf("DISPATCHER_WORKERS must be greater than 0")
	}
	if c.DispatcherQueueSize <= 0 {
		return fmt.Errorf("DISPATCHER_QUEUE_SIZE must be greater than 0")
	}
	if c.DispatcherMaxAttempts <= 0 {
		return fmt.Errorf("DISPATCHER_MAX_ATTEMPTS must be greater than 0")
	}
	if c.DispatcherRetryInitial <= 0 {
		return fmt.Errorf("DISPATCHER_RETRY_SECONDS must be greater than 0")
	}
	if c.DispatcherRetryMax < c.DispatcherRetryInitial {
		return fmt.Errorf("DISPATCHER_RETRY_MAX_SECONDS must be >= DISPATCHER_RETRY_SECONDS")
	}
	if c.DispatcherBackoffMultiplier < 1 {
		return fmt.Errorf("DISPATCHER_BACKOFF_MULTIPLIER must be >= 1")
	}
	return nil
}

// getEnv gets environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets environment variable as int with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func getEnvBool(key string) bool {
    v := os.Getenv(key)
    if v == "" {
        return false
    }
    switch v {
    case "1", "true", "TRUE", "True", "yes", "Y", "y":
        return true
    case "0", "false", "FALSE", "False", "no", "N", "n":
        return false
    default:
        return false
    }
}

// NewProvider creates a provider based on configuration
// This factory function eliminates if-else branches and avoids circular dependencies
func (c *Config) NewProvider() (provider.Provider, error) {
	switch c.Provider {
	case "claude":
		if c.ClaudeAPIKey == "" {
			return nil, fmt.Errorf("claude: ANTHROPIC_API_KEY is required")
		}
		model := c.ClaudeModel
		if model == "" {
			model = "claude-sonnet-4-5-20250929"
		}
		return claude.NewProvider(c.ClaudeAPIKey, model), nil

	case "codex":
		model := c.CodexModel
		if model == "" {
			model = "gpt-5-codex"
		}
		return codex.NewProvider(c.OpenAIAPIKey, c.OpenAIBaseURL, model), nil

	default:
		return nil, fmt.Errorf("unknown provider: %s (supported: claude, codex)", c.Provider)
	}
}
