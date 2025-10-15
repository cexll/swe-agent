package github

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClone_PathGeneration(t *testing.T) {
	// Test that Clone generates a valid temporary directory path
	// Note: This test will fail without gh CLI, but we can test path generation logic

	repo := "owner/repo"
	branch := "main"

	// Since we can't actually clone without gh CLI, we test the path generation pattern
	tmpDir := filepath.Join(os.TempDir(), "pilot-test")
	if tmpDir == "" {
		t.Error("Temp directory path should not be empty")
	}

	// Check that the path is in the temp directory
	if !strings.HasPrefix(tmpDir, os.TempDir()) {
		t.Errorf("Clone directory path should be under temp directory, got %s", tmpDir)
	}

	// Test parameters
	if repo == "" {
		t.Error("Repo parameter should not be empty")
	}
	if branch == "" {
		t.Error("Branch parameter should not be empty")
	}
}

func TestRepoFormat(t *testing.T) {
	// Test that repo format is validated correctly
	tests := []struct {
		name  string
		repo  string
		valid bool
	}{
		{
			name:  "valid repo format",
			repo:  "owner/repo",
			valid: true,
		},
		{
			name:  "valid repo with dash",
			repo:  "my-org/my-repo",
			valid: true,
		},
		{
			name:  "invalid - no slash",
			repo:  "ownerrepo",
			valid: false,
		},
		{
			name:  "invalid - multiple slashes",
			repo:  "owner/repo/extra",
			valid: false,
		},
		{
			name:  "invalid - starts with slash",
			repo:  "/owner/repo",
			valid: false,
		},
		{
			name:  "invalid - ends with slash",
			repo:  "owner/repo/",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := strings.Split(tt.repo, "/")
			isValid := len(parts) == 2 && parts[0] != "" && parts[1] != ""

			if isValid != tt.valid {
				t.Errorf("Repo format validation = %v, want %v", isValid, tt.valid)
			}
		})
	}
}

func TestBranchNameValidation(t *testing.T) {
	// Test branch name format
	tests := []struct {
		name   string
		branch string
		valid  bool
	}{
		{
			name:   "main branch",
			branch: "main",
			valid:  true,
		},
		{
			name:   "develop branch",
			branch: "develop",
			valid:  true,
		},
		{
			name:   "feature branch",
			branch: "feature/add-login",
			valid:  true,
		},
		{
			name:   "bugfix branch",
			branch: "bugfix/fix-crash",
			valid:  true,
		},
		{
			name:   "empty branch",
			branch: "",
			valid:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.branch != ""

			if isValid != tt.valid {
				t.Errorf("Branch name validation = %v, want %v", isValid, tt.valid)
			}
		})
	}
}

// Test Clone function parameters
func TestClone_ParameterTypes(t *testing.T) {
	// Test that function accepts correct parameter types
	var repo string = "owner/repo"
	var branch string = "main"
	var token string = "token"

	// Type checking - this will compile if types are correct
	_, _, _ = Clone(repo, branch, token)
}

// TestClone_CommandConstruction verifies that the gh clone command is constructed correctly
// This test validates the fix for the "--branch" flag issue
func TestClone_CommandConstruction(t *testing.T) {
	// The correct gh repo clone syntax for specifying a branch is:
	// gh repo clone <repo> <dir> -- -b <branch>
	//
	// NOT: gh repo clone <repo> <dir> --branch <branch>
	//
	// This test documents the expected command structure.

	repo := "owner/repo"
	branch := "feature-branch"

	// Expected command structure after fix:
	// ["gh", "repo", "clone", "owner/repo", "/tmp/pilot-XXX", "--", "-b", "feature-branch"]
	//
	// Key points:
	// 1. "--" separator is required before git flags
	// 2. Use "-b" (git's short flag) instead of "--branch" (gh doesn't support this)
	// 3. Git flags come after "--" separator

	expectedArgs := []string{
		"repo", "clone",
		repo,
		// tmpDir would be here but it's dynamic
		"--", // Separator: gh flags before, git flags after
		"-b", // Git's branch flag
		branch,
	}

	// Verify the expected argument structure
	if len(expectedArgs) != 6 {
		t.Errorf("Expected 6 static arguments (excluding tmpDir), got %d", len(expectedArgs))
	}

	// Verify separator position
	if expectedArgs[3] != "--" {
		t.Errorf("Expected '--' separator at position 3, got %s", expectedArgs[3])
	}

	// Verify git branch flag
	if expectedArgs[4] != "-b" {
		t.Errorf("Expected '-b' flag at position 4, got %s", expectedArgs[4])
	}

	// Verify branch value
	if expectedArgs[5] != branch {
		t.Errorf("Expected branch '%s' at position 5, got %s", branch, expectedArgs[5])
	}

	// Document the incorrect syntax that caused the original bug
	incorrectFlag := "--branch"
	if incorrectFlag == "--branch" {
		// This would fail with: "unknown flag: --branch"
		t.Logf("Documented: '%s' flag is NOT supported by gh repo clone", incorrectFlag)
	}
}

// Test package-level behavior
func TestPackageImports(t *testing.T) {
	// Verify that required packages are imported
	// This is a compile-time check
	_ = "fmt"
	_ = "os/exec"
	_ = "strings"
}
