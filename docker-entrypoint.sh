#!/bin/bash
set -e

# Initialize Codex authentication if OPENAI_API_KEY is provided
if [ -n "$OPENAI_API_KEY" ]; then
    mkdir -p ~/.codex
    echo "{\"OPENAI_API_KEY\": \"$OPENAI_API_KEY\"}" > ~/.codex/auth.json
    chmod 600 ~/.codex/auth.json
    echo "[Entrypoint] Codex authentication configured"
fi

# Execute the main application
exec "$@"
