# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Tools

- **Runtime**: Go 1.25.1
- **Web Framework**: Gorilla Mux
- **Key Dependencies**:
  - `github.com/golang-jwt/jwt/v5` - GitHub App JWT authentication
  - `github.com/joho/godotenv` - Environment variable management

## v2.1 Architecture Revolution (January 2025)

**AI-First Redesign - GPT-5 Prompting Best Practices:**
- ✅ **Prompt template restructured**: 361 → 619 lines with XML-based structure (Go constant in `internal/prompt/template.go`)
- ✅ **Decision trees**: Clear flow diagrams for different task scenarios
- ✅ **Full GitHub MCP capability**: 10 → 39 tools (issues, PRs, labels, milestones, search)
- ✅ **Coordinating comment enforcement**: AI MUST use single comment for progress tracking (no duplicate comments)
- ✅ **Massive code reduction**: 5,260 lines deleted (4,750 net reduction)
- ✅ **100% test pass rate**: All 18 test packages passing
- ✅ **GraphQL pagination support**: Handles PRs with 100+ files/comments via cursor-based pagination (October 2025)

**What Changed:**
1. **Prompt System (GPT-5 Best Practices + Go text/template)**:
   - Converted system prompt to Go constant in `internal/prompt/template.go`
   - Applied structured XML tags: `<system_identity>`, `<tool_constraints>`, `<decision_tree>`, etc.
   - Integrated Go text/template for variable substitution (e.g., `{{.GitHubContext}}`)
   - Added comprehensive decision flows for task complexity analysis
   - Included standard output format templates
   - Emphasized AI autonomy and full GitHub control
   - **NEW**: Mandatory coordinating comment usage (prevents duplicate bot comments)

2. **Coordinating Comment System**:
   - **Tool 1**: `mcp__comment_updater__update_claude_comment` (MANDATORY for progress tracking)
   - **Tool 2**: `mcp__github__add_issue_comment` (OPTIONAL for detailed analysis/code review)
   - **Behavior**: AI uses Tool 1 for all task status updates, Tool 2 only for standalone content
   - **Benefits**: Clean issue/PR threads, unified progress tracking, no progress comment spam
   - **Implementation**: `cmd/mcp-comment-server/` Go MCP server + enhanced prompt with decision rules

3. **GitHub MCP Tools Expansion**:
   - **Issue Management**: create_issue, update_issue, close_issue, reopen_issue, list_issues, assign_issue
   - **PR Management**: merge_pull_request, close_pull_request, request_reviewers
   - **Labels & Milestones**: add_labels, remove_labels, create_label, create_milestone
   - **Search**: search_code, search_issues, search_repositories
   - **Repository**: list_repositories, get_repository, create_discussion

4. **GraphQL Pagination System (October 2025)**:
   - **Problem**: GitHub API limits to 100 items per query (files, comments, reviews)
   - **Solution**: Cursor-based pagination with `pageInfo { hasNextPage, endCursor }`
   - **Implementation**: `internal/github/data/fetcher.go`
     - New types: `PageInfo`, `FilesConnection`, `CommentsConnection`, `ReviewCommentsConnection`, `ReviewsConnection`
     - Helper functions: `fetchAllRemainingFiles`, `fetchAllRemainingComments`, `fetchAllRemainingReviews`, `fetchAllReviewComments`
     - Max pagination safety: 50 iterations (5,000 items max)
     - Supports nested pagination: Review comments within reviews
   - **Performance**: 99% of PRs use single query; only large PRs trigger pagination
   - **GraphQL queries updated**: All connections now include `pageInfo` fields

5. **Code Cleanup (Deleted ~5,260 Lines)**:
   - Removed unused packages: `branch/`, `validation/`, `image/`
   - Removed unused files: `apicommit.go`, `gh_client.go`, `label.go`, `retry.go`, `command_runner.go`, `templates.go`
   - Removed obsolete tests: 10+ test files
   - Simplified `clone.go`: Removed retry wrapper (direct execution)

**Key Philosophy Shift:**
- **Before**: Hardcoded workflows with limited MCP tools
- **After**: AI-autonomous workflows with full GitHub management capabilities
- **Linus Principles**: Maintained "Good Taste", "Never Break Userspace", "Pragmatism", "Simplicity"

## v2.0 Architecture Highlights

**Major Simplification (October 2025):**
- ✅ **59% code reduction**: 3,150 → 1,300 lines
- ✅ **85.2% test coverage**: Up from 67%
- ✅ **Modular architecture**: New data, prompt, dispatcher, taskstore, and web packages
- ✅ **Executor simplified**: 1,400 → 150 lines
- ✅ **All tests passing**: Production ready

