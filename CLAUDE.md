# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Tools

- **Runtime**: Go 1.25.1
- **Web Framework**: Gorilla Mux
- **Key Dependencies**:
  - `lancekrogers/claude-code-go` - Claude Code Go SDK
  - `github.com/golang-jwt/jwt/v5` - GitHub App JWT authentication
  - `github.com/joho/godotenv` - Environment variable management

## Common Development Tasks

### Build and Run

```bash
# Build the binary
go build -o pilot-swe cmd/main.go

# Run directly
go run cmd/main.go

# Run with environment variables loaded
source .env && go run cmd/main.go
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test ./... -cover

# Generate detailed coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# View coverage by function
go tool cover -func=coverage.out

# Run specific package tests
go test ./internal/webhook/...
go test ./internal/provider/...
```

### Code Quality

```bash
# Format code
go fmt ./...

# Lint/vet code
go vet ./...

# Tidy dependencies
go mod tidy
```

### Docker

```bash
# Build Docker image
docker build -t pilot-swe .

# Run container
docker run -d -p 3000:3000 \
  -e GITHUB_APP_ID=123456 \
  -e GITHUB_PRIVATE_KEY="$(cat private-key.pem)" \
  -e GITHUB_WEBHOOK_SECRET=secret \
  -e ANTHROPIC_API_KEY=sk-ant-xxx \
  --name pilot-swe \
  pilot-swe
```

## Architecture Overview

Pilot SWE is a GitHub App webhook service that responds to `/code` commands in issue comments to automatically generate and commit code changes.

### Request Flow

```
GitHub Webhook (issue_comment event)
      ↓
  Handler (verify HMAC signature)
      ↓
  Executor (orchestrate task)
      ↓
  Provider (AI code generation)
      ↓
  GitHub Operations (clone, commit, push)
      ↓
  Comment (post PR creation link)
```

### Core Components

#### 1. Webhook Handler (`internal/webhook/`)

- **handler.go**: HTTP endpoint for GitHub webhooks, event parsing
- **verify.go**: HMAC SHA-256 signature verification (constant-time comparison)
- **types.go**: GitHub webhook payload types

#### 2. Provider System (`internal/provider/`)

- **provider.go**: Interface definition for AI backends
- **factory.go**: Provider factory pattern for instantiation
- **claude/**: Claude Code implementation
- **codex/**: Codex implementation (multi-provider support)
 - **prompt/**: Shared prompt manager used by all providers

Provider interface enables zero-branch polymorphism:

```go
type Provider interface {
    GenerateCode(ctx, req) (*CodeResponse, error)
    Name() string
}
```

All providers must source prompts from `internal/prompt` to ensure identical instructions across backends. This avoids drift and duplication.

#### 3. Task Executor (`internal/executor/`)

- **task.go**: Orchestrates the full workflow:
  1. Clone repository
  2. Call AI provider
  3. Apply changes to filesystem
  4. Commit and push to new branch
  5. Post comment with PR link

#### 4. GitHub Operations (`internal/github/`)

- **auth.go**: GitHub App JWT token generation and installation token exchange
- **clone.go**: Repository cloning via `gh repo clone`
- **comment.go**: Comment posting via `gh issue comment`
- **pr.go**: PR creation URL generation

#### 5. Configuration (`internal/config/`)

- **config.go**: Environment variable loading and validation
- Supports multiple providers (Claude, Codex)
- Validates required secrets at startup

### Project Structure

```
swe/
├── cmd/
│   └── main.go                          # HTTP server entry point
├── internal/
│   ├── config/                          # Configuration management
│   ├── webhook/                         # GitHub webhook handling
│   ├── provider/                        # AI provider abstraction
│   │   ├── claude/                      # Claude implementation
│   │   └── codex/                       # Codex implementation
│   ├── executor/                        # Task orchestration
│   └── github/                          # GitHub API operations
├── Dockerfile                           # Container build
├── .env.example                         # Environment template
└── TEST_COVERAGE_REPORT.md              # Detailed test coverage
```

## Important Implementation Notes

### Provider Pattern Design

The provider system eliminates conditional branching through interface polymorphism:

```go
// Adding a new provider requires:
// 1. Implement Provider interface in internal/provider/<name>/
// 2. Add case in factory.go NewProvider() function
// 3. Add config fields in internal/config/config.go
// 4. No changes to executor or handler needed
```

### Authentication Flow

1. **GitHub App JWT**: Signs JWT with private key, includes App ID
2. **Installation Token**: Exchanges JWT for short-lived installation token via GitHub API
3. **Git Operations**: Uses installation token for authenticated git commands

Token generation happens per-request to ensure fresh credentials.

### Webhook Security

- HMAC SHA-256 signature verification using webhook secret
- Constant-time comparison prevents timing attacks (`subtle.ConstantTimeCompare`)
- Signature format: `sha256=<hex-encoded-hmac>`

### Error Handling Strategy

Errors are automatically posted as GitHub comments for user visibility:

```go
if err != nil {
    return e.notifyError(task, errorMsg)
    // User sees detailed error in GitHub comment
    // No need to check logs
}
```

### CLI Tool Dependencies

This project delegates Git operations to CLI tools rather than reimplementing them:

- **`gh` CLI**: All GitHub operations (clone, comment, PR)
- **`claude` CLI**: AI code generation via lancekrogers/claude-code-go

Ensure both CLIs are installed and available in PATH.

## Code Conventions

### Design Philosophy (Linus-Style)

1. **Good Taste - Eliminate Special Cases**

   - Use interfaces over if/else chains
   - Design data structures to make edge cases disappear
   - Prefer polymorphism to conditionals

2. **Shallow Indentation**

   - Functions should not exceed 3 levels of indentation
   - Early returns over nested conditionals
   - Extract complex logic into helper functions

3. **Clear Naming**

   - Use domain-specific names: `Provider`, `Executor`, `Handler`
   - Avoid generic names: `Manager`, `Service`, `Helper`
   - Package names match their primary type

4. **Error Visibility**

   - Don't hide errors in logs
   - Surface errors to users (GitHub comments)
   - Include context in error messages

5. **Backward Compatibility**
   - Provider interface designed for future extension
   - Config fields have sensible defaults
   - No breaking API changes without major version bump

### Testing Standards

- Target: >75% coverage overall
- 100% coverage for security-critical code (webhook verification, auth)
- Test files located alongside implementation: `file.go` → `file_test.go`
- Use table-driven tests for multiple scenarios

## Multi-Provider Support

Current providers:

- **Claude**: Via `lancekrogers/claude-code-go` SDK
- **Codex**: Via Codex provider implementation

Provider selection via environment variable:

```bash
PROVIDER=claude  # or "codex"
CLAUDE_API_KEY=sk-ant-xxx
CLAUDE_MODEL=claude-3-5-sonnet-20241022
```
