[English](README.md) | [简体中文](README.zh-CN.md)

# SWE-Agent - Software Engineering Agent

[![Go Version](https://img.shields.io/badge/Go-1.25%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Test Coverage](https://img.shields.io/badge/coverage-85.2%25-brightgreen)](#-testing)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![GitHub](https://img.shields.io/badge/GitHub-cexll%2Fswe-181717?logo=github)](https://github.com/cexll/swe)

GitHub App webhook service that triggers AI to automatically complete code modification tasks via `/code` commands.

> 🎯 **Core Philosophy**: Empower developers with AI, making code changes as simple as leaving comments.

## 📖 Table of Contents

- [Features](#-features)
- [Quick Start](#quick-start)
- [Usage](#usage)
- [Architecture](#️-architecture)
- [Recent Updates](#-recent-updates)
- [Testing](#-testing)
- [Development](#-development)
- [Deployment](#-deployment)
- [Roadmap](#️-roadmap)

## ✨ Features

### 🚀 AI-First Architecture (v2.1)
- 🧠 **GPT-5 Prompt System** - XML-structured prompts with decision trees and best practices
- 🛠️ **Dynamic MCP Configuration** - Runtime MCP server configuration for Claude/Codex providers
- 🔧 **Coordinating Comment System** - Single comment tracking to prevent comment spam
- 📊 **GraphQL Pagination** - Cursor-based pagination for large PRs (100+ files/comments)

### Core Features
- 🤖 **Multi-AI Provider Support** - Claude Code and Codex, easily extensible
- 🔐 **Security Verification** - GitHub webhook signature verification (HMAC SHA-256)
- ⚡ **Async Processing** - Immediate webhook response, background task execution
- 📦 **Smart Change Detection** - Auto-detect filesystem changes regardless of how AI modifies files
- 🎯 **Configurable Trigger Words** - Default `/code`, customizable
- 🎨 **Clean Architecture** - Provider interface abstraction, GitHub operations abstraction
- ✅ **High Test Coverage** - 85.2% unit test coverage
- 🛡️ **Safe Execution** - Command runner with injection prevention, sandboxed execution
- 📊 **Progress Tracking** - Comment tracker with real-time task status updates
- 🖥️ **Task Dashboard UI** - Built-in `/tasks` web view for queue status and logs
- ⏱️ **Timeout Protection** - 10-minute timeout prevents task hang-ups
- 🔀 **Multi-PR Workflow** - Automatically split large changes into multiple logical PRs
- 🧠 **Smart PR Splitting** - Intelligent grouping by file type and dependency relationships
- 🧵 **Review Comment Triggers** - Support for both Issue comments and PR Review inline comments
- 🔁 **Reliable Task Queue** - Bounded worker pool + exponential backoff auto-retry
- 🔒 **PR Serial Execution** - Commands for the same PR queued serially to avoid conflicts
- 🔗 **Post-Processing** - Automatic branch/PR link generation after execution
- ✍️ **Commit Signing** - Optional GitHub-signed commits via API
- 🧹 **Empty Branch Cleanup** - Auto-delete branches with no commits

## 🎉 Recent Updates

### v2.1 - Architecture Revolution (January 2025)

#### 🎉 AI-First Redesign
- ✅ **GPT-5 Prompt System**: XML-structured system prompt as Go constant with decision trees
- ✅ **Dynamic MCP Configuration**: Runtime MCP server configuration with environment isolation
- ✅ **Coordinating Comment System**: Single comment tracking prevents comment spam
- ✅ **GraphQL Pagination**: Cursor-based pagination for large PRs (100+ files/comments)
- ✅ **Code Reduction**: 5,260 lines deleted (4,750 net reduction) while maintaining functionality
- ✅ **100% Test Pass Rate**: All 18 test packages passing

#### 🔧 Technical Improvements
- **Prompt Template**: Moved to Go constant `internal/prompt/template.go` with text/template syntax
- **MCP Integration**: 39 GitHub MCP tools with dynamic configuration generation
- **Architecture Simplification**: Eliminated factory patterns, direct provider instantiation
- **Performance**: 99% of PRs use single GraphQL query; only large PRs trigger pagination

### v2.0 - Major Architecture Overhaul (October 2025)

#### 🎉 Modular Architecture
- ✅ **59% Code Reduction**: 3,150 → 1,300 lines while improving test coverage to 85.2%
- ✅ **Data Layer**: New `internal/github/data/` package for GraphQL operations (91% coverage)
- ✅ **Prompt Builder**: Template-based prompt system with variable substitution
- ✅ **Task Queue**: Bounded dispatcher with exponential backoff retry
- ✅ **Web UI**: Built-in task dashboard at `/tasks` endpoint
- ✅ **API Commits**: GitHub API-based commits with optional signing

#### 🧪 Testing Excellence
- ✅ **18 Test Packages**: All passing with comprehensive coverage
- ✅ **Modular Testing**: Each component has dedicated test coverage
- ✅ **Integration Tests**: End-to-end workflow validation

## 📊 Project Stats

| Metric             | Value                                        |
| ------------------ | -------------------------------------------- |
| **Lines of Code**  | ~1,300 core lines (59% reduction from 3,150) |
| **Test Coverage**  | 85.2% overall (18 test packages passing) |
| **Key Packages**   | toolconfig 95.7%, web 95.2%, prompt 92.3% |
| **Binary Size**    | ~12MB single binary                          |
| **Dependencies**   | Minimal - Go 1.25+, Claude/Codex, gh CLI     |
| **Performance**    | Startup ~100ms, Memory ~60MB                 |
| **GraphQL Pagination** | 99% of PRs use single query, large PRs use cursor pagination |

## Quick Start

### Prerequisites

- Go 1.25+
- [Claude Code CLI](https://github.com/anthropics/claude-code) or [Codex](https://github.com/codex-rs/codex)
- [GitHub CLI](https://cli.github.com/)
- API Key (Anthropic or OpenAI)
- [uvx](https://github.com/astral-sh/uvx) (for MCP Git server support, optional)

### Installation

```bash
# 1. Clone the repository
git clone https://github.com/cexll/swe-agent.git
cd swe-agent

# 2. Install dependencies
go mod download

# 3. Copy environment template
cp .env.example .env

# 4. Edit .env and fill in your configuration
# GITHUB_APP_ID=your-app-id
# GITHUB_PRIVATE_KEY="your-private-key"
# GITHUB_WEBHOOK_SECRET=your-webhook-secret
# PROVIDER=claude  # or codex
```

### Environment Variables

```bash
# GitHub App Configuration
GITHUB_APP_ID=123456
GITHUB_PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----\n..."
GITHUB_WEBHOOK_SECRET=your-webhook-secret

# AI Provider Configuration (choose one)
# Option 1: Claude (Recommended for v2.1)
PROVIDER=claude
ANTHROPIC_API_KEY=sk-ant-xxx
CLAUDE_MODEL=claude-sonnet-4-5-20250929

# Option 2: Codex
# PROVIDER=codex
# CODEX_MODEL=gpt-5-codex
# OPENAI_API_KEY=your-key  # Optional
# OPENAI_BASE_URL=http://...  # Optional

# Required for MCP features (v2.1+)
GITHUB_TOKEN=github_pat_xxx  # For GitHub MCP tools

# Optional Configuration
TRIGGER_KEYWORD=/code
PORT=8000
DISPATCHER_WORKERS=4
DISPATCHER_QUEUE_SIZE=16
DISPATCHER_MAX_ATTEMPTS=3
DISPATCHER_RETRY_SECONDS=15
DISPATCHER_RETRY_MAX_SECONDS=300
DISPATCHER_BACKOFF_MULTIPLIER=2
# SWE_AGENT_GIT_NAME=swe-agent[bot]
# SWE_AGENT_GIT_EMAIL=123456+swe-agent[bot]@users.noreply.github.com

# Commit Signing (optional)
# USE_COMMIT_SIGNING=false  # When true, use GitHub API signing

# Enable GitHub File Operations via MCP (optional)
# ENABLE_GITHUB_FILE_OPS_MCP=false

# Debugging (optional)
# DEBUG_CLAUDE_PARSING=true
# DEBUG_GIT_DETECTION=true
# DEBUG_MCP_CONFIG=true  # Show MCP configuration generation

# Permission overrides (optional; use with care)
# ALLOW_ALL_USERS=false        # when true, bypass installer-only check
# PERMISSION_MODE=open         # alternative flag to allow all users
```

> 🧵 **Queue Configuration Explanation**
> - `DISPATCHER_WORKERS`: Number of concurrent workers (default 4)
> - `DISPATCHER_QUEUE_SIZE`: Bounded task queue capacity, returns 503 when exceeded
> - `DISPATCHER_MAX_ATTEMPTS`: Maximum execution attempts per task (including initial)
> - `DISPATCHER_RETRY_SECONDS`: Initial retry delay (seconds)
> - `DISPATCHER_RETRY_MAX_SECONDS`: Maximum delay for exponential backoff (seconds)
> - `DISPATCHER_BACKOFF_MULTIPLIER`: Delay multiplier for each retry (default 2)

### Local Development

```bash
# Load environment variables
source .env  # or use export for each variable

# Run the service
go run cmd/main.go
```

After the service starts, visit:

- 🏠 Service Info: http://localhost:8000/
- 📋 Task Dashboard: http://localhost:8000/tasks
- ❤️ Health Check: http://localhost:8000/health
- 🔗 Webhook: http://localhost:8000/webhook

## Usage

### 1. Configure GitHub App

1. **Create GitHub App**: https://github.com/settings/apps/new
2. **Permission Settings**:
   - Repository permissions:
     - ✅ Contents: Read & Write
     - ✅ Issues: Read & Write
     - ✅ Pull requests: Read & Write
   - Subscribe to events:
     - ✅ Issue comments
      - ✅ Pull request review comments
3. **Webhook Settings**:
   - URL: `https://your-domain.com/webhook`
   - Secret: Generate a random key
   - Content type: `application/json`
4. **Install to Repository**

### 2. Trigger in Issue/PR Comments (including Review inline comments)

Comment in any Issue or PR:

```
/code fix the typo in README.md
```

```
/code add error handling to the main function
```

```
/code refactor the database connection code
```

You can also trigger on specific lines in code review:

```
/code tighten error handling here
```

#### Multi-turn (analysis → implementation)

You can split the workflow into analysis and implementation using separate trigger comments:

```
/code Please analyze the approach: list steps, risks, and tests.
```

Then follow up to implement:

```
/code Proceed to implement now. Return full files using <file path=...><content>...</content></file> blocks and push.
```

Only the latest comment containing the trigger keyword is treated as the authoritative instruction. Other comments are context only.

### 3. SWE-Agent Automatically Executes

SWE-Agent will automatically complete the following workflow:

1. ✅ **Clone Repository** - Download latest code to temporary directory
2. ✅ **AI Generation** - Call AI provider to generate or directly modify files
3. ✅ **Detect Changes** - Use `git status` to detect actual file changes
4. ✅ **Commit** - Commit to new branch `swe-agent/<issue-number>-<timestamp>`
5. ✅ **Push** - Push to remote repository
6. ✅ **Reply Comment** - Provide PR creation link

### 4. View Results

SWE-Agent will automatically reply under the original comment:

```markdown
### ✅ Task Completed Successfully

**Summary:** Fixed typo in README.md

**Modified Files:** (1)

- `README.md`

**Next Step:**
[🚀 Click here to create Pull Request](https://github.com/owner/repo/compare/main...swe-agent/123-1234567890?expand=1&quick_pull=1&title=Fix%20typo&body=Fix%20typo)

---

_Generated by SWE-Agent_
```

## 🔄 Version History

### v0.3.0 - Multi-PR Workflow (2025-10)

- **Multi-PR Workflow Orchestration** - Automatically split large changes into multiple logical PRs
- **Smart PR Splitter** - Intelligent grouping by file type, dependencies, and complexity
- **Split Plan Display** - Real-time display of split plan and progress in comments
- **Makefile Build System** - Unified build, test, and deployment commands
- **Enhanced Comment Tracking** - Support for multi-PR status display and progress updates

### v0.2.0 - Major Improvements (2025-10)

- **Filesystem Change Detection** - Auto-detect direct file modifications by AI provider
- **GitHub CLI Abstraction Layer** - `gh_client.go` unifies all gh command execution
- **Safe Command Executor** - `command_runner.go` prevents command injection attacks
- **Comment State Management** - `comment_state.go` enum states (Pending/InProgress/Completed/Failed)
- **Comment Tracker** - `comment_tracker.go` real-time GitHub comment progress updates

## 🏗️ Architecture

### Directory Structure (v2.1)

```
swe-agent/
├── cmd/
│   ├── main.go                          # HTTP server entry point
│   ├── mcp-comment-server/              # MCP Comment Server (v2.1)
│   │   └── main.go                      # Go-based MCP server for comment updates
│   └── main_test.go                     # Integration tests
├── internal/
│   ├── config/                          # Configuration management
│   │   ├── config.go
│   │   └── config_test.go               # Configuration tests (88.4%)
│   ├── webhook/                         # GitHub webhook handling
│   │   ├── handler.go                   # Event handling
│   │   ├── verify.go                    # HMAC verification
│   │   ├── analysis.go                  # Command extraction
│   │   ├── types.go                     # Payload types
│   │   └── *_test.go                    # Tests (94.0%)
│   ├── dispatcher/                      # Task queue (v2.0)
│   │   ├── dispatcher.go                # Queue + retry logic
│   │   └── dispatcher_test.go           # Tests (91.6%)
│   ├── executor/                        # Task orchestration
│   │   ├── task.go                      # Main workflow (150 lines)
│   │   ├── adapter.go                   # Provider adapter
│   │   └── *_test.go                    # Tests (87.3%)
│   ├── github/                          # GitHub operations
│   │   ├── data/                        # GraphQL data layer (v2.0)
│   │   │   ├── client.go                # GraphQL client
│   │   │   ├── fetcher.go               # Data fetching with pagination
│   │   │   ├── formatter.go             # XML formatting
│   │   │   └── *_test.go                # Tests (93.3%)
│   │   ├── auth.go                      # GitHub App auth
│   │   ├── clone.go                     # Repository cloning
│   │   ├── apicommit.go                 # API-based commit (v2.0)
│   │   ├── gh_client.go                 # gh CLI abstraction
│   │   ├── context.go                   # Event context (v2.0)
│   │   └── *_test.go                    # Tests (85.4%)
│   ├── prompt/                          # Prompt building (v2.0)
│   │   ├── template.go                  # System prompt as Go constant (732 lines)
│   │   ├── builder.go                   # Prompt construction
│   │   └── *_test.go                    # Tests (92.3%)
│   ├── provider/                        # AI provider abstraction
│   │   ├── provider.go                  # Interface
│   │   ├── claude/                      # Claude implementation
│   │   │   ├── claude.go                # MCP config generation (v2.1)
│   │   │   └── claude_test.go
│   │   └── codex/                       # Codex implementation
│   │       ├── codex.go                 # MCP config generation (v2.1)
│   │       └── codex_test.go
│   ├── taskstore/                       # Task storage (v2.0)
│   │   ├── store.go                     # In-memory store
│   │   └── store_test.go                # Tests (100.0%)
│   ├── web/                             # Web UI (v2.0)
│   │   ├── handler.go                   # Dashboard handlers
│   │   └── handler_test.go              # Tests (95.2%)
│   ├── modes/                           # Command processing
│   │   └── command/                     # Command mode logic
│   │       ├── mode.go                  # Mode implementation
│   │       └── mode_test.go
│   ├── toolconfig/                      # Tool configuration (v2.1)
│   │   ├── builder.go                   # Build allowed/disallowed tools
│   │   └── builder_test.go              # Tests (95.7%)
│   └── github/                          # GitHub operations
│       ├── operations/                  # GitHub operation abstractions
│       │   └── git/                     # Git operations
│       └── comment/                     # Comment operations
├── templates/                           # HTML templates (v2.0)
│   ├── tasks_list.html
│   └── task_detail.html
├── docs/                                # Documentation (v2.1)
│   └── gpt5_prompting_guide.md          # GPT-5 best practices (542 lines)
├── Dockerfile                           # Container build
├── docker-entrypoint.sh                 # Docker entrypoint (v2.1)
├── Makefile                             # Build automation
├── .env.example                         # Environment template
├── CLAUDE.md                            # Development guide (v2.1)
└── README.md                            # Project documentation
```

### Architecture Highlights (v2.1)

#### 1. AI-First Design - GPT-5 Best Practices

**System Prompt as Go Constant** (`internal/prompt/template.go`):
```go
// 732 lines of XML-structured AI operational guidelines
const SystemPromptTemplate = `
<system_identity>
## Who You Are
You are **SWE Agent**, an autonomous software engineering agent...
</system_identity>

<tool_constraints>
## CRITICAL: Tool Usage Rules
- Use git CLI for all git operations
- Use gh CLI for GitHub operations  
- Use coordinating comment for ALL progress updates
</tool_constraints>

<gpt5_optimizations>
## GPT-5 Performance Optimization
- Context gathering strategy: 5-8 tool calls for initial discovery
- Self-reflection for quality: 5-7 category rubric
- Persistence and autonomy: Keep going until problem solved
</gpt5_optimizations>
`
```

#### 2. Dynamic MCP Configuration

**Runtime Configuration Generation**:
```go
// Claude Provider: Generate JSON config via --mcp-config
func buildMCPConfig(ctx *Context) (string, error) {
    config := map[string]MCPServer{
        "github": {
            "Type": "url",
            "URL": "https://api.githubcopilot.com/mcp",
            "Headers": map[string]string{"Authorization": "Bearer " + ctx.GitHubToken},
        },
        "comment_updater": {
            "Type": "stdio",
            "Command": "mcp-comment-server",
            "Args": []string{"--comment-id", ctx.CommentID},
        },
    }
    return json.Marshal(config)
}
```

#### 3. Coordinating Comment System

**Single Comment Tracking**:
- **Tool 1**: `mcp__comment_updater__update_claude_comment` (MANDATORY for progress)
- **Tool 2**: `mcp__github__add_issue_comment` (OPTIONAL for detailed analysis)
- **Decision Rules**: Clear guidance in prompt template for when to use each tool
- **Benefits**: Clean UI, unified progress tracking, no comment spam

#### 4. GraphQL Pagination System

**Cursor-Based Pagination** (`internal/github/data/fetcher.go`):
```go
type PageInfo struct {
    HasNextPage bool   `json:"hasNextPage"`
    EndCursor   string `json:"endCursor"`
}

type FilesConnection struct {
    Nodes    []File `json:"nodes"`
    PageInfo `json:"pageInfo"`
}

func fetchAllRemainingFiles(ctx context.Context, repo, owner, endCursor string) ([]File, error) {
    // Max 50 iterations (5,000 files max)
    // 99% of PRs use single query
}
```

#### 5. Simplified Architecture Flow

```
GitHub Webhook → Handler → Dispatcher → Executor
      ↓
GitHub Data Layer (GraphQL) → Prompt Builder → Provider
      ↓
AI (Claude/Codex with MCP tools) → Commit API → Push
      ↓
Coordinating Comment Updates → Post-Processing
```

### Core Components (v2.1)

| Component         | Responsibility                                  | Test Coverage |
| ----------------- | ----------------------------------------------- | ------------- |
| Prompt System      | AI operational guidelines (732-line constant)   | 92.3%         |
| Tool Config        | Build allowed/disallowed tools (MCP aware)      | 95.7%         |
| GitHub Data Layer  | GraphQL operations with pagination               | 93.3%         |
| Dispatcher        | Bounded task queue with exponential backoff      | 91.6%         |
| Web UI            | Task dashboard at `/tasks` endpoint              | 95.2%         |
| Task Store        | In-memory task storage                          | 100.0%        |
| MCP Comment Server| Go-based MCP server for comment updates         | 39.5%         |
| Providers         | Claude/Codex with dynamic MCP config            | 83.2%/85.3%   |
| Webhook Handler   | Event processing and command extraction         | 94.0%         |
| Executor          | Simplified orchestration (150 lines)            | 87.3%         |

## 🧪 Testing

### Test Coverage

Overall: **85.2%** coverage across all modules (18 test packages passing)

| Module                | Coverage |
|-----------------------|----------|
| taskstore             | 100.0%   |
| toolconfig            | 95.7%    |
| web                   | 95.2%    |
| github/data           | 93.3%    |
| prompt                | 92.3%    |
| dispatcher            | 91.6%    |
| webhook               | 94.0%    |
| github/comment        | 73.9%    |
| github/operations/git | 82.4%    |
| executor              | 87.3%    |
| github                | 85.4%    |
| codex provider        | 85.3%    |
| claude provider       | 83.2%    |
| config                | 88.4%    |
| modes/command         | 84.6%    |
| modes                 | 90.9%    |
| cmd                   | 93.5%    |

### Test Highlights (v2.1)

- **GraphQL Pagination Tests**: Comprehensive cursor-based pagination testing
- **MCP Configuration Tests**: Dynamic config generation validation
- **Tool Configuration Tests**: 18 test cases for tool selection logic
- **Integration Tests**: End-to-end workflow validation
- **Mock Utilities**: Helpers for uvx availability, temp HOME, JSON/TOML validation

### Run Tests

```bash
# Run all tests with coverage
go test ./... -cover

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## 💻 Development

> 💡 **Developer Tip**: Check [CLAUDE.md](./CLAUDE.md) for complete development guide, including architecture, testing strategies, and code conventions.

### Build

```bash
# Using Makefile (recommended)
make build                    # Build binary
make run                      # Run application
make test                     # Run all tests
make test-coverage           # Run tests and generate coverage report
make test-coverage-html      # Generate HTML coverage report
make fmt                     # Format code
make lint                    # Code check
make check                   # Run all checks (format, check, test)
make clean                   # Clean build files
make all                     # Complete build process

# Manual build
go build -o swe-agent cmd/main.go

# Run
./swe-agent
```

### Code Formatting

```bash
# Using Makefile (recommended)
make fmt                      # Format code
make vet                      # Code check
make lint                     # Full check (includes format check)
make tidy                     # Tidy dependencies

# Manual operations
go fmt ./...                  # Format code
go vet ./...                  # Code check
go mod tidy                   # Tidy dependencies
```

### Adding a New AI Provider

1. Create directory in `internal/provider/<name>/`
2. Implement `Provider` interface:
   ```go
   type Provider interface {
       GenerateCode(ctx, req) (*CodeResponse, error)
       Name() string
   }
   ```
3. Provider can choose:
   - Return `Files` list (Executor will apply these files)
   - Directly modify files in `req.RepoPath` (Executor will auto-detect)
4. Add case in `factory.go`
5. Add test file
6. Update documentation

## 🐳 Deployment

### Docker Deployment (v2.1)

**Dynamic MCP Configuration**: The Docker image uses dynamic MCP configuration generated at runtime by providers.

```bash
# Using Makefile (recommended)
make docker-build           # Build Docker image
make docker-run             # Run Docker container (requires .env file)
make docker-stop            # Stop and remove container
make docker-logs            # View container logs

# Manual Docker commands
docker build -t swe-agent .

# Run container with required environment variables
docker run -d \
  -p 8000:8000 \
  -e GITHUB_APP_ID=123456 \
  -e GITHUB_PRIVATE_KEY="$(cat private-key.pem)" \
  -e GITHUB_WEBHOOK_SECRET=secret \
  -e GITHUB_TOKEN=github_pat_xxx \
  -e ANTHROPIC_API_KEY=sk-ant-xxx \
  -e PROVIDER=claude \
  --name swe-agent \
  swe-agent
```

### Docker Compose (v2.1)

```yaml
version: "3.8"

services:
  swe-agent:
    build: .
    ports:
      - "8000:8000"
    environment:
      - GITHUB_APP_ID=${GITHUB_APP_ID}
      - GITHUB_PRIVATE_KEY=${GITHUB_PRIVATE_KEY}
      - GITHUB_WEBHOOK_SECRET=${GITHUB_WEBHOOK_SECRET}
      - GITHUB_TOKEN=${GITHUB_TOKEN}  # Required for MCP
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - PROVIDER=claude
      - CLAUDE_MODEL=claude-sonnet-4-5-20250929
      - TRIGGER_KEYWORD=/code
    restart: unless-stopped
```

### MCP Configuration in Docker

**Claude Provider**:
- Generates MCP config as JSON via `--mcp-config` CLI parameter
- Configuration is passed dynamically for each execution
- Supports GitHub HTTP MCP, Git MCP, and Comment Updater MCP

**Codex Provider**:
- Generates `~/.codex/config.toml` at runtime before each execution
- Configuration includes MCP servers with environment variables

**Debug Logging**:
```bash
# Enable detailed MCP config logging
DEBUG_MCP_CONFIG=true docker run -d swe-agent
```

## 📦 Dependencies

- **Go 1.25+** - Build and runtime environment
- **Codex CLI** / **Claude Code CLI** - AI code generation
- **GitHub CLI (`gh`)** - Git operations
- **Gorilla Mux** - HTTP routing

### AI Provider Support

Currently supported AI providers:

- **Codex** (Recommended) - Requires Codex CLI, optional `OPENAI_API_KEY`
- **Claude** (Anthropic) - Requires `ANTHROPIC_API_KEY`

Switch via environment variable `PROVIDER=codex` or `PROVIDER=claude`.

## ⚡ Current Capabilities

### ✅ v2.1 Implemented

**AI-First Architecture**:
- ✅ **GPT-5 Prompt System**: 732-line XML-structured system prompt as Go constant
- ✅ **Dynamic MCP Configuration**: Runtime MCP server configuration with environment isolation
- ✅ **Coordinating Comment System**: Single comment tracking prevents comment spam
- ✅ **GraphQL Pagination**: Cursor-based pagination for large PRs (100+ files/comments)
- ✅ **AI Autonomy**: Full GitHub management capabilities via 39 MCP tools

**Core Features**:
- ✅ Respond to `/code` commands in Issue and PR Review comments
- ✅ HMAC SHA-256 webhook signature verification (anti-forgery)
- ✅ Multi-Provider support: Claude + Codex with dynamic MCP configuration
- ✅ **Smart file change detection** (via git status)
- ✅ **Multi-PR workflow** (auto-split large changes)
- ✅ **Smart PR splitter** (group by file type and complexity)
- ✅ **Timeout protection** (10-minute timeout)
- ✅ **Task Dashboard UI** at `/tasks` endpoint
- ✅ **Reliable Task Queue** with exponential backoff retry
- ✅ **API-based commits** with optional GitHub signing
- ✅ Auto clone, modify, commit, push to new branch
- ✅ **Post-processing system** (auto branch/PR links, empty branch cleanup)
- ✅ Docker deployment with dynamic MCP configuration
- ✅ Auto-notify errors to GitHub comments
- ✅ **85.2% test coverage** (18 test packages passing)
- ✅ Bot comment filtering (prevent infinite loops)

**MCP Integration**:
- ✅ **GitHub HTTP MCP**: 39 GitHub tools (issues, PRs, labels, milestones, search)
- ✅ **Git MCP**: Git operations via uvx when commit signing disabled
- ✅ **Comment Updater MCP**: Custom Go-based server for coordinating comments
- ✅ **Sequential Thinking MCP**: Deep reasoning for complex problems
- ✅ **Fetch MCP**: Web content fetching for research tasks
- ✅ **Environment Isolation**: Each MCP server has isolated environment scope

### ⚠️ Current Limitations

**Execution Layer Limitations**:
- ⚠️ Task queue is in-memory implementation, queued tasks lost on service restart
- ⚠️ No global rate limiting / quota management yet
- ⚠️ Missing visual task panel and scheduler monitoring

**Quality Assurance Gaps**:
- ⚠️ No automatic test execution after code generation
- ⚠️ Missing lint/format/compile verification
- ⚠️ No security scanning or vulnerability detection
- ⚠️ Generated code pushed without validation

**Interaction & Collaboration Gaps**:
- ⚠️ No requirement clarification (AI doesn't ask questions)
- ⚠️ No multi-turn iteration support (no conversation context)
- ⚠️ Missing real-time progress reporting
- ⚠️ Single-shot execution without design confirmation

**Context & Understanding Gaps**:
- ⚠️ Doesn't understand codebase architecture
- ⚠️ No analysis of historical commits or evolution
- ⚠️ Missing project knowledge base indexing
- ⚠️ Cannot learn from similar issues/PRs

## 🗺️ Roadmap

### v2.2 - Enhanced AI Capabilities (Q2 2025)

**Quality Assurance Layer**:
- [ ] **Automatic test execution** - Run project tests after code generation
- [ ] **Lint and format checks** - Auto-run `go vet`, `gofmt`, `golint`
- [ ] **Compilation verification** - Ensure code compiles before push
- [ ] **Security scanning** - Basic vulnerability and sensitive data detection
- [ ] **Test failure handling** - Auto-fix or rollback when tests fail

**Enhanced MCP Integration**:
- [ ] **Repository Management MCP** - Create, clone, manage multiple repositories
- [ ] **CI/CD MCP Tools** - Trigger builds, monitor test results
- [ ] **Dependency Management MCP** - Auto-add/upgrade packages
- [ ] **Performance Analysis MCP** - Run benchmarks, identify bottlenecks

**Interaction & Collaboration Layer**:
- [ ] **Requirement clarification** - AI asks questions when unclear
- [ ] **Multi-turn collaboration** - Support conversation context and follow-ups
- [ ] **Design confirmation** - Send design draft before implementation
- [ ] **Progress reporting** - Enhanced real-time status updates

### v3.0 - Enterprise & Production Ready (2026)

**Enterprise Governance**:
- [ ] **Team permission management** - Role-based access control
- [ ] **Cost control** - API spend budgets and alerts
- [ ] **Audit log** - Record every action for compliance
- [ ] **Model policy center** - Configure models/providers per repo
- [ ] **Secure merge** - Draft PR / Fork sandbox workflows

**Production Infrastructure**:
- [ ] **Horizontal scaling** - Multi-worker node support
- [ ] **Queue persistence** - Redis/Database for task durability
- [ ] **Advanced rate limiting** - Repo/org/user granularity
- [ ] **Alerting pipelines** - Comprehensive monitoring and alerts
- [ ] **Webhook replay** - Manually retry failed tasks

**Advanced AI Features**:
- [ ] **Context understanding** - Parse codebase architecture and documentation
- [ ] **Knowledge base indexing** - Vector search for relevant information
- [ ] **Historical analysis** - Learn from similar issues and PRs
- [ ] **Planning & design** - Intelligent task decomposition and risk assessment

## 🔒 Security Considerations

| Item                        | Status        | Note                                     |
| --------------------------- | ------------- | ---------------------------------------- |
| Webhook signature verification | ✅ Implemented | HMAC SHA-256                             |
| Constant-time comparison    | ✅ Implemented | Prevent timing attacks                    |
| Command injection protection | ✅ Implemented | SafeCommandRunner                         |
| Timeout protection          | ✅ Implemented | 10-minute timeout                         |
| Bot comment filtering       | ✅ Implemented | Prevent infinite loops                    |
| API key management          | ⚠️ Recommended | Use environment variables or a secrets manager |
| Queue persistence           | ⚠️ Planned    | v0.6 work (external storage + replay)     |
| Rate limiting               | ❌ Pending    | v0.6 roadmap                              |
| Concurrency control         | ✅ Implemented | In-memory queue + KeyedMutex serialization |

## 🛠️ Troubleshooting

### 1. Webhook not firing

Check:
- Is the GitHub App installed correctly
- Is the webhook URL reachable
- Does the webhook secret match
- Review the GitHub App's Recent Deliveries
- If the response code is 503, the job queue is full; retry later or increase `DISPATCHER_QUEUE_SIZE`

### 2. Codex/Claude API errors

Check:
- Is the API key correct
- Is the CLI installed properly (`codex --version` or `claude --version`)
- Has the API quota been exhausted
- Is the network connection stable

### 3. Git operations failing

Check:
- Is the `gh` CLI installed and authenticated (`gh auth status`)
- Does the GitHub App have Contents write permission
- Is there a branch name conflict
- Is the network connection stable

### 4. PR not created

Possible causes:
- The AI did not modify any files (analysis-only result)
- Git detected no changes
- Push failed (permission issue)

Check the logs:
```
[Codex] Command completed in 2.5s
No file changes detected in working directory (analysis/answer only)
```

### 5. Task stuck

- Check whether the 10-minute timeout triggered
- Compare the timestamps between `[Codex] Executing` and `Command completed` in the logs
- Manually test whether the codex command works

## 🎯 Design Philosophy - Linus Style

### 1. Simple beats complex

- **Single responsibility:** Each package does exactly one thing
- **Clear naming:** `provider.Provider` instead of `AIService`
- **Shallow indentation:** Functions stay within three levels of indentation

### 2. Code with good taste

```go
// ❌ Bad taste: assume the AI output format
if len(result.Files) == 0 {
    return  // Might miss files modified directly
}

// ✅ Good taste: check the real state of the filesystem
hasChanges := detectGitChanges(workdir)
if hasChanges {
    commitAndPush()  // Detects changes no matter how the AI edits them
}
```

### 3. Eliminate special cases

```go
// ✅ Unified handling: Providers can modify files any way they want
// 1. Return Files -> Executor applies them
// 2. Modify directly -> Executor detects via git
// Both paths validated with git status, zero special branches
```

### 4. Backward compatibility

- Provider interface design leaves room for future expansion
- Configuration stays forward-compatible (new fields have defaults)
- APIs avoid breaking changes

### 5. Pragmatism

- Call CLIs directly instead of reimplementing them (stand on giants' shoulders)
- Use `gh` CLI instead of complex GitHub API libraries
- Rely on `git status` to detect changes instead of parsing AI output
- Surface errors directly to GitHub instead of burying them in logs

## 🤝 Contributing Guide

Issues and PRs welcome!

### Contribution workflow

1. Fork this repository
2. Create a feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

### Code style

- Run `go fmt`
- Follow Linus's "good taste" principles
- Keep functions under 50 lines
- Avoid deep nesting
- Add unit tests (target coverage >75%)
- Use [Conventional Commits](https://www.conventionalcommits.org/) for commit messages

## 📄 License

MIT License - see the [LICENSE](LICENSE) file

## 🙏 Acknowledgments

- [Codex](https://github.com/codex-rs/codex) - AI coding assistant
- [Claude Code](https://github.com/anthropics/claude-code) - AI coding assistant
- [GitHub CLI](https://cli.github.com/) - Git operations tool
- [Gorilla Mux](https://github.com/gorilla/mux) - HTTP router library
- Linus Torvalds - "Good taste" programming philosophy

## 📞 Contact

- **Issues**: [GitHub Issues](https://github.com/cexll/swe/issues)
- **Discussions**: [GitHub Discussions](https://github.com/cexll/swe/discussions)

---

<div align="center">

**If this project helps you, please leave a ⭐️ Star!**

Made with ❤️ by [cexll](https://github.com/cexll)

</div>