**New Components:**
- `internal/github/data/` - GraphQL data layer for fetching GitHub context (91% coverage)
- `internal/prompt/` - Prompt template using Go text/template (`template.go` constant + `builder.go`) (92% coverage)
- `internal/dispatcher/` - Task queue with exponential backoff (91% coverage)
- `internal/taskstore/` - In-memory task storage (100% coverage)
- `internal/web/` - Web UI for task dashboard (95% coverage)
- `internal/github/postprocess/` - **DELETED in v2.1**: Post-execution now handled by AI via MCP

**Key Improvements:**
- **No factory pattern**: Direct provider instantiation in main.go
- **GraphQL over REST**: Efficient data fetching via GraphQL
- **API-based commits**: Use GitHub API instead of local git
- **Go text/template system**: Type-safe template with `{{}}` placeholders in `internal/prompt/template.go`

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

**MCP Configuration (v2.0.1 - Dynamic Configuration):**

The Docker image uses **dynamic MCP configuration** generated at runtime by providers:

**Claude Provider (`internal/provider/claude/claude.go`):**
- Generates MCP config as JSON via `--mcp-config` CLI parameter
- Configuration is passed dynamically for each execution
- Merges with user's `~/.claude.json` without conflicts
- Supports GitHub HTTP MCP, Git MCP, and Comment Updater MCP

**Codex Provider (`internal/provider/codex/codex.go`):**
- Generates `~/.codex/config.toml` at runtime before each execution
- Configuration includes MCP servers with environment variables
- Supports GitHub HTTP MCP, Git MCP, and Comment Updater MCP

**MCP Servers:**
- **GitHub MCP**: HTTP endpoint at `https://api.githubcopilot.com/mcp` (no Docker required)
- **Git MCP**: Uses `uvx mcp-server-git` for git operations
- **Comment Updater MCP**: Custom server (`mcp-comment-server`) for updating coordinating comments

**Environment Variable Isolation:**
- Each MCP server has its own environment scope via config's `env` field
- No global environment variable pollution
- Follows claude-code-action best practices

**Debug Logging:**
```bash
# Enable detailed MCP config logging
DEBUG_MCP_CONFIG=true go run cmd/main.go

# Logs will show:
# [Claude] Dynamic MCP config generated: 752 bytes
# [Codex] Dynamic MCP config written to ~/.codex/config.toml
```

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
  Prompt Builder (template.go constant + XML context via text/template)
      ↓
  Provider (AI code generation: Claude/Codex)
      ↓
  Commit (local git OR GitHub API signing)
      ↓
  Push (gh CLI)
      ↓
  Post-Processing (DELETED in v2.1 - AI handles via MCP)
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

**Updated in v2.1+ - Go text/template**

- **template.go**: System prompt template as Go constant with `{{.GitHubContext}}` placeholders
- **builder.go**: Parse and execute template using Go's text/template package

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
├── Dockerfile                           # Container build
├── .env.example                         # Environment template
└── CLAUDE.md                            # This file
```

**Note**: In v2.1, `system-prompt.md` was moved to `internal/prompt/template.md`, then in v2.1+ converted to a Go constant (`template.go`) using Go's text/template syntax.

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
- **Go text/template system**: Template defined as Go constant in `internal/prompt/template.go`
- **API-based commits**: Use GitHub API for commits instead of local git
- **Task queue**: Dispatcher with exponential backoff and retry logic
- **Post-processing (DELETED in v2.1)**: AI now handles branch/PR link generation via MCP
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

### System Prompt Customization (v2.1+)

The system prompt is defined as a Go constant in `internal/prompt/template.go`. This approach:
- Uses Go's `text/template` package for variable substitution (e.g., `{{.GitHubContext}}`)
- Compiled into the binary at build time (no runtime file dependencies)
- Allows template logic and placeholders for dynamic content
- Provides type-safe template data structures
- Contains core AI instructions following GPT-5 best practices
- Uses structured XML tags for clarity (`<system_identity>`, `<decision_tree>`, etc.)

**To customize AI behavior:**
1. Edit `internal/prompt/template.go` - modify the `SystemPromptTemplate` constant
2. Add new template variables in `builder.go` if needed (e.g., `data["NewField"] = value`)
3. Rebuild the binary: `go build cmd/main.go`
4. The new template will be compiled into the binary

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
