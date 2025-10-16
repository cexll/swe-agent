package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	// 1. Validate required environment variables
	requiredEnv := []string{"GITHUB_TOKEN", "REPO_OWNER", "REPO_NAME", "CLAUDE_COMMENT_ID"}
	for _, env := range requiredEnv {
		if os.Getenv(env) == "" {
			log.Fatalf("[MCP Comment Server] Missing required environment variable: %s", env)
		}
	}

	log.Println("[MCP Comment Server] Starting GitHub Comment MCP Server v1.0.0")
	log.Printf("[MCP Comment Server] Repository: %s/%s", os.Getenv("REPO_OWNER"), os.Getenv("REPO_NAME"))
	log.Printf("[MCP Comment Server] Comment ID: %s", os.Getenv("CLAUDE_COMMENT_ID"))

	// 2. Create MCP server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "github-comment-server",
		Version: "v1.0.0",
	}, nil)

	// 3. Register update_claude_comment tool
	tool := &mcp.Tool{
		Name:        "update_claude_comment",
		Description: "Update the Claude comment with progress and results (automatically handles both issue and PR comments)",
	}
	mcp.AddTool(server, tool, HandleUpdateComment)
	log.Println("[MCP Comment Server] Registered tool: update_claude_comment")

	// 4. Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("[MCP Comment Server] Received shutdown signal")
		cancel()
	}()

	// 5. Start server with stdio transport
	log.Println("[MCP Comment Server] Starting on stdio transport...")
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("[MCP Comment Server] Server error: %v", err)
	}
	log.Println("[MCP Comment Server] Server stopped gracefully")
}
