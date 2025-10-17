package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// UpdateCommentRequest represents the request body for updating a comment
type UpdateCommentRequest struct {
	Body string `json:"body"`
}

// UpdateComment updates an existing issue or PR comment using GitHub REST API
// PATCH /repos/{owner}/{repo}/issues/comments/{comment_id}
func UpdateComment(owner, repo string, commentID int64, body, token string) error {
	if token == "" {
		return fmt.Errorf("github token is required")
	}
	if commentID <= 0 {
		return fmt.Errorf("invalid comment ID: %d", commentID)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/comments/%d", owner, repo, commentID)

	reqBody := UpdateCommentRequest{Body: body}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
