package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the pilot-swe service
type Config struct {
	// Server settings
	Port int

	// GitHub App settings
	GitHubAppID         string
	GitHubPrivateKey    string
	GitHubWebhookSecret string

	// Claude settings
	ClaudeAPIKey string
	ClaudeModel  string

	// Trigger settings
	TriggerKeyword string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Port:                getEnvInt("PORT", 3000),
		GitHubAppID:         os.Getenv("GITHUB_APP_ID"),
		GitHubPrivateKey:    os.Getenv("GITHUB_PRIVATE_KEY"),
		GitHubWebhookSecret: os.Getenv("GITHUB_WEBHOOK_SECRET"),
		ClaudeAPIKey:        os.Getenv("ANTHROPIC_API_KEY"),
		ClaudeModel:         getEnv("CLAUDE_MODEL", "claude-3-5-sonnet-20241022"),
		TriggerKeyword:      getEnv("TRIGGER_KEYWORD", "/pilot"),
	}

	// Validate required fields
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validate checks that all required configuration is present
func (c *Config) validate() error {
	if c.GitHubAppID == "" {
		return fmt.Errorf("GITHUB_APP_ID is required")
	}
	if c.GitHubPrivateKey == "" {
		return fmt.Errorf("GITHUB_PRIVATE_KEY is required")
	}
	if c.GitHubWebhookSecret == "" {
		return fmt.Errorf("GITHUB_WEBHOOK_SECRET is required")
	}
	if c.ClaudeAPIKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY is required")
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
