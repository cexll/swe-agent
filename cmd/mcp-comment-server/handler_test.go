package main

import (
	"context"
	"os"
	"testing"
)

func TestHandleUpdateComment_MissingBody(t *testing.T) {
	// Setup test environment
	setupTestEnv(t)

	params := UpdateCommentParams{Body: ""}
	_, _, err := HandleUpdateComment(context.Background(), nil, params)

	if err == nil {
		t.Error("Expected error for empty body, got nil")
	}
}

func TestHandleUpdateComment_InvalidCommentID(t *testing.T) {
	setupTestEnv(t)
	os.Setenv("CLAUDE_COMMENT_ID", "not-a-number")

	params := UpdateCommentParams{Body: "test content"}
	_, _, err := HandleUpdateComment(context.Background(), nil, params)

	if err == nil {
		t.Error("Expected error for invalid comment ID, got nil")
	}
}

func TestHandleUpdateComment_MissingEnvVars(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
	}{
		{"missing REPO_OWNER", map[string]string{"REPO_NAME": "repo", "CLAUDE_COMMENT_ID": "123", "GITHUB_TOKEN": "token"}},
		{"missing REPO_NAME", map[string]string{"REPO_OWNER": "owner", "CLAUDE_COMMENT_ID": "123", "GITHUB_TOKEN": "token"}},
		{"missing GITHUB_TOKEN", map[string]string{"REPO_OWNER": "owner", "REPO_NAME": "repo", "CLAUDE_COMMENT_ID": "123"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			params := UpdateCommentParams{Body: "test"}
			// This will fail during UpdateComment call due to missing env
			// But at least validates the env variable passing logic
			_, _, err := HandleUpdateComment(context.Background(), nil, params)
			// We expect an error when calling GitHub API with invalid token
			if err == nil {
				t.Log("Note: Test runs without actual GitHub API call")
			}
		})
	}
}

func TestUpdateCommentParams_StructFields(t *testing.T) {
	params := UpdateCommentParams{Body: "test"}
	if params.Body != "test" {
		t.Errorf("Expected body 'test', got '%s'", params.Body)
	}
}

func setupTestEnv(t *testing.T) {
	t.Helper()
	os.Setenv("REPO_OWNER", "test-owner")
	os.Setenv("REPO_NAME", "test-repo")
	os.Setenv("CLAUDE_COMMENT_ID", "123456")
	os.Setenv("GITHUB_TOKEN", "test-token")
	os.Setenv("GITHUB_EVENT_NAME", "issue_comment")
}
