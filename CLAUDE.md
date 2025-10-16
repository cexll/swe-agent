# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Tools

- **Runtime**: Go 1.25.1
- **Web Framework**: Gorilla Mux
- **Key Dependencies**:
  - `github.com/golang-jwt/jwt/v5` - GitHub App JWT authentication
  - `github.com/joho/godotenv` - Environment variable management

## v2.0 Architecture Highlights

**Major Simplification (October 2025):**
- ✅ **59% code reduction**: 3,150 → 1,300 lines
- ✅ **85.2% test coverage**: Up from 67%
- ✅ **Modular architecture**: New data, prompt, dispatcher, taskstore, and web packages
- ✅ **Executor simplified**: 1,400 → 150 lines
- ✅ **All tests passing**: Production ready

**New Components:**
- `internal/github/data/` - GraphQL data layer for fetching GitHub context (91% coverage)
- `internal/prompt/` - System prompt loading and building (92% coverage)
- `internal/dispatcher/` - Task queue with exponential backoff (91% coverage)
- `internal/taskstore/` - In-memory task storage (100% coverage)
- `internal/web/` - Web UI for task dashboard (95% coverage)
- `internal/github/postprocess/` - **NEW**: Post-execution processing (40% coverage)
  - Branch status detection and cleanup
  - PR/branch link generation
  - Coordinating comment updates

**Key Improvements:**
- **No factory pattern**: Direct provider instantiation in main.go
- **GraphQL over REST**: Efficient data fetching via GraphQL
- **API-based commits**: Use GitHub API instead of local git
- **System prompt file**: External `system-prompt.md` for easy customization

## Common Development Tasks

### Build and Run

```bash
# Build the binary
go build -o swe-agent cmd/main.go

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

### Web UI Dashboard (v2.0)

Access the task dashboard after starting the service:

```bash
# Start the service
go run cmd/main.go

# Access the dashboard
open http://localhost:8000/tasks
```

The dashboard provides:
- List of all tasks (pending, running, completed, failed)
- Task details with execution logs
- Repository and actor information
- Real-time status updates

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

**MCP Configuration (v2.0):**

The Docker image uses **HTTP-based MCP servers** configured at runtime via `docker-entrypoint.sh`:

- **GitHub MCP**: Connects to `https://api.githubcopilot.com/mcp` (requires `GITHUB_TOKEN`)
- **Git MCP**: Uses `uvx mcp-server-git` (requires `REPO_DIR` set by executor)

MCP configs are dynamically generated:
- `~/.claude.json` - Claude Code MCP configuration
- `~/.codex/config.toml` - Codex MCP configuration

**Build and run:**

```bash
# Build Docker image
docker build -t swe-agent .

# Run container (requires GITHUB_TOKEN for MCP access)
docker run -d -p 8000:8000 \
  -e GITHUB_APP_ID=123456 \
  -e GITHUB_PRIVATE_KEY="$(cat private-key.pem)" \
  -e GITHUB_WEBHOOK_SECRET=secret \
  -e GITHUB_TOKEN=github_pat_xxx \
  -e ANTHROPIC_API_KEY=sk-ant-xxx \
  --name swe-agent \
  swe-agent
```

**Required environment variables:**
- `GITHUB_TOKEN`: GitHub Personal Access Token (scopes: `repo`, `read:org`) for MCP HTTP access
- `GITHUB_APP_ID`: GitHub App ID for webhook authentication
- `GITHUB_PRIVATE_KEY`: GitHub App private key
- `ANTHROPIC_API_KEY` or `OPENAI_API_KEY`: AI provider credentials

## Architecture Overview

SWE-Agent is a GitHub App webhook service that responds to `/code` commands in issue/PR comments to automatically generate and commit code changes.

### Request Flow (v2.0 Architecture)

