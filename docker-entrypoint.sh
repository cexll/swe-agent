#!/bin/sh
set -e

# Ensure directories exist
mkdir -p /root/.codex /root/.claude

# Write Codex auth.json
jq -n --arg key "${OPENAI_API_KEY:-}" '{OPENAI_API_KEY: $key}' > /root/.codex/auth.json

# Write Codex MCP config (config.toml)
cat > /root/.codex/config.toml <<TOML
model = "gpt-5-codex"
model_reasoning_effort = "high"
model_reasoning_summary = "detailed"
approval_policy = "never"
sandbox_mode = "danger-full-access"
disable_response_storage = true
network_access = true

[mcp_servers.github]
type = "http"
url = "https://api.githubcopilot.com/mcp"

[mcp_servers.github.headers]
Authorization = "Bearer ${GITHUB_TOKEN}"

[mcp_servers.git]
command = "uvx"
args = ["mcp-server-git"]
TOML

# Write Claude MCP config (.claude.json)
cat > /root/.claude.json <<JSON
{
  "mcpServers": {
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp",
      "headers": {
        "Authorization": "Bearer ${GITHUB_TOKEN}"
      }
    },
    "git": {
      "command": "uvx",
      "args": ["mcp-server-git"]
    }
  }
}
JSON

# Start the service
exec swe-agent "$@"
