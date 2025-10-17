package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	gh "github.com/cexll/swe/internal/github"
)

// Client is a thin GitHub GraphQL client that acquires
// an installation token via our GitHub App auth provider.
// Keep it minimal and focused.
type Client struct {
	httpClient   *http.Client
	endpoint     string
	authProvider gh.AuthProvider
}

// NewClient creates a GraphQL client using the provided auth provider.
func NewClient(auth gh.AuthProvider) *Client {
	return &Client{
		httpClient:   &http.Client{Timeout: 20 * time.Second},
		endpoint:     "https://api.github.com/graphql",
		authProvider: auth,
	}
}

// GraphQLRequest represents a GraphQL request body.
type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// Do executes a GraphQL POST to GitHub's API using an installation token
// for the given repository ("owner/repo"). The response body is decoded
// into out; if GitHub returns errors, an error is produced with details.
func (c *Client) Do(ctx context.Context, repo, query string, variables map[string]interface{}, out interface{}) error {
	if repo == "" {
		return fmt.Errorf("repo is required (owner/repo)")
	}

	token, err := c.authProvider.GetInstallationToken(repo)
	if err != nil {
		return fmt.Errorf("failed to get installation token: %w", err)
	}

	reqBody := GraphQLRequest{Query: query, Variables: variables}
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(reqBody); err != nil {
		return fmt.Errorf("encode graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, buf)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("graphql http error: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("graphql status %d: %s", resp.StatusCode, string(body))
	}

	var wrapper struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return fmt.Errorf("decode graphql envelope: %w", err)
	}
	if len(wrapper.Errors) > 0 {
		// return the first error message to keep it simple
		return fmt.Errorf("graphql error: %s", wrapper.Errors[0].Message)
	}
	if len(wrapper.Data) == 0 {
		// Some queries legitimately have null data. We still try to decode.
		// If a "data" field is absent, decode against JSON null to avoid EOF.
		if out != nil {
			wrapper.Data = json.RawMessage("null")
		}
	}
	if out != nil {
		if err := json.Unmarshal(wrapper.Data, out); err != nil {
			return fmt.Errorf("decode graphql data: %w", err)
		}
	}
	return nil
}
