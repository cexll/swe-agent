package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr bool
		check   func(*testing.T, *Config)
	}{
		{
			name: "all required fields present",
			env: map[string]string{
				"GITHUB_APP_ID":         "123456",
				"GITHUB_PRIVATE_KEY":    "test-private-key",
				"GITHUB_WEBHOOK_SECRET": "test-webhook-secret",
				"ANTHROPIC_API_KEY":     "sk-ant-test",
				"PORT":                  "8080",
				"CLAUDE_MODEL":          "claude-3-opus-20240229",
				"OPENAI_API_KEY":        "sk-openai-test",
				"OPENAI_BASE_URL":       "https://api.example.com/v1",
				"CODEX_MODEL":           "gpt-5-codex-plus",
				"TRIGGER_KEYWORD":       "/test",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				if cfg.Port != 8080 {
					t.Errorf("Port = %d, want 8080", cfg.Port)
				}
				if cfg.GitHubAppID != "123456" {
					t.Errorf("GitHubAppID = %s, want 123456", cfg.GitHubAppID)
				}
				if cfg.ClaudeModel != "claude-3-opus-20240229" {
					t.Errorf("ClaudeModel = %s, want claude-3-opus-20240229", cfg.ClaudeModel)
				}
				if cfg.OpenAIAPIKey != "sk-openai-test" {
					t.Errorf("OpenAIAPIKey = %s, want sk-openai-test", cfg.OpenAIAPIKey)
				}
				if cfg.OpenAIBaseURL != "https://api.example.com/v1" {
					t.Errorf("OpenAIBaseURL = %s, want https://api.example.com/v1", cfg.OpenAIBaseURL)
				}
				if cfg.CodexModel != "gpt-5-codex-plus" {
					t.Errorf("CodexModel = %s, want gpt-5-codex-plus", cfg.CodexModel)
				}
				if cfg.TriggerKeyword != "/test" {
					t.Errorf("TriggerKeyword = %s, want /test", cfg.TriggerKeyword)
				}
				if cfg.DispatcherWorkers != 4 {
					t.Errorf("DispatcherWorkers = %d, want 4", cfg.DispatcherWorkers)
				}
				if cfg.DispatcherQueueSize != 16 {
					t.Errorf("DispatcherQueueSize = %d, want 16", cfg.DispatcherQueueSize)
				}
				if cfg.DispatcherMaxAttempts != 3 {
					t.Errorf("DispatcherMaxAttempts = %d, want 3", cfg.DispatcherMaxAttempts)
				}
				if cfg.DispatcherRetryInitial != 15*time.Second {
					t.Errorf("DispatcherRetryInitial = %s, want 15s", cfg.DispatcherRetryInitial)
				}
				if cfg.DispatcherRetryMax != 300*time.Second {
					t.Errorf("DispatcherRetryMax = %s, want 5m", cfg.DispatcherRetryMax)
				}
				if cfg.DispatcherBackoffMultiplier != 2 {
					t.Errorf("DispatcherBackoffMultiplier = %f, want 2", cfg.DispatcherBackoffMultiplier)
				}
			},
		},
		{
			name: "use default port and model",
			env: map[string]string{
				"GITHUB_APP_ID":         "123456",
				"GITHUB_PRIVATE_KEY":    "test-private-key",
				"GITHUB_WEBHOOK_SECRET": "test-webhook-secret",
				"ANTHROPIC_API_KEY":     "sk-ant-test",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				if cfg.Port != 3000 {
					t.Errorf("Port = %d, want 3000 (default)", cfg.Port)
				}
				if cfg.ClaudeModel != "claude-3-5-sonnet-20241022" {
					t.Errorf("ClaudeModel = %s, want default", cfg.ClaudeModel)
				}
				if cfg.CodexModel != "gpt-5-codex" {
					t.Errorf("CodexModel = %s, want gpt-5-codex (default)", cfg.CodexModel)
				}
				if cfg.OpenAIAPIKey != "" {
					t.Errorf("OpenAIAPIKey = %s, want empty default", cfg.OpenAIAPIKey)
				}
				if cfg.OpenAIBaseURL != "" {
					t.Errorf("OpenAIBaseURL = %s, want empty default", cfg.OpenAIBaseURL)
				}
				if cfg.TriggerKeyword != "/code" {
					t.Errorf("TriggerKeyword = %s, want /code (default)", cfg.TriggerKeyword)
				}
				if cfg.DispatcherWorkers != 4 {
					t.Errorf("DispatcherWorkers = %d, want 4", cfg.DispatcherWorkers)
				}
				if cfg.DispatcherQueueSize != 16 {
					t.Errorf("DispatcherQueueSize = %d, want 16", cfg.DispatcherQueueSize)
				}
				if cfg.DispatcherMaxAttempts != 3 {
					t.Errorf("DispatcherMaxAttempts = %d, want 3", cfg.DispatcherMaxAttempts)
				}
				if cfg.DispatcherRetryInitial != 15*time.Second {
					t.Errorf("DispatcherRetryInitial = %s, want 15s", cfg.DispatcherRetryInitial)
				}
				if cfg.DispatcherRetryMax != 300*time.Second {
					t.Errorf("DispatcherRetryMax = %s, want 5m", cfg.DispatcherRetryMax)
				}
				if cfg.DispatcherBackoffMultiplier != 2 {
					t.Errorf("DispatcherBackoffMultiplier = %f, want 2", cfg.DispatcherBackoffMultiplier)
				}
			},
		},
		{
			name: "missing GITHUB_APP_ID",
			env: map[string]string{
				"GITHUB_PRIVATE_KEY":    "test-private-key",
				"GITHUB_WEBHOOK_SECRET": "test-webhook-secret",
				"ANTHROPIC_API_KEY":     "sk-ant-test",
			},
			wantErr: true,
		},
		{
			name: "missing GITHUB_PRIVATE_KEY",
			env: map[string]string{
				"GITHUB_APP_ID":         "123456",
				"GITHUB_WEBHOOK_SECRET": "test-webhook-secret",
				"ANTHROPIC_API_KEY":     "sk-ant-test",
			},
			wantErr: true,
		},
		{
			name: "missing GITHUB_WEBHOOK_SECRET",
			env: map[string]string{
				"GITHUB_APP_ID":      "123456",
				"GITHUB_PRIVATE_KEY": "test-private-key",
				"ANTHROPIC_API_KEY":  "sk-ant-test",
			},
			wantErr: true,
		},
		{
			name: "missing ANTHROPIC_API_KEY",
			env: map[string]string{
				"GITHUB_APP_ID":         "123456",
				"GITHUB_PRIVATE_KEY":    "test-private-key",
				"GITHUB_WEBHOOK_SECRET": "test-webhook-secret",
			},
			wantErr: true,
		},
		{
			name: "invalid port number",
			env: map[string]string{
				"GITHUB_APP_ID":         "123456",
				"GITHUB_PRIVATE_KEY":    "test-private-key",
				"GITHUB_WEBHOOK_SECRET": "test-webhook-secret",
				"ANTHROPIC_API_KEY":     "sk-ant-test",
				"PORT":                  "invalid",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				// Invalid port should fall back to default
				if cfg.Port != 3000 {
					t.Errorf("Port = %d, want 3000 (default for invalid)", cfg.Port)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all environment variables
			os.Clearenv()

			// Set test environment variables
			for k, v := range tt.env {
				os.Setenv(k, v)
			}

			// Test Load
			cfg, err := Load()

			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestConfigValidateDefaultsApplied(t *testing.T) {
	cfg := &Config{
		GitHubAppID:                 "app",
		GitHubPrivateKey:            "key",
		GitHubWebhookSecret:         "secret",
		Provider:                    "claude",
		ClaudeAPIKey:                "api",
		DispatcherWorkers:           0,
		DispatcherQueueSize:         0,
		DispatcherMaxAttempts:       0,
		DispatcherRetryInitial:      0,
		DispatcherRetryMax:          0,
		DispatcherBackoffMultiplier: 0.5,
	}

	if err := cfg.validate(); err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	if cfg.DispatcherWorkers != 4 {
		t.Fatalf("DispatcherWorkers default = %d, want 4", cfg.DispatcherWorkers)
	}
	if cfg.DispatcherQueueSize != 16 {
		t.Fatalf("DispatcherQueueSize default = %d, want 16", cfg.DispatcherQueueSize)
	}
	if cfg.DispatcherRetryInitial != 15*time.Second {
		t.Fatalf("DispatcherRetryInitial default = %s, want 15s", cfg.DispatcherRetryInitial)
	}
	if cfg.DispatcherRetryMax != 5*time.Minute {
		t.Fatalf("DispatcherRetryMax default = %s, want 5m", cfg.DispatcherRetryMax)
	}
	if cfg.DispatcherBackoffMultiplier != 2 {
		t.Fatalf("DispatcherBackoffMultiplier default = %f, want 2", cfg.DispatcherBackoffMultiplier)
	}
}

func TestConfigValidateRetryWindow(t *testing.T) {
	cfg := &Config{
		GitHubAppID:                 "app",
		GitHubPrivateKey:            "key",
		GitHubWebhookSecret:         "secret",
		Provider:                    "claude",
		ClaudeAPIKey:                "api",
		DispatcherWorkers:           2,
		DispatcherQueueSize:         4,
		DispatcherMaxAttempts:       2,
		DispatcherRetryInitial:      10 * time.Second,
		DispatcherRetryMax:          5 * time.Second,
		DispatcherBackoffMultiplier: 2,
	}

	err := cfg.validate()
	if err == nil || !strings.Contains(err.Error(), "DISPATCHER_RETRY_MAX_SECONDS") {
		t.Fatalf("expected retry window error, got %v", err)
	}
}

func TestGetEnvFloat(t *testing.T) {
	t.Setenv("TEST_FLOAT", "3.14")
	if got := getEnvFloat("TEST_FLOAT", 1.0); got != 3.14 {
		t.Fatalf("getEnvFloat parsed %v, want 3.14", got)
	}

	t.Setenv("TEST_FLOAT", "invalid")
	if got := getEnvFloat("TEST_FLOAT", 1.5); got != 1.5 {
		t.Fatalf("getEnvFloat fallback %v, want 1.5", got)
	}
}

func applyDispatcherDefaults(cfg *Config) {
	cfg.DispatcherWorkers = 1
	cfg.DispatcherQueueSize = 1
	cfg.DispatcherMaxAttempts = 1
	cfg.DispatcherRetryInitial = time.Second
	cfg.DispatcherRetryMax = time.Second
	cfg.DispatcherBackoffMultiplier = 2
}

func TestConfig_validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: &Config{
				GitHubAppID:         "123456",
				GitHubPrivateKey:    "test-key",
				GitHubWebhookSecret: "test-secret",
				Provider:            "claude",
				ClaudeAPIKey:        "sk-ant-test",
			},
			wantErr: false,
		},
		{
			name: "missing GitHubAppID",
			cfg: &Config{
				GitHubPrivateKey:    "test-key",
				GitHubWebhookSecret: "test-secret",
				ClaudeAPIKey:        "sk-ant-test",
			},
			wantErr: true,
			errMsg:  "GITHUB_APP_ID is required",
		},
		{
			name: "missing GitHubPrivateKey",
			cfg: &Config{
				GitHubAppID:         "123456",
				GitHubWebhookSecret: "test-secret",
				ClaudeAPIKey:        "sk-ant-test",
			},
			wantErr: true,
			errMsg:  "GITHUB_PRIVATE_KEY is required",
		},
		{
			name: "missing GitHubWebhookSecret",
			cfg: &Config{
				GitHubAppID:      "123456",
				GitHubPrivateKey: "test-key",
				ClaudeAPIKey:     "sk-ant-test",
			},
			wantErr: true,
			errMsg:  "GITHUB_WEBHOOK_SECRET is required",
		},
		{
			name: "missing ClaudeAPIKey",
			cfg: &Config{
				GitHubAppID:         "123456",
				GitHubPrivateKey:    "test-key",
				GitHubWebhookSecret: "test-secret",
				Provider:            "claude",
			},
			wantErr: true,
			errMsg:  "ANTHROPIC_API_KEY is required for claude provider",
		},
		{
			name: "valid codex config with OpenAI key",
			cfg: &Config{
				GitHubAppID:         "123456",
				GitHubPrivateKey:    "test-key",
				GitHubWebhookSecret: "test-secret",
				Provider:            "codex",
				OpenAIAPIKey:        "sk-openai-test",
			},
			wantErr: false,
		},
		{
			name: "valid codex config without OpenAI key (warning logged)",
			cfg: &Config{
				GitHubAppID:         "123456",
				GitHubPrivateKey:    "test-key",
				GitHubWebhookSecret: "test-secret",
				Provider:            "codex",
				OpenAIAPIKey:        "", // Empty, should log warning but not fail
			},
			wantErr: false,
		},
		{
			name: "invalid provider",
			cfg: &Config{
				GitHubAppID:         "123456",
				GitHubPrivateKey:    "test-key",
				GitHubWebhookSecret: "test-secret",
				Provider:            "invalid-provider",
			},
			wantErr: true,
			errMsg:  "invalid provider: invalid-provider (must be 'claude' or 'codex')",
		},
		{
			name: "empty provider (should default but validate will catch)",
			cfg: &Config{
				GitHubAppID:         "123456",
				GitHubPrivateKey:    "test-key",
				GitHubWebhookSecret: "test-secret",
				Provider:            "",
			},
			wantErr: true,
			errMsg:  "invalid provider:  (must be 'claude' or 'codex')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyDispatcherDefaults(tt.cfg)
			err := tt.cfg.validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("validate() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		want         string
	}{
		{
			name:         "env var exists",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "actual",
			want:         "actual",
		},
		{
			name:         "env var empty",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "",
			want:         "default",
		},
		{
			name:         "env var not set",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "",
			want:         "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
			}

			got := getEnv(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue int
		envValue     string
		want         int
	}{
		{
			name:         "valid int",
			key:          "TEST_PORT",
			defaultValue: 3000,
			envValue:     "8080",
			want:         8080,
		},
		{
			name:         "invalid int",
			key:          "TEST_PORT",
			defaultValue: 3000,
			envValue:     "invalid",
			want:         3000,
		},
		{
			name:         "empty env var",
			key:          "TEST_PORT",
			defaultValue: 3000,
			envValue:     "",
			want:         3000,
		},
		{
			name:         "zero value",
			key:          "TEST_PORT",
			defaultValue: 3000,
			envValue:     "0",
			want:         0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
			}

			got := getEnvInt(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvInt() = %v, want %v", got, tt.want)
			}
		})
	}
}
