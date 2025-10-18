[English](README.md) | [简体中文](README.zh-CN.md)

# SWE-Agent - Software Engineering Agent

[![Go Version](https://img.shields.io/badge/Go-1.25.1-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Test Coverage](https://img.shields.io/badge/coverage-93.4%25-brightgreen)](#testing)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![GitHub](https://img.shields.io/badge/GitHub-cexll%2Fswe-agent-181717?logo=github)](https://github.com/cexll/swe-agent)

GitHub App webhook service that triggers AI to automatically complete code modification tasks via `/code` commands.

> 🎯 **Core Philosophy**: AI-first software engineering with full GitHub autonomy. Make code changes as simple as leaving comments.
>
> 🚀 **v0.4.2**: Simplified documentation structure with changelog and comprehensive docs.

## 📚 Documentation

| Document | Description |
|----------|-------------|
| **[Quick Start](docs/quick-start.md)** | Get up and running in minutes |
| **[Features](docs/features.md)** | Complete feature list and capabilities |
| **[Architecture](docs/architecture.md)** | System design and component overview |
| **[Development](docs/development.md)** | Build, test, and contribute |
| **[CHANGELOG](CHANGELOG.md)** | Version history and release notes |
| **[CLAUDE.md](CLAUDE.md)** | Development guide for Claude Code |

## 🚀 Quick Start

1. **Prerequisites**: Go 1.25.1+, Claude/Codex CLI, GitHub CLI
2. **Install**: `git clone https://github.com/cexll/swe-agent && cd swe-agent && go mod download`
3. **Configure**: Copy `.env.example` to `.env` and fill in your GitHub App and AI provider settings
4. **Run**: `source .env && go run cmd/main.go`
5. **Use**: Comment `/code fix the bug` in any Issue or PR

For detailed instructions, see the [Quick Start guide](docs/quick-start.md).

## ✨ Key Features

- 🤖 **Multi-AI Provider Support** - Claude Code and Codex
- 🔐 **Security Verification** - HMAC SHA-256 webhook verification
- ⚡ **Async Processing** - Background task execution with progress tracking
- 📦 **Smart Change Detection** - Auto-detect file system changes
- 🔀 **Multi-PR Workflow** - Split large changes into logical PRs
- 🎯 **PR Context Awareness** - Updates existing PRs intelligently
- 🛠️ **MCP Integration** - 39 GitHub MCP tools
- ✅ **High Test Coverage** - 93.4% coverage overall

[Explore all features](docs/features.md)

## 🏗️ Architecture

SWE-Agent follows Linus Torvalds' "good taste" philosophy:

- **Filesystem Change Detection**: Trust `git status` over AI output format
- **Zero-Branch Polymorphism**: Unified provider interface with no special cases
- **Safe Command Execution**: Prevent injection with validated command execution
- **Clear Data Flow**: Webhook → Handler → Executor → Provider → Git → Comment

[Learn more about architecture](docs/architecture.md)

## 🧪 Testing

```bash
# Run all tests with coverage
go test ./... -cover

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

Overall test coverage: **84.7%** across all modules.

[See development guide](docs/development.md) for detailed testing instructions.

## 📄 License

MIT License - see the [LICENSE](LICENSE) file

## 🙏 Acknowledgments

- [Codex](https://github.com/codex-rs/codex) - AI coding assistant
- [Claude Code](https://github.com/anthropics/claude-code) - AI coding assistant
- [GitHub CLI](https://cli.github.com/) - Git operations tool
- [Gorilla Mux](https://github.com/gorilla/mux) - HTTP router library
- Linus Torvalds - "Good taste" programming philosophy

---

<div align="center">

**If this project helps you, please leave a ⭐️ Star!**

Made with ❤️ by [cexll](https://github.com/cexll)

</div>