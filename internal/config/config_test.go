package config

import (
	"os"
	"testing"
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
				if cfg.TriggerKeyword != "/test" {
					t.Errorf("TriggerKeyword = %s, want /test", cfg.TriggerKeyword)
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
				if cfg.TriggerKeyword != "/pilot" {
					t.Errorf("TriggerKeyword = %s, want /pilot (default)", cfg.TriggerKeyword)
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
			},
			wantErr: true,
			errMsg:  "ANTHROPIC_API_KEY is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
