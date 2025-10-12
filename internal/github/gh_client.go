package github

import (
	"encoding/json"
	"fmt"
	"os"
)

// GHClient is an interface for GitHub CLI operations
// This abstraction allows mocking gh CLI in tests
type GHClient interface {
	// CreateComment creates a comment and returns its ID
	CreateComment(repo string, number int, body, token string) (int, error)

	// UpdateComment updates an existing comment
	UpdateComment(repo string, commentID int, body, token string) error

	// GetCommentBody retrieves the current body of a comment
	GetCommentBody(repo string, commentID int, token string) (string, error)

	// AddLabel adds a label to an issue/PR
	AddLabel(repo string, number int, label, token string) error

	// Clone clones a repository to a directory
	Clone(repo, branch, destDir string) error

	// CreatePR creates a pull request
	CreatePR(workdir, repo, head, base, title, body string) (string, error)
}

// RealGHClient is the production implementation using gh CLI
type RealGHClient struct {
	runner CommandRunner
}

// NewRealGHClient creates a new real gh client
func NewRealGHClient() *RealGHClient {
	return &RealGHClient{
		runner: &RealCommandRunner{},
	}
}

// CreateComment creates a comment and returns its ID
func (c *RealGHClient) CreateComment(repo string, number int, body, token string) (int, error) {
	var commentID int
	err := retryWithBackoff(func() error {
		args := []string{
			"api",
			fmt.Sprintf("/repos/%s/issues/%d/comments", repo, number),
			"-X", "POST",
			"-f", fmt.Sprintf("body=%s", body),
		}

		// Set token via environment
		oldToken := os.Getenv("GITHUB_TOKEN")
		os.Setenv("GITHUB_TOKEN", token)
		defer os.Setenv("GITHUB_TOKEN", oldToken)

		output, err := c.runner.Run("gh", args...)
		if err != nil {
			return fmt.Errorf("gh api failed: %w\nOutput: %s", err, string(output))
		}

		// Parse JSON response
		var result struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal(output, &result); err != nil {
			return fmt.Errorf("failed to parse comment response: %w", err)
		}

		commentID = result.ID
		return nil
	})

	return commentID, err
}

// UpdateComment updates an existing comment
func (c *RealGHClient) UpdateComment(repo string, commentID int, body, token string) error {
	return retryWithBackoff(func() error {
		args := []string{
			"api",
			fmt.Sprintf("/repos/%s/issues/comments/%d", repo, commentID),
			"-X", "PATCH",
			"-f", fmt.Sprintf("body=%s", body),
		}

		oldToken := os.Getenv("GITHUB_TOKEN")
		os.Setenv("GITHUB_TOKEN", token)
		defer os.Setenv("GITHUB_TOKEN", oldToken)

		output, err := c.runner.Run("gh", args...)
		if err != nil {
			return fmt.Errorf("gh api update failed: %w\nOutput: %s", err, string(output))
		}

		return nil
	})
}

// GetCommentBody retrieves the current body of a comment
func (c *RealGHClient) GetCommentBody(repo string, commentID int, token string) (string, error) {
	var body string
	err := retryWithBackoff(func() error {
		args := []string{
			"api",
			fmt.Sprintf("/repos/%s/issues/comments/%d", repo, commentID),
		}

		oldToken := os.Getenv("GITHUB_TOKEN")
		os.Setenv("GITHUB_TOKEN", token)
		defer os.Setenv("GITHUB_TOKEN", oldToken)

		output, err := c.runner.Run("gh", args...)
		if err != nil {
			return fmt.Errorf("gh api get failed: %w\nOutput: %s", err, string(output))
		}

		var result struct {
			Body string `json:"body"`
		}
		if err := json.Unmarshal(output, &result); err != nil {
			return fmt.Errorf("failed to parse comment: %w", err)
		}

		body = result.Body
		return nil
	})

	return body, err
}

// AddLabel adds a label to an issue/PR
func (c *RealGHClient) AddLabel(repo string, number int, label, token string) error {
	return retryWithBackoff(func() error {
		args := []string{
			"issue", "edit", fmt.Sprintf("%d", number),
			"--repo", repo,
			"--add-label", label,
		}

		oldToken := os.Getenv("GITHUB_TOKEN")
		os.Setenv("GITHUB_TOKEN", token)
		defer os.Setenv("GITHUB_TOKEN", oldToken)

		output, err := c.runner.Run("gh", args...)
		if err != nil {
			return fmt.Errorf("gh issue edit failed: %w\nOutput: %s", err, string(output))
		}

		return nil
	})
}

