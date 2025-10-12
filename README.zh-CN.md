[English](README.md) | [简体中文](README.zh-CN.md)

# SWE-Agent - 软件工程智能体

[![Go Version](https://img.shields.io/badge/Go-1.25%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Test Coverage](https://img.shields.io/badge/coverage-70%25-brightgreen)](#-测试)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![GitHub](https://img.shields.io/badge/GitHub-cexll%2Fswe-181717?logo=github)](https://github.com/cexll/swe)

GitHub App webhook 服务，通过 `/code` 命令触发 AI 自动完成代码修改任务。

> 🎯 **核心理念**：让 AI 赋能开发者，使得修改代码像留言一样简单。

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

## 📊 项目数据

| 指标                | 数值                                         |
| ------------------- | -------------------------------------------- |
| **代码行数**        | 42 个 Go 文件，约 12,500 行代码             |
| **测试覆盖率**      | 75%+（Codex 92.6%，PR 拆分器 85%+）         |
| **测试文件数**      | 21 个测试文件，200+ 个测试函数              |
| **二进制大小**      | ~12MB 单一二进制文件                        |
| **依赖**            | 极少 - Go 1.25+、Claude CLI/Codex、gh CLI    |
| **性能**            | 启动 ~100ms，内存 ~60MB                      |

## 快速入门

### 前置条件

- Go 1.25+
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
PORT=3000
DISPATCHER_WORKERS=4
DISPATCHER_QUEUE_SIZE=16
DISPATCHER_MAX_ATTEMPTS=3
DISPATCHER_RETRY_SECONDS=15
DISPATCHER_RETRY_MAX_SECONDS=300
DISPATCHER_BACKOFF_MULTIPLIER=2
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

- 🏠 服务信息：http://localhost:3000/
- ❤️ 健康检查：http://localhost:3000/health
- 🔗 Webhook：http://localhost:3000/webhook

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

## 🔄 最新更新

### v0.4.0 - 任务队列与 Review 评论（2025-10）

#### 🎉 新特性

- **Review 评论触发** - `/code` 同时支持 Issue 评论与 PR Review 行内评论
- **可靠任务队列** - 新增调度器，具备有界队列、工作池与指数退避重试
- **PR 串行执行** - 同一仓库和 PR 的任务自动排队，避免冲突
- **队列状态提示** - 评论初始状态显示 `Queued`，worker 开始时自动更新为 `Working`
- **可调度配置** - 新增 `DISPATCHER_*` 环境变量，用于调整并发与重试策略

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

## 🧪 测试

### 运行测试

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

### 覆盖率

| 包                       | 覆盖率 | 状态             |
| ------------------------ | ------ | ---------------- |
| internal/provider        | 100.0% | ✅ 优秀          |
| internal/provider/codex  | 92.6%  | ✅ 优秀          |
| internal/webhook         | 90.6%  | ✅ 优秀          |
| internal/config          | 87.5%  | ✅ 优秀          |
| internal/provider/claude | 68.2%  | ⚠️ 良好          |
| internal/github          | 62.0%  | ⚠️ 良好          |
| internal/executor        | 39.1%  | ⚠️ 有待提升      |
| **整体**                 | **70%+** | **✅ 良好**     |

### 测试策略

- **单元测试**：每个公开函数都对应测试
- **Mock 测试**：使用 mock Provider 与命令执行器
- **集成测试**：端到端工作流测试
- **边界测试**：异常处理、超时、并发场景

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
  -p 3000:3000 \
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
      - "3000:3000"
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

- ⚠️ 任务队列为内存实现，服务重启会丢失排队任务
- ⚠️ 尚无全局限流/配额管理
- ⚠️ 缺少可视化任务面板与调度监控

### 🚀 距离 1.0 尚需

1. **可靠调度与可观测性**：队列持久化（Redis/数据库）、任务历史、执行断点恢复、Web 控制台、结构化日志与指标监控。
2. **上下文增强**：自动聚合全部 Issue/PR 评论、关联提交与关键文件摘要；在需要时引入向量检索与“记忆”系统，以减少 AI 误解。
3. **质量/安全护栏**：默认运行 lint/测试与安全扫描；提供敏感信息检测、限额/权限控制、成本预算与审计日志。
4. **多轮协作体验**：支持任务澄清、子任务拆解、交互式跟进以及草稿 -> 评审 -> 迭代循环。
5. **弹性与多实例**：将调度器拆分为独立服务，支持多 worker 节点横向扩展；完善日志、指标与告警管线。
6. **企业级治理**：仓库/团队白名单、角色权限模型、成本控制策略、模型/供应商策略集中化配置。
7. **触发器与集成**：扩展至定时任务、CI/CD 钩子、仓库事件与其他工作流。
8. **安全合并**：默认使用 Draft PR/Fork 工作流，输出详细变更摘要与测试报告，加强人工评审与合并前验证。

## 🗺️ 路线图

### v0.4 - 队列与并发（已完成）

- [x] **并发控制** - 同一 PR/Issue 仅允许一个任务执行
- [x] **任务队列** - 内存队列 + 指数退避重试
- [ ] **限流** - 防止滥用（按仓库/小时限额）
- [ ] **日志改进** - 结构化日志（JSON）+ 日志等级

### v0.5 - 功能扩展

- [x] **PR Review 评论支持** - 在代码行评论时触发
- [ ] **上下文增强** - 聚合历史评论、关联提交、文件摘要
- [ ] **多轮协作模式** - 任务澄清、草稿迭代、交互式跟进
- [ ] **Web UI** - 任务监控与配置管理
- [ ] **指标与监控** - Prometheus 指标 + 告警

### v0.6 - 企业特性

- [ ] **团队权限管理** - 限制可触发人员
- [ ] **成本控制** - API 开销预算与告警
- [ ] **审计日志** - 记录所有动作
- [ ] **Webhook 重放** - 手动重试失败任务
- [ ] **限流** - 仓库/组织/用户粒度
- [ ] **安全合并** - Draft PR / Fork 沙箱 + 测试报告输出
- [ ] **模型策略中心** - 按仓库配置模型/Provider/阈值

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
