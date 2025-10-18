# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.2] - 2025-10-18

### Changed
- **Version Alignment**: Unified version across README.md, README.zh-CN.md, and CLAUDE.md to match git tag v0.4.2
- **Documentation Consistency**: All documentation now shows the same version number

## [0.4.1] - 2025-10-18

### Added
- **GraphQL Pagination**: Cursor-based pagination support for large PRs with 100+ files/comments
- **Enhanced Testing**: 13 new test cases for pagination scenarios
- **Performance Optimization**: 99% of PRs now use single query, only large PRs trigger pagination

### Changed
- **Test Coverage**: `internal/github/data` coverage improved from 70.4% to 93.4%
- **GraphQL Queries**: Updated all connections to include `pageInfo` fields

## [0.4.0] - 2025-10-17

### Added
- **MCP Dynamic Configuration**: Runtime MCP server configuration for Claude and Codex providers
- **MCP Comment Server**: Custom Go-based MCP server for GitHub comment updates
- **Review Comment Triggers**: Support for `/code` in PR Review inline comments
- **Reliable Task Queue**: Dispatcher with bounded queue, worker pool, and exponential backoff
- **PR Serial Execution**: Tasks within same repo/PR queued to avoid conflicts

### Changed
- **Test Coverage**: Overall coverage improved from 70.5% to 84.7%
- **Provider Configuration**: Dynamic MCP config generation instead of static files
- **Architecture**: Enhanced provider abstraction with better MCP integration

## [0.3.0] - 2025-10-15

### Added
- **Multi-PR Workflow**: Automatically split large changes into multiple logical PRs
- **Smart PR Splitter**: Intelligent grouping by file type and dependency relationships
- **Split Plan Display**: Real-time display of split plan and progress in comments
- **Makefile Build System**: Unified build, test, and deployment commands

### Changed
- **Comment Tracking**: Enhanced support for multi-PR status display and progress updates
- **Test Coverage**: Significant improvements across all modules

## [0.2.0] - 2025-10-12

### Added
- **Filesystem Change Detection**: Auto-detect direct file modifications by AI provider
- **GitHub CLI Abstraction Layer**: `gh_client.go` unifies all gh command execution
- **Safe Command Executor**: `command_runner.go` prevents command injection attacks
- **Comment State Management**: `comment_state.go` enum states (Pending/InProgress/Completed/Failed)
- **Comment Tracker**: `comment_tracker.go` real-time GitHub comment progress updates

### Fixed
- **Codex CLI Arguments**: Fixed incorrect `--search` parameter usage
- **File Detection**: Fixed issue where provider file modifications weren't detected
- **Infinite Loops**: Fixed bot comment filtering to prevent self-triggering
- **Timeout Protection**: Added 10-minute timeout to prevent hanging tasks

## [0.1.0] - 2025-10-10

### Added
- **Initial Release**: GitHub App webhook service for AI-powered code modification
- **Multi-Provider Support**: Claude Code and Codex integration
- **Webhook Security**: HMAC SHA-256 signature verification
- **Basic Workflow**: Clone → Generate → Commit → Push → Comment
- **Docker Support**: Container deployment with environment configuration
- **Test Suite**: Comprehensive unit testing with 70%+ coverage

---

## Migration Guide

### From v0.3.x to v0.4.x

No breaking changes. Dynamic MCP configuration is backward compatible with existing setups.

### From v0.2.x to v0.3.x

No breaking changes. Multi-PR workflow is opt-in via configuration.

### From v0.1.x to v0.2.x

No breaking changes. All new features are additive and don't affect existing functionality.