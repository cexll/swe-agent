#!/bin/sh
set -e

# Ensure directories exist
mkdir -p /root/.codex /root/.claude

# Write Codex auth.json
jq -n --arg key "${OPENAI_API_KEY:-}" '{OPENAI_API_KEY: $key}' > /root/.codex/auth.json

# Note: Codex MCP config (config.toml) is now dynamically generated at runtime
# in internal/provider/codex/codex.go to avoid conflicts and allow per-execution customization

# Note: Claude MCP config is now dynamically generated via --mcp-config flag
# in internal/provider/claude/claude.go to avoid conflicts with user's ~/.claude.json

# Start the service
exec swe-agent "$@"