```
GitHub Webhook (issue_comment/pr_review_comment)
      ↓
  Handler (verify HMAC signature, parse event)
      ↓
  Dispatcher (queue management, exponential backoff retry)
      ↓
  Executor (orchestrate task)
      ↓
  GitHub Data Layer (fetch issue/PR context via GraphQL)
      ↓
  Prompt Builder (system-prompt.md + XML context)
      ↓
  Provider (AI code generation: Claude/Codex)
      ↓
  Commit (local git OR GitHub API signing)
      ↓
  Push (gh CLI)
      ↓
  Post-Processing (NEW)
    - Check branch status
    - Generate branch/PR links
    - Update coordinating comment
    - Delete empty branches
```

### Core Components (v2.0 Simplified)

#### 1. Webhook Handler (`internal/webhook/`)

- **handler.go**: HTTP endpoint for GitHub webhooks, event parsing, permission checks
- **verify.go**: HMAC SHA-256 signature verification (constant-time comparison)
- **analysis.go**: Command analysis and extraction from comments
- **types.go**: GitHub webhook payload types

#### 2. Dispatcher (`internal/dispatcher/`)

- **dispatcher.go**: Task queue with bounded capacity, worker pool
- **Keyed mutex**: Serializes tasks per PR to avoid conflicts
- **Exponential backoff**: Auto-retry with configurable backoff strategy

#### 3. Task Executor (`internal/executor/`)

**Simplified from 1,400 to 150 lines in v2.0**

- **task.go**: Orchestrates the full workflow:
  1. Fetch GitHub context via data layer
  2. Build prompt via prompt builder
  3. Call AI provider
  4. Commit changes via GitHub API
  5. Push branch via gh CLI
  6. Post PR creation link
- **adapter.go**: Adapter interface for provider integration

#### 4. GitHub Data Layer (`internal/github/data/`)

**New in v2.0 - 91% test coverage**

- **client.go**: GraphQL client with installation token auth
- **fetcher.go**: Fetch issue/PR data, comments, reviews, files
- **formatter.go**: Format data as XML for AI consumption
- **fetcher_wrapper.go**: High-level fetch orchestration

#### 5. Prompt System (`internal/prompt/`)

**New in v2.0 - 92% test coverage**

- **manager.go**: Load system prompt from system-prompt.md
- **builder.go**: Construct final prompt (system + XML context)

#### 6. Provider System (`internal/provider/`)

