package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cexll/swe/internal/github"
	prov "github.com/cexll/swe/internal/provider"
	"github.com/cexll/swe/internal/webhook"
)

func TestExecutorAdapter_New(t *testing.T) {
	provider := &mockProvider{}
	auth := &mockAuthProvider{}
	executor := New(provider, auth)
	adapter := NewAdapter(executor)

	if adapter == nil {
		t.Fatal("NewAdapter() returned nil")
	}
	if adapter.inner == nil {
		t.Error("adapter.inner is nil")
	}
}

func TestExecutorAdapter_Execute(t *testing.T) {
	tests := []struct {
		name        string
		eventType   string
		payload     string
		wantErr     bool
		errContains string
	}{
		{
			name:        "invalid JSON payload",
			eventType:   "issue_comment",
			payload:     `{invalid json`,
			wantErr:     true,
			errContains: "failed to parse",
		},
		{
			name:        "unsupported event type",
			eventType:   "unsupported_event",
			payload:     `{}`,
			wantErr:     true,
			errContains: "unsupported event type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock provider that returns success
			provider := &mockProvider{
				generateFunc: func(ctx context.Context, req *prov.CodeRequest) (*prov.CodeResponse, error) {
					return &prov.CodeResponse{Summary: "Test completed"}, nil
				},
			}
			auth := &mockAuthProvider{}
			executor := New(provider, auth)
			adapter := NewAdapter(executor)

			// Create webhook task with raw payload
			task := &webhook.Task{
				ID:         "test-task",
				Repo:       "owner/repo",
				Number:     42,
				EventType:  tt.eventType,
				RawPayload: []byte(tt.payload),
			}

			err := adapter.Execute(context.Background(), task)

			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !contains(err.Error(), tt.errContains) {
					t.Errorf("Execute() error = %v, want error containing %q", err, tt.errContains)
				}
			}
		})
	}
}

func TestExecutorAdapter_ParseWebhookEvent(t *testing.T) {
	// Test that the adapter correctly calls github.ParseWebhookEvent
	tests := []struct {
		name      string
		eventType string
		payload   map[string]interface{}
	}{
		{
			name:      "issue_comment parses correctly",
			eventType: "issue_comment",
			payload: map[string]interface{}{
				"action": "created",
				"issue": map[string]interface{}{
					"number": 42,
				},
				"comment": map[string]interface{}{
					"id":   float64(123),
					"body": "test comment",
					"user": map[string]interface{}{"login": "testuser"},
				},
				"repository": map[string]interface{}{
					"full_name": "owner/repo",
					"owner":     map[string]interface{}{"login": "owner"},
					"name":      "repo",
				},
				"sender": map[string]interface{}{"login": "testuser"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := json.Marshal(tt.payload)
			if err != nil {
				t.Fatalf("Failed to marshal payload: %v", err)
			}

			// Verify github.ParseWebhookEvent can parse the payload
			ctx, err := github.ParseWebhookEvent(tt.eventType, payload)
			if err != nil {
				t.Errorf("ParseWebhookEvent() error = %v", err)
				return
			}

			if ctx == nil {
				t.Error("ParseWebhookEvent() returned nil context")
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
