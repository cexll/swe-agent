[English](README.md) | [简体中文](README.zh-CN.md)

# SWE-Agent - 软件工程智能体

[![Go Version](https://img.shields.io/badge/Go-1.25.1-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Test Coverage](https://img.shields.io/badge/coverage-93.4%25-brightgreen)](#-测试)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![GitHub](https://img.shields.io/badge/GitHub-cexll%2Fswe-181717?logo=github)](https://github.com/cexll/swe)

GitHub App webhook 服务，通过 `/code` 命令触发 AI 自动完成代码修改任务。

> 🎯 **核心理念**：AI 优先的软件工程，完全的 GitHub 自主性。让修改代码像留言一样简单。
>
> 🚀 **v2.1 架构革命**：GPT-5 最佳实践、MCP 集成、59% 代码精简。

## 📖 目录

- [功能](#-功能)
- [快速入门](#快速入门)
- [使用方法](#使用方法)
- [架构](#️-架构)
- [最新更新](#-最新更新)
- [测试](#-测试)
- [开发](#-开发)
- [部署](#-部署)
- [路线图](#️-路线图)

## ✨ 功能

- 🤖 **多模型支持** - 支持 Claude Code 与 Codex，易于扩展
- 🔐 **安全校验** - GitHub webhook 签名验证（HMAC SHA-256）
- ⚡ **异步处理** - Webhook 即刻响应，后台执行任务
- 📦 **智能变更检测** - 无论 AI 如何修改文件，都能自动识别文件系统变更
- 🎯 **可配置触发词** - 默认 `/code`，可按需自定义
- 🎨 **干净架构** - Provider 接口抽象、GitHub 操作抽象
- ✅ **高测试覆盖率** - 单元测试覆盖率 70%+
- 🛡️ **安全执行** - 命令执行器防注入，沙箱执行
- 📊 **进度追踪** - 评论跟踪器实时更新任务状态
- ⏱️ **超时保护** - 10 分钟超时，防止任务悬挂
- 🔀 **多 PR 工作流** - 自动将大型改动拆分成多个逻辑 PR
- 🧠 **智能 PR 拆分** - 按文件类型与依赖关系智能分组
- 🧵 **评论触发** - 支持 Issue 评论与 PR Review 行内评论
- 🔁 **可靠任务队列** - 有界工作池 + 指数退避自动重试
- 🔒 **PR 串行执行** - 同一 PR 的指令串行排队，避免分支/评论冲突
 - 🔗 **后处理** - 执行结束后自动生成分支/PR 链接
 - ✍️ **提交签名** - 可选的 GitHub API 自动签名提交
 - 🧹 **空分支清理** - 无提交分支自动删除
 - 📊 **GraphQL 分页** - 通过游标分页处理 100+ 文件/评论的大型 PR

## 🎉 最新更新

### v0.4.1 - GraphQL 分页支持（2025年10月）

#### 🎉 新功能

- ✅ **GraphQL 分页**：大型 PR 的游标分页支持
  - 通过 `fetchAllRemainingFiles` 处理 100+ 文件的 PR
  - 通过 `fetchAllRemainingComments` 支持 100+ 评论
  - Review 评论嵌套分页
  - 最大迭代安全限制（50次迭代 = 5,000条记录）
  - 性能优化：99% 的 PR 单次查询完成；仅大型 PR 触发分页

#### 🧪 测试改进

- ✅ **测试覆盖率**：`internal/github/data` 达到 **93.4%**（从 70.4% 提升）
  - 所有分页函数：100% 覆盖
  - FetchGitHubData：66.2% → 95.6%
  - FilterCommentsToTriggerTime：0% → 100%
- ✅ **13 个新测试用例**：全面的分页场景覆盖
  - 单页、多页、空结果
  - 错误处理和最大迭代限制
  - PR 和 Issue 评论分页
  - Review 和 review 评论分页
- ✅ **表驱动测试**：易于扩展新场景

#### 🔧 技术亮点

- 新类型：`PageInfo`、`FilesConnection`、`CommentsConnection`、`ReviewCommentsConnection`
- 辅助函数：`fetchAllRemainingFiles`、`fetchAllRemainingComments`、`fetchAllRemainingReviews`、`fetchAllReviewComments`
- GraphQL 查询更新：所有连接包含 `pageInfo { hasNextPage, endCursor }`
- 修复 GitHub API 限制错误："Requesting 300 records exceeds the first limit of 100"

### v0.4.0 - MCP 动态配置与增强测试（2025年10月）

#### 🎉 新功能

- ✅ **MCP 动态配置**：Claude 和 Codex Provider 运行时 MCP 服务器配置
  - Claude：通过 `--mcp-config` CLI 参数传递 JSON 配置
  - Codex：写入 TOML 配置到 `~/.codex/config.toml`
  - 自动设置 GitHub HTTP MCP、Git MCP 和 Comment Updater MCP
  - 每个 MCP 服务器独立的环境变量隔离

- ✅ **MCP Comment Server**：自定义 Go 基 MCP 服务器用于 GitHub 评论更新
  - 使用 stdio 传输与 Claude/Codex 集成
  - 工具：`mcp__comment_updater__update_claude_comment`
  - 支持 Issue 和 PR 评论

- ✅ **Review 评论触发**：`/code` 支持 Issue 评论和 PR Review 行内评论
- ✅ **可靠任务队列**：调度器带有有界队列、工作池和指数退避重试
- ✅ **PR 串行执行**：同一 repo/PR 内的任务串行排队，避免冲突

#### 🧪 测试改进

- ✅ **测试覆盖率**：达到 **84.7%** 总体覆盖率（从 70.5% 提升）
  - Claude Provider: 83.2% (buildMCPConfig: 94.4%)
  - Codex Provider: 85.3% (buildCodexMCPConfig: 95.7%)
  - Executor: 85.5% (Execute: 96.9%)
- ✅ **17 个新单元测试**：MCP 配置的全面覆盖
  - 5 个 Claude provider 测试
  - 7 个 Codex provider 测试
  - 5 个 Executor 上下文映射测试
- ✅ **测试工具**：uvx 可用性 mock、临时 HOME、JSON/TOML 验证

#### 🔧 技术亮点

- 遵循 claude-code-action 最佳实践配置 MCP
- 不与用户的 `~/.claude.json` 或 `~/.codex/config.toml` 冲突
- 通过 `DEBUG_MCP_CONFIG` 环境变量支持调试日志
- 配置生成：平均 ~18µs，~56K configs/秒
- 所有代码分支覆盖：GitHub token 存在、uvx 检测、PR/Issue 上下文

## 📊 项目数据

| 指标                | 数值                                         |
| ------------------- | -------------------------------------------- |
| **代码行数**        | ~1,300 核心代码（从 3,150 减少 59%）        |
| **测试覆盖率**      | 93.4%（github/data），总体 84.7% |
| **测试文件数**      | 32 个测试文件，300+ 个测试函数             |
| **二进制大小**      | ~12MB 单一二进制文件                        |
| **依赖**            | 极少 - Go 1.25+、Codex/Claude、gh CLI        |
| **性能**            | 启动 ~100ms，内存 ~60MB                      |

## 快速入门

### 前置条件

- Go 1.25.1+
- [Claude Code CLI](https://github.com/anthropics/claude-code) 或 [Codex](https://github.com/codex-rs/codex)
- [GitHub CLI](https://cli.github.com/)
- API Key（Anthropic 或 OpenAI）

### 安装

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

### 环境变量

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

# 提交签名（可选）
# USE_COMMIT_SIGNING=false  # 设为 true 时使用 GitHub API 提交（自动签名）

# 调试（可选）
# DEBUG_CLAUDE_PARSING=true
# DEBUG_GIT_DETECTION=true

# 权限覆盖（可选，谨慎使用）
# ALLOW_ALL_USERS=false       # 设为 true 时放开安装者校验
# PERMISSION_MODE=open        # 另一种放开方式
```

> 🧵 **队列配置说明**
> - `DISPATCHER_WORKERS`：并发 worker 数量（默认 4）
> - `DISPATCHER_QUEUE_SIZE`：有界任务队列容量，超出返回 503
> - `DISPATCHER_MAX_ATTEMPTS`：每个任务的最大执行次数（包含首次执行）
> - `DISPATCHER_RETRY_SECONDS`：首次重试延迟（秒）
> - `DISPATCHER_RETRY_MAX_SECONDS`：指数退避的最大延迟（秒）
> - `DISPATCHER_BACKOFF_MULTIPLIER`：每次重试的延迟倍数（默认 2）

### 本地开发

```bash
# Load environment variables
source .env  # or use export for each variable

# Run the service
go run cmd/main.go
```

服务启动后可访问：

- 🏠 服务信息：http://localhost:8000/
- ❤️ 健康检查：http://localhost:8000/health
- 🔗 Webhook：http://localhost:8000/webhook

## 使用方法

### 1. 配置 GitHub App

1. **创建 GitHub App**：https://github.com/settings/apps/new
2. **权限设置**：
   - 仓库权限：
     - ✅ Contents: Read & Write
     - ✅ Issues: Read & Write
     - ✅ Pull requests: Read & Write
   - 订阅事件：
     - ✅ Issue comments
      - ✅ Pull request review comments
3. **Webhook 设置**：
   - URL: `https://your-domain.com/webhook`
   - Secret: 随机生成密钥
   - Content type: `application/json`
4. **安装到目标仓库**

### 2. 在 Issue/PR 评论中触发（包含 Review 行内评论）

在任意 Issue 或 PR 中评论：

```
/code fix the typo in README.md
```

```
/code add error handling to the main function
```

```
/code refactor the database connection code
```

也可以在代码评审的具体行上触发：

```
/code tighten error handling here
```

#### 多轮（先分析 → 后实现）

可以将流程拆分为两条触发评论：

```
/code 先进行方案分析：请列出实现步骤、风险与测试建议。
```

随后执行实现：

```
/code 按方案开始实现。请以 <file path=...><content>...</content></file> 形式返回完整文件并推送。
```

仅包含触发词的最新评论被视为“唯一指令源”，其他评论只作为上下文参考。

### 3. SWE-Agent 自动执行

SWE-Agent 会自动完成如下流程：

1. ✅ **克隆仓库** - 将最新版代码下载到临时目录
2. ✅ **AI 生成/修改** - 调用 Provider 生成或直接修改文件
3. ✅ **检测变更** - 使用 `git status` 检测实际文件变更
4. ✅ **提交** - 提交到新分支 `swe-agent/<issue-number>-<timestamp>`
5. ✅ **推送** - 推送到远程仓库
6. ✅ **回复评论** - 返回 PR 创建链接

### 4. 查看结果

SWE-Agent 会在原评论下自动回复：

```markdown
### ✅ Task Completed Successfully

**Summary:** Fixed typo in README.md

**Modified Files:** (1)

- `README.md`

**Next Step:**
[🚀 Click here to create Pull Request](https://github.com/owner/repo/compare/main...swe-agent/123-1234567890?expand=1)

---

_Generated by SWE-Agent_
```

## 🔄 版本历史

### v0.3.0 - 多 PR 工作流（2025-10）

#### 🎉 新特性

- **多 PR 编排** - 自动将大型改动拆分成多个逻辑 PR
- **智能 PR 拆分器** - 按文件类型、依赖与复杂度进行智能分组
- **拆分计划展示** - 评论中实时展示拆分计划与进度
- **Makefile 构建系统** - 统一构建、测试与部署命令
- **增强评论追踪** - 支持多 PR 状态展示与进度更新

#### 🧠 智能拆分逻辑

- **文件分类**：对文档、测试、核心/内部、cmd 等文件智能分类
- **阈值控制**：默认单个 PR 不超过 8 个文件或 300 行代码
- **依赖排序**：按优先级排序（文档 → 测试 → 核心 → cmd）
- **自动命名**：根据文件类型与内容自动生成 PR 名称

#### 📊 性能提升

- 增加多 PR 工作流测试：`task_multipr_test.go`
- PR 拆分器测试覆盖率：85%+
- 增强评论追踪测试：`comment_tracker_split_test.go`

### v0.2.0 - 重大改进（2025-10）

#### 🎉 新特性

- **文件系统变更检测** - 自动识别 Provider 直接改动的文件，解决无法创建 PR 的问题
- **GitHub CLI 抽象层** - `gh_client.go` 统一所有 gh 命令执行
- **安全命令执行器** - `command_runner.go` 防止命令注入攻击
- **评论状态管理** - `comment_state.go` 枚举状态（Pending/InProgress/Completed/Failed）
- **评论追踪器** - `comment_tracker.go` 实时更新 GitHub 评论进度

#### 🐛 缺陷修复

- 修复 Codex CLI 参数错误（不存在 `--search`）
- 修复 Provider 直接改动文件却没有创建 PR 的问题
- 修复无限循环问题（Bot 评论触发自身）
- 增加 10 分钟超时，防止 Codex 卡住

#### 🚀 性能优化

- Codex 测试覆盖率从 20.2% 提升至 92.6%
- 新增 15+ 个测试文件、180+ 个测试用例
- 整体覆盖率提升至 70%+

#### 📚 文档更新

- 更新 CLAUDE.md，反映新架构
- 增补测试指南
- 更新 API 文档

## 🏗️ 架构

### 目录结构

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

### 新增组件

- `internal/github/data/` - GraphQL 数据获取与 XML 格式化
- `internal/prompt/` - 系统提示词与上下文构建

### 架构亮点（Linus 风格）

#### 1. 文件系统变更检测 - 杜绝臆测

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

**品味要点**：相信 git 的事实，而不是信任 AI 的输出格式。

#### 2. Provider 抽象 - 零分支多态

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

#### 3. 清晰数据流

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

#### 4. 安全命令执行

```go
// CommandRunner: Prevent command injection
runner := NewSafeCommandRunner()
runner.Run("git", []string{"add", userInput})  // ✅ Safe
// Auto-validate command whitelist, argument sanitization, path validation
```

### 核心组件

| 组件             | 职责                                          | 文件数 | 测试覆盖率     |
| ---------------- | --------------------------------------------- | ------ | -------------- |
| Webhook Handler  | 接收、验证、解析 GitHub 事件                  | 3      | 90.6%          |
| Provider         | AI 代码生成抽象层                             | 6      | 80%+           |
| Executor         | 任务编排（Clone → Generate → Detect → Commit）| 3      | 45%+           |
| GitHub Ops       | Git 操作封装（抽象层）                        | 16     | 65%+           |
| PR Splitter      | 智能 PR 拆分与多工作流编排                    | 2      | 85%+           |
| Config           | 环境变量管理与校验                            | 2      | 87.5%          |
| Comment Tracker  | 进度追踪与状态更新                            | 4      | -              |
| Command Runner   | 安全命令执行                                  | 2      | -              |
| Post-Processing  | 分支链接生成、PR 链接、空分支清理             | 4      | 40.5%          |

## 🧪 测试

### 测试覆盖率

整体：**84.7%** 覆盖率

| 模块            | 覆盖率 |
|-----------------|--------|
| toolconfig      | 98.0%  |
| web             | 95.2%  |
| github/data     | **93.4%** ← **新增分页测试** |
| prompt          | 92.3%  |
| dispatcher      | 91.6%  |
| webhook         | 89.6%  |
| executor        | 85.5%  |
| github          | 85.4%  |
| codex provider  | 85.3%  |
| claude provider | 83.2%  |

### 运行测试

```bash
# 运行全部测试并输出覆盖率
go test ./... -cover

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## 💻 开发

> 💡 **开发者提示**：完整开发指南（架构、测试策略、编码规范）见 [CLAUDE.md](./CLAUDE.md)。

### 构建

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

### 代码格式化

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

### 新增 AI Provider

1. 在 `internal/provider/<name>/` 创建目录
2. 实现 `Provider` 接口：
   ```go
   type Provider interface {
       GenerateCode(ctx, req) (*CodeResponse, error)
       Name() string
   }
   ```
3. Provider 可以：
   - 返回 `Files` 列表（Executor 会应用这些文件）
   - 直接修改 `req.RepoPath` 中的文件（Executor 会自动检测）
4. 在 `factory.go` 中新增 case
5. 补充测试文件
6. 更新文档

## 🐳 部署

### Docker 部署

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

## 📦 依赖

- **Go 1.25+** - 构建与运行时环境
- **Codex CLI** / **Claude Code CLI** - AI 代码生成
- **GitHub CLI (`gh`)** - Git 操作
- **Gorilla Mux** - HTTP 路由

### AI Provider 支持

当前支持的 AI Provider：

- **Codex**（推荐）- 需要 Codex CLI，可选提供 `OPENAI_API_KEY`
- **Claude**（Anthropic）- 需要 `ANTHROPIC_API_KEY`

通过环境变量 `PROVIDER=codex` 或 `PROVIDER=claude` 切换。

## ⚡ 当前能力

### ✅ v0.3 已实现

- ✅ 响应 `issue_comment` 事件中的 `/code` 指令
- ✅ HMAC SHA-256 webhook 签名校验（防伪造）
- ✅ 多 Provider 支持：Claude + Codex
- ✅ **智能文件变更检测**（依赖 git status）
- ✅ **多 PR 工作流**（自动拆分大型改动）
- ✅ **智能 PR 拆分器**（按文件类型与复杂度分组）
- ✅ **拆分计划展示**（实时展示拆分进度）
- ✅ **超时保护**（10 分钟超时）
- ✅ **Makefile 构建系统**（统一开发命令）
- ✅ **GitHub CLI 抽象层**
- ✅ **安全命令执行器**（防注入）
- ✅ **增强评论追踪系统**（支持多 PR 状态）
- ✅ 自动 clone、修改、提交、推送新分支
- ✅ 创建 PR 链接并回复原评论
- ✅ 支持 Docker 部署
- ✅ 自动将错误通知到 GitHub 评论
- ✅ 测试覆盖率 75%+
- ✅ Bot 评论过滤（防止循环）
- ✅ 自动标签管理

### ⚠️ 当前限制

**执行层限制**：
- ⚠️ 任务队列为内存实现，服务重启会丢失排队任务
- ⚠️ 尚无全局限流/配额管理
- ⚠️ 缺少可视化任务面板与调度监控

**质量保证缺口**：
- ⚠️ 代码生成后不自动执行测试
- ⚠️ 缺少 lint/format/compile 自动检查
- ⚠️ 无安全扫描和漏洞检测
- ⚠️ 未经验证的代码直接推送

**交互协作缺口**：
- ⚠️ 无需求澄清机制（AI 不会主动提问）
- ⚠️ 不支持多轮迭代（无对话上下文）
- ⚠️ 缺少实时进度汇报
- ⚠️ 单次执行，无设计确认环节

**认知理解缺口**：
- ⚠️ 不理解代码库整体架构
- ⚠️ 不分析历史 commits 和演进路径
- ⚠️ 没有项目知识库索引
- ⚠️ 无法从类似 issue/PR 中学习

**其他限制**：
- ⚠️ 缺少调试和性能分析能力
- ⚠️ 无学习和记忆机制（每次任务独立）
- ⚠️ 不进行代码审查和重构建议
- ⚠️ 文档更新不完善（缺少详细的 PR description）

## 🎯 工程能力缺口分析

### 当前状态：执行者 → 目标状态：工程师

当前 swe-agent 具备"执行层"能力，要成为真正的工程师，还需要发展以下 8 大能力层：

#### 🔴 P0 - 质量保证层
**状态**：已规划但未实现

**缺失能力**：
- **自动测试执行**：代码生成后自动运行 `go test ./...`、`npm test`
- **代码质量检查**：Lint（`go vet`、`golint`）、格式化（`gofmt`）、编译验证
- **安全扫描**：依赖漏洞检测、敏感信息泄露防范、注入风险检查
- **测试失败处理**：测试失败时自动修复或回滚

**影响**：当前推送未经验证的代码，可能破坏 CI/CD

---

#### 🔴 P0 - 交互协作层
**状态**：roadmap v0.5 规划中

**缺失能力**：
- **需求澄清**：Issue 内容不明确时主动提问
- **设计确认**：实施前发送设计草稿征求确认
- **多轮迭代**：基于上次对话继续工作
- **进度汇报**：实时更新如"数据层已完成，正在实现业务逻辑"
- **增量指令**：支持"修复错误"这类后续指令

**影响**：黑盒执行，用户无法干预或调整

---

#### 🟠 P1 - 认知理解层
**状态**：roadmap v0.5 "上下文增强"规划中

**缺失能力**：
- **代码库架构理解**：解析 README、CLAUDE.md、架构图
- **模块依赖分析**：理解项目结构和关系
- **历史演进分析**：研究相关 commits 和类似 issue/PR 解决方案
- **知识库索引**：向量搜索相关文档，构建项目特定知识图谱
- **编码约定**：学习项目特定的风格指南和最佳实践

**影响**：AI 可能不理解整体设计，做出不符合项目风格的修改

---

#### 🟠 P1 - 规划设计层
**状态**：部分实现（PR Splitter），但缺少逻辑规划

**缺失能力**：
- **智能任务分解**：将复杂需求拆解为子任务并排序依赖
- **风险评估**：分析哪些修改可能影响其他模块，识别高风险操作
- **实施设计**：生成详细的技术方案文档，提供多个备选方案
- **测试策略设计**：在实施前规划全面的测试方法
- **并行任务识别**：确定哪些任务可以并发执行

**影响**：一次性执行，缺少规划能力

---

#### 🟡 P2 - 工具使用层
**状态**：尚未规划

**缺失能力**：
- **调试能力**：分析错误日志、添加 debug 日志、追踪执行流程
- **依赖管理**：自动添加缺失的 Go modules、解决冲突、升级依赖
- **性能分析**：运行 benchmark、识别瓶颈、建议优化
- **CI/CD 集成**：自动触发构建、检查测试结果、监控部署
- **环境感知**：处理本地/staging/production 的差异

**影响**：遇到问题时无法自主调试和修复

---

#### 🟡 P2 - 学习记忆层
**状态**：roadmap 中提到"记忆系统"

**缺失能力**：
- **决策记录**：追踪为什么选择某个实现方案（ADR - 架构决策记录）
- **错误学习**：记录失败尝试和原因，避免重复错误
- **项目知识积累**：记住"这个模块很敏感"或"总是用 GORM 做数据库操作"这类模式
- **经验构建**：从项目内的成功和失败中学习
- **模式识别**：识别重复出现的问题及其解决方案

**影响**：每次任务都是独立的，无法从历史中学习

---

#### 🟢 P3 - 审查优化层
**状态**：尚未规划

**缺失能力**：
- **代码审查**：检测 code smell、性能问题、安全漏洞
- **重构建议**：识别重复代码、过度复杂的函数、更好的设计模式
- **自我反思**：提交前审查自己生成的代码
- **最佳实践验证**：检查是否遵循项目约定和行业标准
- **可维护性评估**：评估代码是否易于理解和修改

**影响**：代码质量完全依赖 prompt 质量，缺少自我优化

---

#### 🟢 P3 - 文档传承层
**状态**：部分实现（comment tracker），但不完善

**缺失能力**：
- **文档自动更新**：代码变更时更新 README、API 文档、使用指南
- **详细的 PR 描述**：解释修改原因、影响范围、测试方法
- **变更日志管理**：自动更新 CHANGELOG.md、生成 release notes
- **代码注释**：为复杂逻辑添加注释、更新过时注释、添加 TODO/FIXME 标记
- **架构决策记录**：记录重大技术决策和理由

**影响**：代码修改缺少上下文说明，后续维护困难

---

### 🎯 实施优先级（Linus 风格 - 实用主义方法）

#### Phase 1：质量保证（立即实施 - P0）
```
"Never break userspace" 的基础
- 代码生成后自动运行测试
- Lint 和格式检查
- 编译验证
- 基础安全扫描
```

#### Phase 2：交互协作（v0.5 - P0）
```
让 AI 能够"问问题"而非盲目执行
- 需求澄清机制
- 多轮迭代支持
- 实时进度反馈
- 设计确认工作流
```

#### Phase 3：认知理解（v0.6 - P1）
```
让 AI 理解项目而非只看单个文件
- 代码库架构分析
- 历史演进理解
- 知识库索引
- 类似解决方案搜索
```

#### Phase 4：其他能力（v1.0+ - P1-P3）
```
逐步完善工具使用、学习记忆、审查优化、文档传承层
- 调试和性能分析
- 学习和记忆机制
- 代码审查和重构
- 完整文档更新
```

---

### 🚀 距离 1.0 尚需

本节将高层需求映射到上述 8 大能力层：

#### 1. **质量与安全护栏**（对应：🔴 P0 质量保证层）
- 默认运行 lint/测试与安全扫描
- 提供敏感信息检测
- 限额/权限控制和成本预算
- 审计日志与合规追踪
- **为何关键**：防止破坏 CI/CD 和引入安全漏洞

#### 2. **多轮协作体验**（对应：🔴 P0 交互协作层）
- 支持任务澄清和需求消歧
- 带依赖追踪的子任务分解
- 交互式跟进和增量指令
- 草稿 → 评审 → 迭代循环
- **为何关键**：实现引导式执行而非盲目自动化

#### 3. **上下文增强**（对应：🟠 P1 认知理解层）
- 自动聚合所有 Issue/PR 评论、相关提交和关键文件摘要
- 引入向量检索和"记忆"系统以减少 AI 误解
- 解析项目文档和架构
- 从类似历史解决方案中学习
- **为何关键**：AI 需要理解项目，而非只看孤立文件

#### 4. **智能规划**（对应：🟠 P1 规划设计层）
- 将复杂任务拆解为逻辑子任务
- 评估风险和潜在副作用
- 生成技术设计方案
- 提供多个实施备选方案
- **为何关键**：真正的工程师先规划再编码

#### 5. **可靠调度与可观测性**（基础设施）
- 队列持久化（Redis/数据库）以承受重启
- 任务历史和执行断点恢复
- Web 控制台用于任务监控
- 结构化日志和指标监控
- **为何关键**：生产级可靠性

#### 6. **高级工具**（对应：🟡 P2 工具使用层）
- 测试失败时自动调试
- 依赖管理（添加/升级包）
- 性能分析和优化
- CI/CD 集成（触发构建、监控结果）
- **为何关键**：工程师使用工具，而非只写代码

#### 7. **学习与改进**（对应：🟡 P2 学习记忆层 + 🟢 P3 审查优化层）
- 记录决策和理由（ADR）
- 从失败尝试中学习
- 代码审查和重构建议
- 构建项目特定知识库
- **为何关键**：持续改进和质量演进

#### 8. **企业治理**（企业特性）
- 仓库/团队白名单
- 角色权限模型
- 成本控制策略
- 模型/供应商策略集中化配置
- 安全合并工作流（Draft PR/Fork 沙箱）
- **为何关键**：企业采用需要治理

## 🗺️ 路线图

### v0.4 - 队列与并发（已完成）

- [x] **并发控制** - 同一 PR/Issue 仅允许一个任务执行
- [x] **任务队列** - 内存队列 + 指数退避重试
- [x] **PR Review 评论支持** - 在代码行评论时触发
- [ ] **限流** - 防止滥用（按仓库/小时限额）
- [ ] **日志改进** - 结构化日志（JSON）+ 日志等级

### v0.5 - 质量保证与交互（🔴 P0 能力） - 已完成

**后处理与签名（Phase 7 & 8）**：
- [x] **后处理系统** - 执行后自动生成分支/PR 链接
- [x] **提交签名** - 基于 GitHub API 的自动签名提交
- [x] **空分支清理** - 0 提交分支自动删除
- [x] **工具配置** - 根据签名模式智能切换工具

**质量保证层**：
- [ ] **自动测试执行** - 代码生成后运行项目测试
- [ ] **Lint 和格式检查** - 自动运行 `go vet`、`gofmt`、`golint`
- [ ] **编译验证** - 推送前确保代码可编译
- [ ] **安全扫描** - 基础漏洞和敏感数据检测
- [ ] **测试失败处理** - 测试失败时自动修复或回滚

**交互协作层**：
- [ ] **需求澄清** - 不明确时 AI 主动提问
- [ ] **多轮协作** - 支持对话上下文和后续跟进
- [ ] **设计确认** - 实施前发送设计草稿
- [ ] **进度汇报** - 执行期间实时状态更新

**基础设施**：
- [x] **Web UI** - `/tasks` 任务看板与日志查看（v0.5.0 已发布）
- [ ] **指标与监控** - Prometheus 指标 + 告警

### v0.6 - 上下文理解与规划（🟠 P1 能力）

**认知理解层**：
- [ ] **代码库架构解析** - 解析 README、CLAUDE.md、架构
- [ ] **知识库索引** - 向量搜索相关文档
- [ ] **历史分析** - 研究相关 commits 和类似 issues/PRs
- [ ] **上下文增强** - 聚合所有评论、提交、文件摘要

**规划设计层**：
- [ ] **智能任务分解** - 将复杂任务拆解为子任务
- [ ] **风险评估** - 分析潜在影响和冲突
- [ ] **设计方案生成** - 创建技术设计文档
- [ ] **备选方案** - 提供多个实施选项

**基础设施**：
- [ ] **队列持久化** - Redis/数据库实现任务持久性
- [ ] **任务历史** - 追踪执行历史并从断点恢复

### v0.7 - 高级能力（🟡 P2 能力）

**工具使用层**：
- [ ] **自动调试** - 自主分析错误并修复问题
- [ ] **依赖管理** - 自动添加/升级 Go modules 和包
- [ ] **性能分析** - 运行 benchmark 并识别瓶颈
- [ ] **CI/CD 集成** - 触发构建并监控测试结果

**学习记忆层**：
- [ ] **决策记录** - 追踪实施选择（ADR）
- [ ] **错误学习** - 记住并避免过去的错误
- [ ] **项目知识积累** - 构建项目特定知识库
- [ ] **模式识别** - 识别重复问题和解决方案

### v0.8 - 质量演进（🟢 P3 能力）

**审查优化层**：
- [ ] **代码审查能力** - 检测 code smell 和安全问题
- [ ] **重构建议** - 识别改进机会
- [ ] **自我反思** - 提交前审查自己的代码
- [ ] **最佳实践验证** - 检查是否遵循标准

**文档传承层**：
- [ ] **文档自动更新** - 代码变更时更新 README、API 文档
- [ ] **详细的 PR 描述** - 解释理由、影响、测试方法
- [ ] **变更日志管理** - 自动更新 CHANGELOG.md
- [ ] **代码注释** - 为复杂逻辑添加注释

### v1.0 - 企业与生产就绪

**企业治理**：
- [ ] **团队权限管理** - 基于角色的访问控制
- [ ] **成本控制** - API 开销预算与告警
- [ ] **审计日志** - 记录所有操作以满足合规
- [ ] **模型策略中心** - 按仓库配置模型/Provider
- [ ] **安全合并** - Draft PR / Fork 沙箱工作流

**生产基础设施**：
- [ ] **横向扩展** - 多 worker 节点支持
- [ ] **Webhook 重放** - 手动重试失败任务
- [ ] **高级限流** - 仓库/组织/用户粒度
- [ ] **告警管线** - 全面监控和告警

## 🔒 安全考量

| 项目                         | 状态        | 说明                                      |
| ---------------------------- | ----------- | ----------------------------------------- |
| Webhook 签名校验             | ✅ 已实现   | HMAC SHA-256                              |
| 恒定时间比较                 | ✅ 已实现   | 防止计时攻击                               |
| 命令注入防护                 | ✅ 已实现   | SafeCommandRunner                         |
| 超时保护                     | ✅ 已实现   | 10 分钟超时                               |
| Bot 评论过滤                 | ✅ 已实现   | 防止无限循环                               |
| API Key 管理                 | ⚠️ 建议     | 使用环境变量或秘密管理服务                |
| 队列持久化                   | ⚠️ 规划中   | v0.6 目标（外部存储 + 重放）              |
| 限流                         | ❌ 未完成   | v0.6 路线图                               |
| 并发控制                     | ✅ 已实现   | 内存队列 + KeyedMutex 串行化              |

## 🛠️ 故障排查

### 1. Webhook 未触发

排查：

- GitHub App 是否正确安装
- Webhook URL 是否可达
- Webhook Secret 是否匹配
- 查看 GitHub App 的 Recent Deliveries
- 如果响应码为 503，表示队列已满；稍后重试或增大 `DISPATCHER_QUEUE_SIZE`

### 2. Codex/Claude API 报错

排查：

- API Key 是否正确
- CLI 是否正确安装（`codex --version` 或 `claude --version`）
- API 配额是否耗尽
- 网络连接是否稳定

### 3. Git 操作失败

排查：

- `gh` CLI 是否已安装并认证（`gh auth status`）
- GitHub App 是否拥有 Contents 写权限
- 是否存在分支名冲突
- 网络连接是否稳定

### 4. 未创建 PR

可能原因：

- AI 未修改任何文件（仅分析结果）
- Git 未检测到改动
- 推送失败（权限问题）

检查日志：

```
[Codex] Command completed in 2.5s
No file changes detected in working directory (analysis/answer only)
```

### 5. 任务卡住

- 查看是否触发 10 分钟超时
- 对比日志中 `[Codex] Executing` 与 `Command completed` 的时间戳
- 手动测试 codex 指令是否可用

## 🎯 设计哲学 - Linus 风格

### 1. 简单胜于复杂

- **单一职责：** 每个包只做一件事
- **清晰命名：** 使用 `provider.Provider` 而非 `AIService`
- **浅层缩进：** 函数保持在三级缩进以内

### 2. 写出有品味的代码

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

### 3. 消灭特殊分支

```go
// ✅ Unified handling: Providers can modify files any way they want
// 1. Return Files -> Executor applies them
// 2. Modify directly -> Executor detects via git
// Both paths validated with git status, zero special branches
```

### 4. 保持向后兼容

- Provider 接口设计保留扩展空间
- 配置保持前向兼容（新字段有默认值）
- API 避免破坏性改动

### 5. 务实主义

- 直接调用 CLI，而不是重写其功能（站在巨人肩膀上）
- 使用 `gh` CLI，而不是复杂的 GitHub API 库
- 依赖 `git status` 检测变更，而不是解析 AI 输出
- 直接把错误反馈到 GitHub，而不是藏在日志里

## 🤝 贡献指南

欢迎提交 Issue 与 PR！

### 提交流程

1. Fork 本仓库
2. 创建功能分支（`git checkout -b feature/AmazingFeature`）
3. 提交改动（`git commit -m 'Add some AmazingFeature'`）
4. 推送分支（`git push origin feature/AmazingFeature`）
5. 发起 Pull Request

### 代码规范

- 运行 `go fmt`
- 遵循 Linus 的“品味”原则
- 函数保持在 50 行以内
- 避免深层嵌套
- 添加单元测试（目标覆盖率 >75%）
- 提交信息遵循 [Conventional Commits](https://www.conventionalcommits.org/)

## 📄 许可证

MIT License - 详见 [LICENSE](LICENSE)

## 🙏 致谢

- [Codex](https://github.com/codex-rs/codex) - AI 编程助手
- [Claude Code](https://github.com/anthropics/claude-code) - AI 编程助手
- [GitHub CLI](https://cli.github.com/) - Git 操作工具
- [Gorilla Mux](https://github.com/gorilla/mux) - HTTP 路由库
- Linus Torvalds - “Good taste” 编程哲学

## 📞 联系

- **Issues**：[GitHub Issues](https://github.com/cexll/swe/issues)
- **Discussions**：[GitHub Discussions](https://github.com/cexll/swe/discussions)

---

<div align="center">

**如果这个项目对你有帮助，请点个 ⭐️ Star！**

Made with ❤️ by [cexll](https://github.com/cexll)

</div>