- **provider.go**: Interface definition for AI backends
- **claude/**: Claude implementation
- **codex/**: Codex implementation

**Provider interface:**

```go
type Provider interface {
    GenerateCode(ctx, req) (*CodeResponse, error)
    Name() string
}
```

**Note:** Provider instantiation now happens directly in main.go, no factory pattern.

#### 7. GitHub Operations (`internal/github/`)

- **auth.go**: GitHub App JWT token generation and installation token exchange
- **clone.go**: Repository cloning via `gh repo clone`
- **apicommit.go**: Commit via GitHub API with optional signing support
  - `CommitFiles()`: Multi-file commit via API (supports GitHub-signed commits)
  - Supports both REST and GraphQL paths
  - When `USE_COMMIT_SIGNING=true`, commits are automatically signed by GitHub
- **gh_client.go**: GitHub CLI command abstraction
- **context.go**: GitHub context struct for passing event data

**Post-Processing (`internal/github/postprocess/`)** - **NEW in v2.0**

- **processor.go**: Main post-execution logic
  - Runs after AI provider completes
  - Non-blocking (failures only log warnings)
- **branch_check.go**: Branch status detection
  - Check if branch exists remotely
  - Compare commits with base branch
  - Detect empty branches (0 commits, 0 files)
- **link_generator.go**: Generate GitHub links
  - Branch view links
  - PR creation links (with pre-filled title/body)
  - Job run links
- **comment_updater.go**: Update coordinating comment
  - Add branch links after execution
  - Add PR creation links if changes exist
  - Avoid duplicate links

**Post-Processing Flow:**
1. Check branch status (exists? has commits?)
2. Generate branch link (if has commits)
3. Generate PR link (if has changes)
4. Delete empty branch (if no commits and no files)
5. Update coordinating comment with links

#### 8. Task Store (`internal/taskstore/`)

**New in v2.0 - 100% test coverage**

- **store.go**: In-memory task storage for web UI and status tracking

#### 9. Web UI (`internal/web/`)

**New in v2.0 - 95% test coverage**

- **handler.go**: Task dashboard HTTP handlers (`/tasks`, `/tasks/{id}`)

#### 10. Configuration (`internal/config/`)

- **config.go**: Environment variable loading and validation
- Supports multiple providers (Claude, Codex)
- Validates required secrets at startup

### Project Structure (v2.0)

```
swe-agent/
├── cmd/
│   ├── main.go                          # HTTP server entry point
│   └── main_test.go                     # Integration tests
├── internal/
│   ├── config/                          # Configuration management
│   │   ├── config.go
│   │   └── config_test.go
│   ├── webhook/                         # GitHub webhook handling
│   │   ├── handler.go                   # Event handling
│   │   ├── analysis.go                  # Command extraction
│   │   ├── verify.go                    # HMAC verification
│   │   ├── types.go                     # Payload types
│   │   └── *_test.go                    # Tests (94% coverage)
│   ├── dispatcher/                      # Task queue (NEW v2.0)
│   │   ├── dispatcher.go                # Queue + retry logic
│   │   └── dispatcher_test.go           # Tests (91% coverage)
│   ├── executor/                        # Task orchestration
│   │   ├── task.go                      # Main workflow (150 lines)
│   │   ├── adapter.go                   # Provider adapter
│   │   └── *_test.go                    # Tests (87% coverage)
│   ├── github/                          # GitHub operations
│   │   ├── data/                        # GraphQL data layer (NEW v2.0)
│   │   │   ├── client.go                # GraphQL client
│   │   │   ├── fetcher.go               # Data fetching
│   │   │   ├── formatter.go             # XML formatting
│   │   │   └── *_test.go                # Tests (91% coverage)
│   │   ├── postprocess/                 # Post-execution (NEW v2.0)
│   │   │   ├── processor.go             # Main post-processing logic
│   │   │   ├── branch_check.go          # Branch status detection
│   │   │   ├── link_generator.go        # PR/branch link generation
│   │   │   ├── comment_updater.go       # Comment updates
│   │   │   └── processor_test.go        # Tests (40% coverage)
│   │   ├── auth.go                      # GitHub App auth
│   │   ├── clone.go                     # Repository cloning
│   │   ├── apicommit.go                 # API-based commit (NEW v2.0)
│   │   ├── gh_client.go                 # gh CLI abstraction
│   │   ├── context.go                   # Event context (NEW v2.0)
│   │   └── *_test.go                    # Tests (85% coverage)
│   ├── prompt/                          # Prompt building (NEW v2.0)
│   │   ├── manager.go                   # System prompt loader
│   │   ├── builder.go                   # Prompt construction
│   │   └── *_test.go                    # Tests (92% coverage)
│   ├── provider/                        # AI provider abstraction
│   │   ├── provider.go                  # Interface
│   │   ├── claude/                      # Claude implementation
│   │   └── codex/                       # Codex implementation
│   ├── taskstore/                       # Task storage (NEW v2.0)
│   │   ├── store.go                     # In-memory store
│   │   └── store_test.go                # Tests (100% coverage)
│   └── web/                             # Web UI (NEW v2.0)
│       ├── handler.go                   # Dashboard handlers
│       └── handler_test.go              # Tests (95% coverage)
├── templates/                           # HTML templates (NEW v2.0)
│   ├── tasks_list.html
│   └── task_detail.html
├── system-prompt.md                     # System prompt (NEW v2.0)
├── Dockerfile                           # Container build
├── .env.example                         # Environment template
└── CLAUDE.md                            # This file
```

## Important Implementation Notes

### v2.0 Architecture Improvements

**Code Reduction:** The codebase was reduced by 59% (3,150 → 1,300 lines) by:
- Simplifying executor from 1,400 to 150 lines
- Creating dedicated data layer for GitHub operations
- Extracting prompt building into separate package
- Removing redundant files and abstractions

**Key Changes:**
- **No factory pattern**: Providers instantiated directly in main.go
- **GraphQL data fetching**: New `internal/github/data` package replaces REST API calls
- **Prompt builder**: System prompt loaded from `system-prompt.md` file
- **API-based commits**: Use GitHub API for commits instead of local git
- **Task queue**: Dispatcher with exponential backoff and retry logic
- **Post-processing**: Automatic branch/PR link generation after execution
- **Commit signing**: Optional GitHub-signed commits via API

### Commit Signing Support

**Two commit modes available:**

1. **Local Git (default)**
   - Uses local git commands (`git add`, `git commit`, `git push`)
   - Fast and familiar workflow
   - No automatic signing
   - Requires git tools in PATH

2. **GitHub API Signing** (`USE_COMMIT_SIGNING=true`)
   - Commits via GitHub API (REST or GraphQL)
   - Automatic signing by GitHub
   - No local git commands needed
   - More secure (no local git execution)
   
**Configuration:**
```bash
# Enable commit signing
USE_COMMIT_SIGNING=true
```

**Tool implications:**
- When signing enabled: `git_*` tools disabled, `github_push_files` enabled
- When signing disabled: `git_*` tools enabled, `github_create_or_update_file` enabled

**Implementation:**
- `internal/github/apicommit.go::CommitFiles()` handles API commits
- Supports both REST (manual tree creation) and GraphQL (automatic signing)
- GraphQL path uses `createCommitOnBranch` mutation with `expectedHeadOid`

### Post-Processing System

**Runs automatically after AI provider execution:**

1. **Branch Status Check**
   - Detect if branch exists remotely
   - Compare commits with base branch
   - Count files changed

2. **Link Generation**
   - Branch view link: `https://github.com/owner/repo/tree/branch`
   - PR creation link: Pre-filled title/body via `quick_pull=1`

3. **Empty Branch Cleanup**
   - Delete branch if 0 commits AND 0 files changed
   - Prevents clutter from failed executions

4. **Comment Update**
   - Add generated links to coordinating comment
   - Avoid duplicate links (idempotent)

**Non-blocking design:**
- Post-processing failures only log warnings
- Main execution flow not affected
- User still gets PR/branch links when possible

**Implementation:**
- `internal/github/postprocess/processor.go::Process()`
- Called at end of `internal/executor/task.go::Execute()`

### Provider Pattern Design

The provider system uses interface polymorphism for extensibility:

```go
// Adding a new provider requires:
// 1. Implement Provider interface in internal/provider/<name>/
// 2. Add provider instantiation in cmd/main.go
// 3. Add config fields in internal/config/config.go
// 4. No changes to executor, handler, or dispatcher needed
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

This project delegates some operations to CLI tools:

- **`gh` CLI**: GitHub operations (clone, push, auth)
- **`codex` CLI**: Codex AI provider (when PROVIDER=codex)

Ensure the `gh` CLI is installed and authenticated. The `codex` CLI is only required if using the Codex provider.

### System Prompt Customization (v2.0)

The system prompt is loaded from `system-prompt.md` in the repository root. This file:
- Contains the core instructions for the AI provider
- Is loaded by `internal/prompt/manager.go` at runtime
- Can be customized per repository for domain-specific guidance
- Falls back to a minimal default if the file is not found

To customize AI behavior, edit `system-prompt.md` directly.

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

- **Target**: 85%+ coverage overall (achieved in v2.0)
- **Critical code**: 100% coverage for security-critical code (webhook verification, auth)
- **Test files**: Located alongside implementation: `file.go` → `file_test.go`
- **Test style**: Use table-driven tests for multiple scenarios

**Current Coverage (v2.0):**
- executor: 87.3%
- github/data: 91.2%
- taskstore: 100.0%
- github: 85.0%
- webhook: 94.0%
- web: 95.2%
- prompt: 92.3%
- dispatcher: 91.6%

## Multi-Provider Support

Current providers:

- **Claude**: Via `lancekrogers/claude-code-go` SDK
- **Codex**: Via Codex provider implementation

Provider selection via environment variable:

```bash
PROVIDER=claude  # or "codex"
CLAUDE_API_KEY=sk-ant-xxx
CLAUDE_MODEL=claude-sonnet-4-5-20250929
```
