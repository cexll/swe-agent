ARG GO_VERSION=1.25.1
ARG CLAUDE_CLI_VERSION=1.0.111
ARG CODEX_CLI_VERSION=0.40.0

FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /build

# Install git (required for go modules)
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o swe-agent ./cmd

# Final stage
FROM alpine:3.20 AS runtime

ARG CLAUDE_CLI_VERSION
ARG CODEX_CLI_VERSION

ENV NODE_ENV=production \
    NPM_CONFIG_FUND=false \
    NPM_CONFIG_AUDIT=false \
    NPM_CONFIG_UPDATE_NOTIFIER=false

# Install base tooling for GitHub operations and CLI dependencies
RUN apk add --no-cache \
        bash \
        ca-certificates \
        git \
        github-cli \
        openssh-client \
        wget \
        py3-pip \
        make \
        g++ \
        python3 \
        nodejs \
        npm \
        jq \
    && npm install -g \
        @anthropic-ai/claude-code@${CLAUDE_CLI_VERSION} \
        @openai/codex@${CODEX_CLI_VERSION} \
    && npm cache clean --force \
    && wget -qO- https://astral.sh/uv/install.sh | sh

# Ensure uv/uvx are on PATH
ENV PATH="/root/.local/bin:${PATH}"

# Copy binary from builder
COPY --from=builder /build/swe-agent /usr/local/bin/swe-agent

# Copy shared system prompt to agent config locations
COPY --from=builder /build/system-prompt.md /tmp/system-prompt.md
RUN mkdir -p /root/.codex /root/.claude \
    && cp /tmp/system-prompt.md /root/.codex/AGENTS.md \
    && cp /tmp/system-prompt.md /root/.claude/CLAUDE.md \
    && rm /tmp/system-prompt.md

WORKDIR /app

# Copy runtime assets
COPY --from=builder /build/templates ./templates

# Copy and setup entrypoint script
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Expose port
EXPOSE 8000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8000/health || exit 1

# Run the application
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
