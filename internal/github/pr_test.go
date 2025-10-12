package github

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreatePR_ErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		workdir string
		repo    string
		head    string
		base    string
		title   string
		body    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "invalid workdir",
			workdir: "/nonexistent/directory",
			repo:    "owner/repo",
			head:    "feature",
			base:    "main",
			title:   "Test PR",
			body:    "Description",
			wantErr: true,
			errMsg:  "create",
		},
		{
			name:    "empty repo",
			workdir: tmpDir,
			repo:    "",
			head:    "feature",
			base:    "main",
			title:   "Test PR",
			body:    "Description",
			wantErr: true,
			errMsg:  "create",
		},
		{
			name:    "empty head branch",
			workdir: tmpDir,
			repo:    "owner/repo",
			head:    "",
			base:    "main",
			title:   "Test PR",
			body:    "Description",
			wantErr: true,
			errMsg:  "create",
		},
		{
			name:    "empty base branch",
			workdir: tmpDir,
			repo:    "owner/repo",
			head:    "feature",
			base:    "",
			title:   "Test PR",
			body:    "Description",
			wantErr: true,
			errMsg:  "create",
		},
		{
			name:    "empty title",
			workdir: tmpDir,
			repo:    "owner/repo",
			head:    "feature",
			base:    "main",
			title:   "",
			body:    "Description",
			wantErr: true,
			errMsg:  "create",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prURL, err := CreatePR(tt.workdir, tt.repo, tt.head, tt.base, tt.title, tt.body)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreatePR() should return error for %s", tt.name)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("CreatePR() error = %v, want error containing %q", err, tt.errMsg)
				}
				if prURL != "" {
					t.Errorf("CreatePR() should return empty URL on error, got %s", prURL)
				}
			} else {
				if err != nil {
					t.Errorf("CreatePR() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCreatePR_GitSetup(t *testing.T) {
	// Test that function signature is correct
	tmpDir := t.TempDir()

	// Create minimal git repo structure
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	// This will fail because we don't have proper git setup,
	// but it tests parameter validation
	_, err := CreatePR(tmpDir, "owner/repo", "feature", "main", "Test Title", "Test Body")

	// Should get an error from gh CLI
	if err == nil {
		t.Error("CreatePR() should fail without proper git setup")
	}

	// Error should mention PR creation
	if !strings.Contains(err.Error(), "pr") && !strings.Contains(err.Error(), "create") {
		t.Errorf("Error should mention PR creation, got: %v", err)
	}
}

func TestCreatePR_OutputFormatting(t *testing.T) {
	// Test that CreatePR properly handles output
	// We can't test actual PR creation without GitHub access,
	// but we can test error output formatting

	tmpDir := t.TempDir()

	prURL, err := CreatePR(tmpDir, "nonexistent/repo", "branch1", "branch2", "Title", "Body")

	if err == nil {
		t.Error("CreatePR() should fail for nonexistent repo")
		return
	}

	// Verify error format
	errMsg := err.Error()
	if !strings.Contains(errMsg, "gh pr create failed") {
		t.Errorf("Error should contain 'gh pr create failed', got: %s", errMsg)
	}

	// URL should be empty on error
	if prURL != "" {
		t.Errorf("CreatePR() prURL = %s, want empty string on error", prURL)
	}
}

func TestCreatePR_SpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name  string
		title string
		body  string
	}{
		{
			name:  "title with quotes",
			title: `Title with "quotes"`,
			body:  "Normal body",
		},
		{
			name:  "body with newlines",
			title: "Normal title",
			body:  "Body with\nmultiple\nlines",
		},
		{
			name:  "special characters",
			title: "Fix bug #123",
			body:  "Fixed issue & added feature",
		},
		{
			name:  "unicode characters",
			title: "修复 Bug 🐛",
			body:  "Description with émojis 🎉",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Will fail due to no git setup, but tests parameter passing
			_, err := CreatePR(tmpDir, "owner/repo", "head", "base", tt.title, tt.body)

			if err == nil {
				t.Error("CreatePR() should fail without git setup")
				return
			}

			// Should get pr create error, not parameter parsing error
			if strings.Contains(err.Error(), "invalid character") {
				t.Errorf("CreatePR() should handle special characters, got: %v", err)
			}
		})
	}
}

func TestCreatePR_LongTitle(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with very long title
	longTitle := strings.Repeat("Very long title ", 50) // ~800 characters

	_, err := CreatePR(tmpDir, "owner/repo", "head", "base", longTitle, "body")

	if err == nil {
		t.Error("CreatePR() should fail without git setup")
		return
	}

	// Should fail at gh pr create, not at parameter validation
	if !strings.Contains(err.Error(), "pr") && !strings.Contains(err.Error(), "create") {
		t.Errorf("Error should mention PR creation, got: %v", err)
	}
}

func TestCreatePR_EmptyBody(t *testing.T) {
	tmpDir := t.TempDir()

	// Empty body should be allowed (some PRs don't need descriptions)
	_, err := CreatePR(tmpDir, "owner/repo", "head", "base", "Title", "")

	if err == nil {
		t.Error("CreatePR() should fail without git setup")
		return
	}

	// Should fail at gh CLI execution, not parameter validation
	errMsg := err.Error()
	if strings.Contains(errMsg, "body") && strings.Contains(errMsg, "required") {
		t.Error("CreatePR() should allow empty body")
	}
}
