package provider

import (
	"testing"
)

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		wantErr     bool
		errContains string
		checkName   string
	}{
		{
			name: "claude provider with all fields",
			cfg: &Config{
				Name:         "claude",
				ClaudeAPIKey: "sk-ant-test-key",
				ClaudeModel:  "claude-3-opus-20240229",
			},
			wantErr:   false,
			checkName: "claude",
		},
		{
			name: "claude provider with default model",
			cfg: &Config{
				Name:         "claude",
				ClaudeAPIKey: "sk-ant-test-key",
			},
			wantErr:   false,
			checkName: "claude",
		},
		{
			name: "claude provider missing API key",
			cfg: &Config{
				Name:        "claude",
				ClaudeModel: "claude-3-opus-20240229",
			},
			wantErr:     true,
			errContains: "ANTHROPIC_API_KEY is required",
		},
		{
			name: "unknown provider",
			cfg: &Config{
				Name: "unknown",
			},
			wantErr:     true,
			errContains: "unknown provider: unknown (supported: claude, codex)",
		},
		{
			name: "empty provider name",
			cfg: &Config{
				Name: "",
			},
			wantErr:     true,
			errContains: "unknown provider",
		},
		{
			name: "codex provider default model",
			cfg: &Config{
				Name: "codex",
			},
			wantErr:   false,
			checkName: "codex",
		},
		{
			name: "gemini provider (not yet implemented)",
			cfg: &Config{
				Name: "gemini",
			},
			wantErr:     true,
			errContains: "unknown provider: gemini (supported: claude, codex)",
		},
		{
			name: "amp provider (not yet implemented)",
			cfg: &Config{
				Name: "amp",
			},
			wantErr:     true,
			errContains: "unknown provider: amp (supported: claude, codex)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProvider(tt.cfg)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check error message
			if tt.wantErr && err != nil {
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("NewProvider() error = %v, want to contain %v", err.Error(), tt.errContains)
				}
			}

			// Check provider name
			if !tt.wantErr && provider != nil {
				if provider.Name() != tt.checkName {
					t.Errorf("Provider.Name() = %v, want %v", provider.Name(), tt.checkName)
				}
			}

			// Ensure provider is nil when error occurs
			if tt.wantErr && provider != nil {
				t.Errorf("NewProvider() provider = %v, want nil when error occurs", provider)
			}
		})
	}
}

func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want bool
	}{
		{
			name: "valid claude config",
			cfg: &Config{
				Name:         "claude",
				ClaudeAPIKey: "sk-ant-test",
				ClaudeModel:  "claude-3-5-sonnet-20241022",
			},
			want: true,
		},
		{
			name: "empty API key",
			cfg: &Config{
				Name:        "claude",
				ClaudeModel: "claude-3-5-sonnet-20241022",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProvider(tt.cfg)
			if tt.want && err != nil {
				t.Errorf("NewProvider() should succeed but got error: %v", err)
			}
			if !tt.want && err == nil {
				t.Errorf("NewProvider() should fail but succeeded")
			}
		})
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
