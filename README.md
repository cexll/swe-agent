[English](README.md) | [简体中文](README.zh-CN.md)

# SWE-Agent - Software Engineering Agent

[![Go Version](https://img.shields.io/badge/Go-1.25%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Test Coverage](https://img.shields.io/badge/coverage-70%25-brightgreen)](#-testing)
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

- 🤖 **Multi-AI Provider Support** - Claude Code and Codex, easily extensible
- 🔐 **Security Verification** - GitHub webhook signature verification (HMAC SHA-256)
- ⚡ **Async Processing** - Immediate webhook response, background task execution
- 📦 **Smart Change Detection** - Auto-detect filesystem changes regardless of how AI modifies files
- 🎯 **Configurable Trigger Words** - Default `/code`, customizable
- 🎨 **Clean Architecture** - Provider interface abstraction, GitHub operations abstraction
- ✅ **High Test Coverage** - 70%+ unit test coverage
- 🛡️ **Safe Execution** - Command runner with injection prevention, sandboxed execution
- 📊 **Progress Tracking** - Comment tracker with real-time task status updates
- 🖥️ **Task Dashboard UI** - Built-in `/tasks` web view for queue status, assignees, and logs
- ⏱️ **Timeout Protection** - 10-minute timeout prevents task hang-ups
- 🔀 **Multi-PR Workflow** - Automatically split large changes into multiple logical PRs
- 🧠 **Smart PR Splitting** - Intelligent grouping by file type and dependency relationships
- 🧵 **Review Comment Triggers** - Support for both Issue comments and PR Review inline comments
- 🔁 **Reliable Task Queue** - Bounded worker pool + exponential backoff auto-retry
- 🔒 **PR Serial Execution** - Commands for the same PR queued serially to avoid branch/comment conflicts

## 📊 Project Stats

| Metric             | Value                                        |
| ------------------ | -------------------------------------------- |
| **Lines of Code**  | 42 Go files, ~12,500 lines of code           |
| **Test Coverage**  | 75%+ (Codex 92.6%, PR Splitter 85%+)         |
| **Test Files**     | 21 test files, 200+ test functions           |
| **Binary Size**    | ~12MB single binary                          |
| **Dependencies**   | Minimal - Go 1.25+, Claude CLI/Codex, gh CLI |
| **Performance**    | Startup ~100ms, Memory ~60MB                 |

## Quick Start

### Prerequisites

- Go 1.25+
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

## 🔄 Recent Updates

### v0.5.0 - Task Dashboard & Web UI (2025-10)

#### 🎉 New Features

- **Task Dashboard UI** - New `/tasks` page lists pending, running, completed, and failed jobs with repo metadata and actors.
- **Task Detail Logs** - `/tasks/{id}` exposes structured execution logs so you can inspect command output without reading server logs.
- **Template-driven Web Layer** - HTML templates under `templates/` keep the UI fast to iterate while keeping Go handlers slim.

#### 🔧 Improvements

- **In-memory Task Store** - Shared in-memory state backs the executor, webhook handler, and web layer so the dashboard reflects real-time progress.
- **Documented Entry Points** - Local URLs for the dashboard ship alongside health and webhook endpoints for faster onboarding.

### v0.4.0 - Task Queue & Review Comments (2025-10)

#### 🎉 New Features

- **Review Comment Triggers** - `/code` now supports both Issue comments and PR Review inline comments
- **Reliable Task Queue** - Added dispatcher with bounded queue, worker pool, and exponential backoff retry
- **PR Serial Execution** - Tasks within the same repo and PR automatically queued to avoid conflicts
- **Queue Status Hints** - Comment initial state shows `Queued`, auto-updates to `Working` when worker starts
- **Schedulable Configuration** - Added `DISPATCHER_*` environment variables to adjust concurrency and retry strategies

### v0.3.0 - Multi-PR Workflow (2025-10)

#### 🎉 New Features

- **Multi-PR Workflow Orchestration** - Automatically split large changes into multiple logical PRs
- **Smart PR Splitter** - Intelligent grouping by file type, dependencies, and complexity
- **Split Plan Display** - Real-time display of split plan and progress in comments
- **Makefile Build System** - Unified build, test, and deployment commands
- **Enhanced Comment Tracking** - Support for multi-PR status display and progress updates

#### 🧠 Smart Splitting Logic

- **File Classification**: Intelligent classification of docs, tests, core/internal, cmd, etc.
- **Threshold Control**: Default single PR no more than 8 files or 300 lines of code
- **Dependency Ordering**: Sorted by priority (docs → tests → core → cmd)
- **Auto Naming**: Automatically generate PR names based on file type and content

#### 📊 Performance Improvements

- Added multi-PR workflow tests: `task_multipr_test.go`
- PR splitter test coverage: 85%+
- Enhanced comment tracker tests: `comment_tracker_split_test.go`

### v0.2.0 - Major Improvements (2025-10)

#### 🎉 New Features

- **Filesystem Change Detection** - Auto-detect direct file modifications by AI provider, solving PR creation failures
- **GitHub CLI Abstraction Layer** - `gh_client.go` unifies all gh command execution
- **Safe Command Executor** - `command_runner.go` prevents command injection attacks
- **Comment State Management** - `comment_state.go` enum states (Pending/InProgress/Completed/Failed)
- **Comment Tracker** - `comment_tracker.go` real-time GitHub comment progress updates

#### 🐛 Bug Fixes

- Fixed Codex CLI parameter error (`--search` does not exist)
- Fixed issue where AI provider directly modified files without creating PR
- Fixed infinite loop issue (Bot comments triggering itself)
- Added 10-minute timeout to prevent Codex hang-ups

#### 🚀 Performance Improvements

- Test coverage improved: Codex 20.2% → 92.6%
- Added 15+ test files, 180+ test cases
- Overall coverage improved to 70%+

#### 📚 Documentation Updates

- Updated CLAUDE.md to reflect new architecture
- Added detailed testing instructions
- Updated API documentation

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

## 🧪 Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test ./... -cover

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# View detailed coverage
go tool cover -func=coverage.out
```

### Test Coverage

| Package                  | Coverage | Status           |
| ------------------------ | -------- | ---------------- |
| internal/provider        | 100.0%   | ✅ Excellent     |
| internal/provider/codex  | 92.6%    | ✅ Excellent     |
| internal/webhook         | 90.6%    | ✅ Excellent     |
| internal/config          | 87.5%    | ✅ Excellent     |
| internal/provider/claude | 68.2%    | ⚠️ Good          |
| internal/github          | 62.0%    | ⚠️ Good          |
| internal/executor        | 39.1%    | ⚠️ Needs Improvement |
| **Overall**              | **70%+** | **✅ Good**      |

### Test Strategy

- **Unit Tests**: Each public function has corresponding tests
- **Mock Testing**: Using mock provider and command runner
- **Integration Tests**: End-to-end workflow testing
- **Boundary Tests**: Error handling, timeout, concurrency scenarios

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

### ✅ v0.3 Implemented

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

**Other Limitations**:
- ⚠️ No debugging or performance analysis capabilities
- ⚠️ No learning/memory mechanism (each task is isolated)
- ⚠️ No code review or refactoring suggestions
- ⚠️ Incomplete documentation updates (missing detailed PR descriptions)

## 🎯 Engineering Capabilities Gap Analysis

### Current State: Executor → Target State: Engineer

The current swe-agent has "Execution Layer" capabilities. To become a true engineer, it needs to develop these 8 capability layers:

#### 🔴 P0 - Quality Assurance Layer
**Status**: Planned in roadmap but not yet implemented

**Missing Capabilities**:
- **Automatic test execution**: Run `go test ./...`, `npm test` after code generation
- **Code quality checks**: Lint (`go vet`, `golint`), format (`gofmt`), compilation verification
- **Security scanning**: Dependency vulnerability detection, sensitive data leak prevention, injection risk checks
- **Test failure handling**: Auto-fix or rollback when tests fail

**Impact**: Currently pushes unvalidated code that may break CI/CD

---

#### 🔴 P0 - Interaction & Collaboration Layer
**Status**: Planned in roadmap v0.5

**Missing Capabilities**:
- **Requirement clarification**: Ask questions when issue content is ambiguous
- **Design confirmation**: Send design draft before implementation for approval
- **Multi-turn iteration**: Continue work based on previous conversation context
- **Progress reporting**: Real-time updates like "Data layer done, implementing business logic"
- **Incremental commands**: Support follow-up commands like "fix the error"

**Impact**: Black-box execution without user intervention or adjustment

---

#### 🟠 P1 - Context & Understanding Layer
**Status**: Planned in roadmap v0.5 "Context enrichment"

**Missing Capabilities**:
- **Codebase architecture understanding**: Parse README, CLAUDE.md, architecture diagrams
- **Module dependency analysis**: Understand project structure and relationships
- **Historical evolution analysis**: Study relevant commits and similar issue/PR solutions
- **Knowledge base indexing**: Vector search for relevant docs, build project-specific knowledge graph
- **Coding conventions**: Learn project-specific style guides and best practices

**Impact**: AI may not understand overall design, making changes that don't fit project style

---

#### 🟠 P1 - Planning & Design Layer
**Status**: Partially implemented (PR Splitter), but lacks logical planning

**Missing Capabilities**:
- **Intelligent task decomposition**: Break complex requirements into sub-tasks with dependency ordering
- **Risk assessment**: Analyze which changes may affect other modules, identify high-risk operations
- **Implementation design**: Generate detailed technical proposals with multiple alternatives
- **Test strategy design**: Plan comprehensive testing approach before implementation
- **Parallel task identification**: Determine which tasks can be executed concurrently

**Impact**: One-shot execution without planning capability

---

#### 🟡 P2 - Tooling & Debugging Layer
**Status**: Not planned yet

**Missing Capabilities**:
- **Debugging ability**: Analyze error logs, add debug logging, trace execution flow
- **Dependency management**: Auto-add missing Go modules, resolve conflicts, upgrade dependencies
- **Performance analysis**: Run benchmarks, identify bottlenecks, suggest optimizations
- **CI/CD integration**: Trigger builds automatically, check test results, monitor deployments
- **Environment awareness**: Handle differences between local/staging/production

**Impact**: Cannot self-debug or fix issues autonomously

---

#### 🟡 P2 - Learning & Memory Layer
**Status**: "Memory system" mentioned in roadmap

**Missing Capabilities**:
- **Decision recording**: Track why certain implementation choices were made (ADR - Architecture Decision Records)
- **Error learning**: Record failed attempts and reasons to avoid repeating mistakes
- **Project knowledge accumulation**: Remember patterns like "this module is sensitive" or "always use GORM for DB ops"
- **Experience building**: Learn from past successes and failures within the project
- **Pattern recognition**: Identify recurring issues and their solutions

**Impact**: Each task is isolated, cannot learn from history

---

#### 🟢 P3 - Review & Refactoring Layer
**Status**: Not planned yet

**Missing Capabilities**:
- **Code review**: Detect code smells, performance issues, security vulnerabilities
- **Refactoring suggestions**: Identify duplicated code, overly complex functions, better design patterns
- **Self-reflection**: Review own generated code before submission
- **Best practices validation**: Check adherence to project conventions and industry standards
- **Maintainability assessment**: Evaluate if code is easy to understand and modify

**Impact**: Code quality entirely depends on prompt quality, lacks self-optimization

---

#### 🟢 P3 - Documentation & Knowledge Transfer Layer
**Status**: Partially implemented (comment tracker), but incomplete

**Missing Capabilities**:
- **Auto-documentation updates**: Update README, API docs, usage guides when code changes
- **Detailed PR descriptions**: Explain why changes were made, scope of impact, testing methods
- **Changelog management**: Auto-update CHANGELOG.md, generate release notes
- **Code commenting**: Add comments for complex logic, update outdated comments, add TODO/FIXME markers
- **Architecture decision records**: Document significant technical decisions and rationale

**Impact**: Code changes lack context, making future maintenance difficult

---

### 🎯 Implementation Priority (Linus Style - Pragmatic Approach)

#### Phase 1: Quality Assurance (Immediate - P0)
```
Foundation for "Never break userspace"
- Auto-run tests after code generation
- Lint and format checks
- Compilation verification
- Basic security scanning
```

#### Phase 2: Interaction & Collaboration (v0.5 - P0)
```
Enable AI to "ask questions" instead of blind execution
- Requirement clarification mechanism
- Multi-turn iteration support
- Real-time progress feedback
- Design confirmation workflow
```

#### Phase 3: Context & Understanding (v0.6 - P1)
```
Make AI understand the project, not just individual files
- Codebase architecture analysis
- Historical evolution understanding
- Knowledge base indexing
- Similar solution search
```

#### Phase 4: Other Capabilities (v1.0+ - P1-P3)
```
Gradually improve tooling, learning, review, and documentation layers
- Debugging and performance analysis
- Learning and memory mechanisms
- Code review and refactoring
- Complete documentation updates
```

---

### 🚀 What's Missing for 1.0

This section maps the high-level requirements to the 8 capability layers described above:

#### 1. **Quality & Security Guardrails** (Maps to: 🔴 P0 Quality Assurance Layer)
- Run lint/tests and security scans by default
- Provide sensitive information detection
- Rate/permission limits and cost budgeting
- Audit logs and compliance tracking
- **Why critical**: Prevents breaking CI/CD and introducing security vulnerabilities

#### 2. **Multi-turn Collaboration Experience** (Maps to: 🔴 P0 Interaction & Collaboration Layer)
- Support task clarification and requirement disambiguation
- Subtask decomposition with dependency tracking
- Interactive follow-ups and incremental commands
- Draft → review → iterate loop
- **Why critical**: Enables guided execution instead of blind automation

#### 3. **Context Enrichment** (Maps to: 🟠 P1 Context & Understanding Layer)
- Automatically aggregate all issue/PR comments, related commits, and key file summaries
- Introduce vector search and a "memory" system to reduce AI misunderstanding
- Parse project documentation and architecture
- Learn from similar historical solutions
- **Why critical**: AI needs to understand the project, not just isolated files

#### 4. **Intelligent Planning** (Maps to: 🟠 P1 Planning & Design Layer)
- Break complex tasks into logical sub-tasks
- Assess risks and potential side effects
- Generate technical design proposals
- Provide multiple implementation alternatives
- **Why critical**: Real engineers plan before coding

#### 5. **Reliable Scheduling and Observability** (Infrastructure)
- Queue persistence (Redis/database) to survive restarts
- Job history and execution checkpoint resume
- Web console for task monitoring
- Structured logging and metrics monitoring
- **Why critical**: Production-grade reliability

#### 6. **Advanced Tooling** (Maps to: 🟡 P2 Tooling & Debugging Layer)
- Automatic debugging when tests fail
- Dependency management (add/upgrade packages)
- Performance analysis and optimization
- CI/CD integration (trigger builds, monitor results)
- **Why critical**: Engineers use tools, not just write code

#### 7. **Learning & Improvement** (Maps to: 🟡 P2 Learning & Memory + 🟢 P3 Review & Refactoring)
- Record decisions and rationale (ADR)
- Learn from failed attempts
- Code review and refactoring suggestions
- Build project-specific knowledge base
- **Why critical**: Continuous improvement and quality evolution

#### 8. **Enterprise Governance** (Enterprise Features)
- Repository/team whitelists
- Role permission models
- Cost control policies
- Centralized configuration for model/vendor policies
- Secure merge workflows (Draft PR/Fork sandbox)
- **Why critical**: Enterprise adoption requires governance

## 🗺️ Roadmap

### v0.4 - Queueing and concurrency (completed)

- [x] **Concurrency control** - Only one task per PR/Issue at a time
- [x] **Task queue** - In-memory queue with exponential backoff retries
- [x] **PR review comments support** - Trigger when commenting on code lines
- [ ] **Rate limiting** - Prevent abuse (per-repo/hour limits)
- [ ] **Logging improvements** - Structured logs (JSON) + log levels

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

**Infrastructure**:
- [x] **Web UI** - Task dashboard and log viewer at `/tasks` (shipped in v0.5.0)
- [ ] **Metrics and monitoring** - Prometheus metrics + alerts

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

**Infrastructure**:
- [ ] **Queue persistence** - Redis/database for task durability
- [ ] **Job history** - Track execution history and resume from checkpoints

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
