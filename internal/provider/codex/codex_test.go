package codex

import (
	"testing"
)

func TestNewProvider_Name(t *testing.T) {
	provider := NewProvider("", "", "gpt-5-codex")
	if provider.Name() != "codex" {
		t.Fatalf("Name() = %s, want codex", provider.Name())
	}
}

func TestNewProvider_APIKey(t *testing.T) {
	// Test that API key is set in environment
	testKey := "test-api-key"
	testBaseURL := "https://api.example.com"
	provider := NewProvider(testKey, testBaseURL, "gpt-5-codex")

	if provider.apiKey != testKey {
		t.Errorf("apiKey = %s, want %s", provider.apiKey, testKey)
	}

	if provider.baseURL != testBaseURL {
		t.Errorf("baseURL = %s, want %s", provider.baseURL, testBaseURL)
	}

	if provider.model != "gpt-5-codex" {
		t.Errorf("model = %s, want gpt-5-codex", provider.model)
	}
}

func TestListRepoFiles(t *testing.T) {
	// Test list repo files functionality (no need to test CLI execution)
	// This is a unit test for the helper function
	files, err := listRepoFiles(".")
	if err != nil {
		t.Fatalf("listRepoFiles() error = %v", err)
	}

	// Should find at least the test file itself
	found := false
	for _, f := range files {
		if f == "codex_test.go" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("listRepoFiles() should find codex_test.go, got: %v", files)
	}
}
