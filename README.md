[English](README.md) | [简体中文](README.zh-CN.md)

# SWE-Agent - Software Engineering Agent

[![Go Version](https://img.shields.io/badge/Go-1.25.1-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Test Coverage](https://img.shields.io/badge/coverage-93.4%25-brightgreen)](#-testing)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![GitHub](https://img.shields.io/badge/GitHub-cexll%2Fswe-181717?logo=github)](https://github.com/cexll/swe)

GitHub App webhook service that triggers AI to automatically complete code modification tasks via `/code` commands.

> 🎯 **Core Philosophy**: AI-first software engineering with full GitHub autonomy. Make code changes as simple as leaving comments.
>
> 🚀 **v2.1 Architecture Revolution**: GPT-5 best practices, MCP integration, and 59% code reduction.

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

- 🤖 **Multi-AI Provider Support** - Claude Code and Codex with dynamic MCP configuration
- 🔐 **Security Verification** - GitHub webhook signature verification (HMAC SHA-256)
- ⚡ **Async Processing** - Immediate webhook response, background task execution
- 📦 **Smart Change Detection** - Auto-detect filesystem changes regardless of how AI modifies files
- 🎯 **Configurable Trigger Words** - Default `/code`, customizable
- 🎨 **Clean Architecture** - 59% code reduction with modular design (1,300 core lines)
- ✅ **High Test Coverage** - 93.4% unit test coverage (github/data), 85%+ overall
- 🛡️ **Safe Execution** - Git and gh CLI tools with security constraints
- 📊 **Progress Tracking** - Coordinating comment system with real-time updates
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
- 📊 **GraphQL Pagination** - Handle PRs with 100+ files/comments via cursor-based pagination
- 🔄 **Cross-Repository Workflow** - AI-driven multi-repo support with zero executor changes
- 🎯 **PR Context Awareness** - Automatically updates existing PRs vs creating new ones
- 🛠️ **MCP Integration** - 39 GitHub MCP tools + coordinating comment system

## 🎉 Recent Updates

### v0.4.0 - MCP Dynamic Configuration & Enhanced Testing (Oct 2025)

#### 🎉 New Features

- ✅ **Dynamic MCP Configuration**: Runtime MCP server configuration for Claude and Codex providers
- ✅ **MCP Comment Server**: Custom Go-based MCP server for GitHub comment updates
- ✅ **Review Comment Triggers**: `/code` supports Issue comments and PR Review inline comments
- ✅ **Reliable Task Queue**: Dispatcher with bounded queue, worker pool, and exponential backoff
- ✅ **PR Serial Execution**: Tasks within same repo/PR queued to avoid conflicts

#### 🧪 Testing Improvements

- ✅ **Test Coverage**: Achieved **84.7%** overall coverage (up from 70.5%)
- ✅ **17 New Unit Tests**: Comprehensive coverage for MCP configuration
- ✅ **Test Utilities**: Mock helpers for uvx availability, temp HOME, JSON/TOML validation

## 📊 Project Stats

| Metric             | Value                                        |
| ------------------ | -------------------------------------------- |
| **Lines of Code**  | ~1,300 core lines (59% reduction from 3,150) |
| **Test Coverage**  | 84.7% (claude 83.2%, codex 85.3%, executor 85.5%) |
| **Test Files**     | 32 test files, 300+ test functions           |
| **Binary Size**    | ~12MB single binary                          |
| **Dependencies**   | Minimal - Go 1.25+, Codex/Claude, gh CLI     |
| **Performance**    | Startup ~100ms, Memory ~60MB                 |

## Quick Start

### Prerequisites

- Go 1.25.1+
- [Claude Code CLI](https://github.com/anthropics/claude-code) or [Codex](https://github.com/codex-rs/codex)
- [GitHub CLI](https://cli.github.com/)
- API Key (Anthropic or OpenAI)

### Installation

```bash
# 1. Clone the repository
git clone git@github.com:cexll/swe.git
cd swe

# 2. Install dependencies
go mod download

# 3. Copy environment template
cp .env.example .env

# 4. Edit .env and fill in your configuration
# GITHUB_APP_ID=your-app-id
# GITHUB_PRIVATE_KEY="your-private-key"
# GITHUB_WEBHOOK_SECRET=your-webhook-secret
# PROVIDER=codex  # or claude
```

### Environment Variables

```bash
# GitHub App Configuration
GITHUB_APP_ID=123456
GITHUB_PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----\n..."
GITHUB_WEBHOOK_SECRET=your-webhook-secret

# AI Provider Configuration (choose one)
# Option 1: Codex (Recommended)
PROVIDER=codex
CODEX_MODEL=gpt-5-codex
# OPENAI_API_KEY=your-key  # Optional
# OPENAI_BASE_URL=http://...  # Optional

# Option 2: Claude
# PROVIDER=claude
# ANTHROPIC_API_KEY=sk-ant-xxx
# CLAUDE_MODEL=claude-sonnet-4-5-20250929

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

# Debugging (optional)
# DEBUG_CLAUDE_PARSING=true
# DEBUG_GIT_DETECTION=true

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

### Directory Structure

```
swe/
├── cmd/
│   └── main.go                          # HTTP server entry point
├── internal/
│   ├── config/
│   │   ├── config.go                    # Configuration management
│   │   └── config_test.go               # Configuration tests (87.5%)
│   ├── webhook/
│   │   ├── handler.go                   # Webhook event handling
│   │   ├── verify.go                    # HMAC signature verification
│   │   ├── types.go                     # Webhook payload types
│   │   ├── handler_test.go              # Handler tests (90.6%)
│   │   └── verify_test.go               # Verification tests
│   ├── provider/
│   │   ├── provider.go                  # Provider interface definition
│   │   ├── factory.go                   # Provider factory
│   │   ├── factory_test.go              # Factory tests (100%)
│   │   ├── claude/                      # Claude Provider
│   │   │   ├── claude.go
│   │   │   └── claude_test.go           # (68.2%)
│   │   └── codex/                       # Codex Provider
│   │       ├── codex.go
│   │       └── codex_test.go            # (92.6%)
│   ├── github/
│   │   ├── auth.go                      # GitHub App auth + JWT
│   │   ├── auth_test.go                 # Auth tests
│   │   ├── gh_client.go                 # GitHub CLI abstraction
│   │   ├── gh_client_test.go            # CLI tests
│   │   ├── command_runner.go            # Safe command execution
│   │   ├── command_runner_test.go       # Command execution tests
│   │   ├── comment_state.go             # Comment state enum
│   │   ├── comment_state_test.go        # State tests
│   │   ├── comment_tracker.go           # Comment tracker
│   │   ├── comment_tracker_test.go      # Tracker tests
│   │   ├── comment_tracker_split_test.go # Split plan tests
│   │   ├── pr_splitter.go               # PR splitter (multi-PR workflow)
│   │   ├── pr_splitter_test.go          # PR splitter tests
│   │   ├── clone.go                     # gh repo clone
│   │   ├── clone_test.go                # Clone tests
│   │   ├── comment.go                   # gh issue comment
│   │   ├── label.go                     # Label operations
│   │   ├── pr.go                        # gh pr create
│   │   ├── pr_test.go                   # PR tests
│   │   └── retry.go                     # Retry logic
│   └── executor/
│       ├── task.go                      # Task executor (core workflow)
│       ├── task_test.go                 # Task tests (39.1%)
│       └── task_multipr_test.go         # Multi-PR workflow tests
├── Dockerfile                           # Docker build file
├── Makefile                             # Build automation
├── .env.example                         # Environment template
├── .gitignore                           # Git ignore file
├── go.mod                               # Go module definition
├── go.sum                               # Go dependency lock
├── CLAUDE.md                            # Claude Code dev guide
└── README.md                            # Project documentation
```

### Architecture Highlights (Linus Style)

#### 1. Filesystem Change Detection - Eliminate Assumptions

```go
// ❌ Old design: Assume Provider returns file list
if len(result.Files) == 0 {
    return // Skip PR creation
}

// ✅ New design: Detect actual filesystem state
hasChanges, _ := executor.detectGitChanges(workdir)
if hasChanges {
    commitAndPush()  // Create PR
}
```

**Good taste**: Let git tell us the truth, rather than trusting AI's output format.

#### 2. Provider Abstraction - Zero-Branch Polymorphism

```go
// Good taste design: No if provider == "claude" branches
type Provider interface {
    GenerateCode(ctx context.Context, req *CodeRequest) (*CodeResponse, error)
    Name() string
}

// Provider can choose:
// 1. Return Files list → Executor applies these files
// 2. Directly modify filesystem → Executor detects via git
// Both approaches work correctly!
```

#### 3. Clear Data Flow

```
GitHub Webhook
      ↓
  Handler (verify signature)
      ↓
  Executor (orchestrate)
      ↓
  Provider (AI generate/modify)
      ↓
  Git Status (detect changes)
      ↓
  Commit & Push
      ↓
  Comment (feedback)
```

#### 4. Safe Command Execution

```go
// CommandRunner: Prevent command injection
runner := NewSafeCommandRunner()
runner.Run("git", []string{"add", userInput})  // ✅ Safe
// Auto-validate command whitelist, argument sanitization, path validation
```

### Core Components

| Component       | Responsibility                                  | Files  | Test Coverage |
| --------------- | ----------------------------------------------- | ------ | ------------- |
| Webhook Handler | Receive, verify, parse GitHub events            | 3      | 90.6%         |
| Provider        | AI code generation abstraction layer            | 6      | 80%+          |
| Executor        | Task orchestration (Clone → Generate → Detect → Commit) | 3      | 45%+          |
| GitHub Ops      | Git operations wrapper (abstraction layer)      | 16     | 65%+          |
| PR Splitter     | Smart PR splitting and multi-workflow orchestration | 2      | 85%+          |
| Config          | Environment variable management and validation  | 2      | 87.5%         |
| Comment Tracker | Progress tracking and status updates            | 4      | -             |
| Command Runner  | Safe command execution                          | 2      | -             |
| Post-Processing | Branch link generation, PR links, empty branch cleanup | 4      | 40.5%         |

## 🧪 Testing

### Test Coverage

Overall: **84.7%** coverage across all modules

| Module            | Coverage |
|-------------------|----------|
| toolconfig        | 98.0%    |
| web               | 95.2%    |
| github/data       | **93.4%** ← **Updated with pagination tests** |
| prompt            | 92.3%    |
| dispatcher        | 91.6%    |
| webhook           | 89.6%    |
| executor          | 85.5%    |
| github            | 85.4%    |
| codex provider    | 85.3%    |
| claude provider   | 83.2%    |

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

### Docker Deployment

```bash
# Using Makefile (recommended)
make docker-build           # Build Docker image
make docker-run             # Run Docker container (requires .env file)
make docker-stop            # Stop and remove container
make docker-logs            # View container logs

# Manual Docker commands
docker build -t swe-agent .

# Run container
docker run -d \
  -p 8000:8000 \
  -e GITHUB_APP_ID=123456 \
  -e GITHUB_PRIVATE_KEY="$(cat private-key.pem)" \
  -e GITHUB_WEBHOOK_SECRET=secret \
  -e PROVIDER=codex \
  -e CODEX_MODEL=gpt-5-codex \
  --name swe-agent \
  swe-agent
```

### Docker Compose

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
      - PROVIDER=codex
      - CODEX_MODEL=gpt-5-codex
      - TRIGGER_KEYWORD=/code
    restart: unless-stopped
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

### ✅ v0.4 Implemented

- ✅ Respond to `/code` commands in `issue_comment` events
- ✅ HMAC SHA-256 webhook signature verification (anti-forgery)
- ✅ Multi-Provider support: Claude + Codex
- ✅ **Smart file change detection** (via git status)
- ✅ **Multi-PR workflow** (auto-split large changes)
- ✅ **Smart PR splitter** (group by file type and complexity)
- ✅ **Split plan display** (real-time split progress)
- ✅ **Timeout protection** (10-minute timeout)
- ✅ **Makefile build system** (unified dev commands)
- ✅ **GitHub CLI abstraction layer**
- ✅ **Safe command executor** (injection prevention)
- ✅ **Enhanced comment tracking system** (multi-PR status support)
- ✅ Auto clone, modify, commit, push to new branch
- ✅ Create PR link and reply to original comment
- ✅ Docker deployment support
- ✅ Auto-notify errors to GitHub comments
- ✅ 75%+ test coverage
- ✅ Bot comment filtering (prevent infinite loops)
- ✅ Auto label management
- ✅ **Post-processing system** (auto branch/PR links, empty branch cleanup)
- ✅ **Commit signing support** (GitHub API with automatic signing)

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

### v0.5 - Quality Assurance & Interaction (🔴 P0 Capabilities)

**Quality Assurance Layer**:
- [ ] **Automatic test execution** - Run project tests after code generation
- [ ] **Lint and format checks** - Auto-run `go vet`, `gofmt`, `golint`
- [ ] **Compilation verification** - Ensure code compiles before push
- [ ] **Security scanning** - Basic vulnerability and sensitive data detection
- [ ] **Test failure handling** - Auto-fix or rollback when tests fail

**Interaction & Collaboration Layer**:
- [ ] **Requirement clarification** - AI asks questions when unclear
- [ ] **Multi-turn collaboration** - Support conversation context and follow-ups
- [ ] **Design confirmation** - Send design draft before implementation
- [ ] **Progress reporting** - Real-time status updates during execution

### v0.6 - Context Understanding & Planning (🟠 P1 Capabilities)

**Context & Understanding Layer**:
- [ ] **Codebase architecture parsing** - Parse README, CLAUDE.md, architecture
- [ ] **Knowledge base indexing** - Vector search for relevant documentation
- [ ] **Historical analysis** - Study relevant commits and similar issues/PRs
- [ ] **Context enrichment** - Aggregate all comments, commits, file summaries

**Planning & Design Layer**:
- [ ] **Intelligent task decomposition** - Break complex tasks into sub-tasks
- [ ] **Risk assessment** - Analyze potential impacts and conflicts
- [ ] **Design proposal generation** - Create technical design documents
- [ ] **Alternative solutions** - Provide multiple implementation options

### v0.7 - Advanced Capabilities (🟡 P2 Capabilities)

**Tooling & Debugging Layer**:
- [ ] **Auto-debugging** - Analyze errors and fix issues autonomously
- [ ] **Dependency management** - Auto-add/upgrade Go modules and packages
- [ ] **Performance analysis** - Run benchmarks and identify bottlenecks
- [ ] **CI/CD integration** - Trigger builds and monitor test results

**Learning & Memory Layer**:
- [ ] **Decision recording** - Track implementation choices (ADR)
- [ ] **Error learning** - Remember and avoid past mistakes
- [ ] **Project knowledge accumulation** - Build project-specific knowledge base
- [ ] **Pattern recognition** - Identify recurring issues and solutions

### v0.8 - Quality Evolution (🟢 P3 Capabilities)

**Review & Refactoring Layer**:
- [ ] **Code review capability** - Detect code smells and security issues
- [ ] **Refactoring suggestions** - Identify improvement opportunities
- [ ] **Self-reflection** - Review own code before submission
- [ ] **Best practices validation** - Check adherence to standards

**Documentation & Knowledge Transfer Layer**:
- [ ] **Auto-documentation updates** - Update README, API docs when code changes
- [ ] **Detailed PR descriptions** - Explain rationale, impact, testing
- [ ] **Changelog management** - Auto-update CHANGELOG.md
- [ ] **Code commenting** - Add comments for complex logic

### v1.0 - Enterprise & Production Ready

**Enterprise Governance**:
- [ ] **Team permission management** - Role-based access control
- [ ] **Cost control** - API spend budgets and alerts
- [ ] **Audit log** - Record every action for compliance
- [ ] **Model policy center** - Configure models/providers per repo
- [ ] **Secure merge** - Draft PR / Fork sandbox workflows

**Production Infrastructure**:
- [ ] **Horizontal scaling** - Multi-worker node support
- [ ] **Webhook replay** - Manually retry failed tasks
- [ ] **Advanced rate limiting** - Repo/org/user granularity
- [ ] **Alerting pipelines** - Comprehensive monitoring and alerts

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