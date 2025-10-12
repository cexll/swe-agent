package github

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// skipIfNetworkUnavailable skips the test if GitHub CLI is not available or network is unreachable
// or if integration tests are disabled
func skipIfNetworkUnavailable(t *testing.T) {
	// Skip integration tests unless explicitly enabled
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Integration tests disabled, set RUN_INTEGRATION_TESTS=true to enable")
	}
	
	// Check if gh CLI is available
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh CLI not available")
	}
	
	// Quick connectivity check with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "gh", "api", "/user")
	if err := cmd.Run(); err != nil {
		t.Skip("GitHub API not accessible or not authenticated")
	}
}

func TestClone_ErrorHandling(t *testing.T) {
	skipIfNetworkUnavailable(t)
	
	tests := []struct {
		name    string
		repo    string
		branch  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "invalid repo format",
			repo:    "invalid-repo",
			branch:  "main",
			wantErr: true,
			errMsg:  "clone",
		},
		{
			name:    "nonexistent repo",
			repo:    "nonexistent/repo-xyz-123",
			branch:  "main",
			wantErr: true,
			errMsg:  "clone",
		},
		{
			name:    "nonexistent branch",
			repo:    "octocat/Hello-World", // Public repo
			branch:  "nonexistent-branch-xyz",
			wantErr: true,
			errMsg:  "clone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workdir, cleanup, err := Clone(tt.repo, tt.branch)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Clone() should return error for %s", tt.name)
					if cleanup != nil {
						cleanup()
					}
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Clone() error = %v, want error containing %q", err, tt.errMsg)
				}
				// Cleanup should be nil on error
				if cleanup != nil {
					t.Error("Clone() cleanup function should be nil on error")
				}
				return
			}

			if err != nil {
				t.Errorf("Clone() unexpected error: %v", err)
				return
			}

			// Verify workdir exists
			if _, err := os.Stat(workdir); os.IsNotExist(err) {
				t.Errorf("Clone() workdir %s does not exist", workdir)
			}

			// Verify cleanup function works
			if cleanup == nil {
				t.Error("Clone() cleanup function is nil")
			} else {
				cleanup()
				// Verify directory was removed
				if _, err := os.Stat(workdir); !os.IsNotExist(err) {
					t.Errorf("cleanup() did not remove directory %s", workdir)
				}
			}
		})
	}
}

func TestClone_WorkdirFormat(t *testing.T) {
	skipIfNetworkUnavailable(t)
	
	// Test that workdir has expected format
	repo := "octocat/Hello-World"
	branch := "master"

	workdir, cleanup, err := Clone(repo, branch)
	if err != nil {
		// If gh CLI not available, skip test
		if strings.Contains(err.Error(), "executable file not found") {
			t.Skip("gh CLI not available")
		}
		t.Fatalf("Clone() error: %v", err)
	}
	defer cleanup()

	// Verify workdir is in temp directory
	tmpDir := os.TempDir()
	if !strings.HasPrefix(workdir, tmpDir) {
		t.Errorf("Clone() workdir = %s, should be in temp dir %s", workdir, tmpDir)
	}

	// Verify workdir name contains "pilot-"
	basename := filepath.Base(workdir)
	if !strings.HasPrefix(basename, "pilot-") {
		t.Errorf("Clone() workdir basename = %s, should start with 'pilot-'", basename)
	}
}

func TestClone_CleanupFunction(t *testing.T) {
	skipIfNetworkUnavailable(t)
	
	// Test cleanup function behavior
	repo := "octocat/Hello-World"
	branch := "master"

	workdir, cleanup, err := Clone(repo, branch)
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			t.Skip("gh CLI not available")
		}
		t.Fatalf("Clone() error: %v", err)
	}

	// Verify directory exists before cleanup
	if _, err := os.Stat(workdir); os.IsNotExist(err) {
		t.Errorf("Workdir %s should exist before cleanup", workdir)
	}

	// Call cleanup
	cleanup()

	// Verify directory no longer exists
	if _, err := os.Stat(workdir); !os.IsNotExist(err) {
		t.Errorf("Workdir %s should not exist after cleanup", workdir)
	}

	// Calling cleanup again should be safe (no panic)
	cleanup()
}

func TestClone_GitDirectoryExists(t *testing.T) {
	skipIfNetworkUnavailable(t)
	
	repo := "octocat/Hello-World"
	branch := "master"

	workdir, cleanup, err := Clone(repo, branch)
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			t.Skip("gh CLI not available")
		}
		t.Fatalf("Clone() error: %v", err)
	}
	defer cleanup()

	// Verify .git directory exists (indicates successful git clone)
	gitDir := filepath.Join(workdir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Errorf(".git directory should exist in %s", workdir)
	}
}

func TestClone_RetryLogic(t *testing.T) {
	skipIfNetworkUnavailable(t)
	
	// This test verifies retry logic is in place
	// We can't easily test actual retries without mocking, but we can verify
	// the function signature and error handling

	// Test with guaranteed failure that should exhaust retries
	repo := "definitely/nonexistent-repo-xyz-123-456"
	branch := "main"

	_, cleanup, err := Clone(repo, branch)
	if err == nil {
		t.Error("Clone() should fail for nonexistent repo")
		if cleanup != nil {
			cleanup()
		}
		return
	}

	// Verify error message mentions clone failure
	if !strings.Contains(err.Error(), "clone") {
		t.Errorf("Clone() error should mention clone, got: %v", err)
	}

	// cleanup should be nil on error
	if cleanup != nil {
		t.Error("Clone() cleanup should be nil on error")
	}
}
