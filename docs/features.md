# Features and Capabilities

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

## 📊 Project Stats

| Metric             | Value                                        |
| ------------------ | -------------------------------------------- |
| **Lines of Code**  | ~1,300 core lines (59% reduction from 3,150) |
| **Test Coverage**  | 84.7% (claude 83.2%, codex 85.3%, executor 85.5%) |
| **Test Files**     | 32 test files, 300+ test functions           |
| **Binary Size**    | ~12MB single binary                          |
| **Dependencies**   | Minimal - Go 1.25+, Codex/Claude, gh CLI     |
| **Performance**    | Startup ~100ms, Memory ~60MB                 |

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