// Clone clones a repository to a directory
func (c *RealGHClient) Clone(repo, branch, destDir string) error {
	return retryWithBackoff(func() error {
		args := []string{
			"repo", "clone", repo, destDir, "--", "-b", branch,
		}

		output, err := c.runner.Run("gh", args...)
		if err != nil {
			return fmt.Errorf("gh repo clone failed: %w\nOutput: %s", err, string(output))
		}

		return nil
	})
}

// CreatePR creates a pull request
func (c *RealGHClient) CreatePR(workdir, repo, head, base, title, body string) (string, error) {
	args := []string{
		"pr", "create",
		"--repo", repo,
		"--head", head,
		"--base", base,
		"--title", title,
		"--body", body,
	}

	output, err := c.runner.RunInDir(workdir, "gh", args...)
	if err != nil {
		return "", fmt.Errorf("gh pr create failed: %w\nOutput: %s", err, string(output))
	}

	// gh pr create returns the PR URL
	return string(output), nil
}

// MockGHClient is a mock implementation for testing
type MockGHClient struct {
	CreateCommentFunc  func(repo string, number int, body, token string) (int, error)
	UpdateCommentFunc  func(repo string, commentID int, body, token string) error
	GetCommentBodyFunc func(repo string, commentID int, token string) (string, error)
	AddLabelFunc       func(repo string, number int, label, token string) error
	CloneFunc          func(repo, branch, destDir string) error
	CreatePRFunc       func(workdir, repo, head, base, title, body string) (string, error)

	// Track calls
	CreateCommentCalls []struct {
		Repo   string
		Number int
		Body   string
		Token  string
	}
	UpdateCommentCalls []struct {
		Repo      string
		CommentID int
		Body      string
		Token     string
	}
	GetCommentCalls []struct {
		Repo      string
		CommentID int
		Token     string
	}
	AddLabelCalls []struct {
		Repo   string
		Number int
		Label  string
		Token  string
	}
	CloneCalls []struct {
		Repo    string
		Branch  string
		DestDir string
	}
	CreatePRCalls []struct {
		Workdir string
		Repo    string
		Head    string
		Base    string
		Title   string
		Body    string
	}
}

// NewMockGHClient creates a new mock gh client
func NewMockGHClient() *MockGHClient {
	return &MockGHClient{}
}

// CreateComment mock implementation
func (m *MockGHClient) CreateComment(repo string, number int, body, token string) (int, error) {
	m.CreateCommentCalls = append(m.CreateCommentCalls, struct {
		Repo   string
		Number int
		Body   string
		Token  string
	}{repo, number, body, token})

	if m.CreateCommentFunc != nil {
		return m.CreateCommentFunc(repo, number, body, token)
	}

	return 12345, nil // Default mock comment ID
}

// UpdateComment mock implementation
func (m *MockGHClient) UpdateComment(repo string, commentID int, body, token string) error {
	m.UpdateCommentCalls = append(m.UpdateCommentCalls, struct {
		Repo      string
		CommentID int
		Body      string
		Token     string
	}{repo, commentID, body, token})

	if m.UpdateCommentFunc != nil {
		return m.UpdateCommentFunc(repo, commentID, body, token)
	}

	return nil
}

// GetCommentBody mock implementation
func (m *MockGHClient) GetCommentBody(repo string, commentID int, token string) (string, error) {
	m.GetCommentCalls = append(m.GetCommentCalls, struct {
		Repo      string
		CommentID int
		Token     string
	}{repo, commentID, token})

	if m.GetCommentBodyFunc != nil {
		return m.GetCommentBodyFunc(repo, commentID, token)
	}

	return "mock comment body", nil
}

// AddLabel mock implementation
func (m *MockGHClient) AddLabel(repo string, number int, label, token string) error {
	m.AddLabelCalls = append(m.AddLabelCalls, struct {
		Repo   string
		Number int
		Label  string
		Token  string
	}{repo, number, label, token})

	if m.AddLabelFunc != nil {
		return m.AddLabelFunc(repo, number, label, token)
	}

	return nil
}

// Clone mock implementation
func (m *MockGHClient) Clone(repo, branch, destDir string) error {
	m.CloneCalls = append(m.CloneCalls, struct {
		Repo    string
		Branch  string
		DestDir string
	}{repo, branch, destDir})

	if m.CloneFunc != nil {
		return m.CloneFunc(repo, branch, destDir)
	}

	return nil
}

// CreatePR mock implementation
func (m *MockGHClient) CreatePR(workdir, repo, head, base, title, body string) (string, error) {
	m.CreatePRCalls = append(m.CreatePRCalls, struct {
		Workdir string
		Repo    string
		Head    string
		Base    string
		Title   string
		Body    string
	}{workdir, repo, head, base, title, body})

	if m.CreatePRFunc != nil {
		return m.CreatePRFunc(workdir, repo, head, base, title, body)
	}

	return "https://github.com/owner/repo/pull/1", nil
}
