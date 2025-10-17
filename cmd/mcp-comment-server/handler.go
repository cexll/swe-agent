package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/cexll/swe/internal/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// UpdateCommentParams defines the input parameters for the tool
// Corresponds to TypeScript: { body: z.string() }
type UpdateCommentParams struct {
	Body string `json:"body" jsonschema:"The updated comment content"`
}

// HandleUpdateComment handles the update_claude_comment tool call
// Corresponds to TypeScript: async ({ body }) => {...}
func HandleUpdateComment(
	ctx context.Context,
	req *mcp.CallToolRequest,
	params UpdateCommentParams,
) (*mcp.CallToolResult, any, error) {
	log.Printf("[MCP Comment Server] Received update_claude_comment request")

	// 1. Read configuration from environment variables (process.env in TypeScript)
	owner := os.Getenv("REPO_OWNER")
	repo := os.Getenv("REPO_NAME")
	commentIDStr := os.Getenv("CLAUDE_COMMENT_ID")
	token := os.Getenv("GITHUB_TOKEN")
	eventName := os.Getenv("GITHUB_EVENT_NAME")

	// 2. Validate parameters
	if params.Body == "" {
		return nil, nil, fmt.Errorf("body parameter is required")
	}

	// 3. Parse comment ID
	commentID, err := strconv.ParseInt(commentIDStr, 10, 64)
	if err != nil {
		log.Printf("[MCP Comment Server] Invalid CLAUDE_COMMENT_ID: %v", err)
		return nil, nil, fmt.Errorf("invalid CLAUDE_COMMENT_ID: %w", err)
	}

	// 4. Content sanitization (corresponds to TypeScript sanitizeContent)
	// Note: Go version simplified for now, can add sanitizer later
	sanitizedBody := params.Body
	log.Printf("[MCP Comment Server] Updating comment with %d characters", len(sanitizedBody))

	// 5. Call GitHub API to update comment
	// Corresponds to TypeScript: updateClaudeComment(octokit, {...})
	if err := github.UpdateComment(owner, repo, commentID, sanitizedBody, token); err != nil {
		log.Printf("[MCP Comment Server] Failed to update comment: %v", err)

		// Return error result (corresponds to TypeScript isError: true)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil, nil
	}

	// 6. Construct success response
	// Corresponds to TypeScript: { content: [{ type: "text", text: JSON.stringify(result) }] }
	resultText := fmt.Sprintf(`{
  "success": true,
  "owner": "%s",
  "repo": "%s",
  "comment_id": %d,
  "event_name": "%s",
  "body_length": %d
}`, owner, repo, commentID, eventName, len(sanitizedBody))

	log.Printf("[MCP Comment Server] Successfully updated comment #%d", commentID)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: resultText},
		},
	}, nil, nil
}
