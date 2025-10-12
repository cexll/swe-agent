# Pilot SWE - Software Engineering Agent

[![Go Version](https://img.shields.io/badge/Go-1.25%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Test Coverage](https://img.shields.io/badge/coverage-70%25-brightgreen)](#-测试)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![GitHub](https://img.shields.io/badge/GitHub-cexll%2Fswe-181717?logo=github)](https://github.com/cexll/swe)

GitHub App webhook 服务，通过 `/code` 命令触发 AI 自动完成代码修改任务。

> 🎯 **核心理念**: 用 AI 赋能开发者，让代码修改变得像评论一样简单。

## 📖 目录

- [特性](#-特性)
- [快速开始](#快速开始)
- [使用方法](#使用方法)
- [架构设计](#️-架构设计)
- [最近更新](#-最近更新)
- [测试](#-测试)
- [开发](#-开发)
- [部署](#-部署)
- [路线图](#️-路线图)

## ✨ 特性

- 🤖 **多 AI Provider 支持** - Claude Code 和 Codex，易扩展
- 🔐 **安全验证** - GitHub webhook 签名验证（HMAC SHA-256）
- ⚡ **异步处理** - 立即响应 webhook，后台执行任务
- 📦 **智能变化检测** - 自动检测文件系统变化，无论 AI 如何修改文件
- 🎯 **可配置触发词** - 默认 `/code`，可自定义
- 🎨 **Clean Architecture** - Provider 接口抽象，GitHub 操作抽象
- ✅ **高测试覆盖率** - 70%+ 单元测试覆盖率
- 🛡️ **安全执行** - Command runner 防注入，沙箱执行
- 📊 **进度追踪** - Comment tracker 实时更新任务状态
- ⏱️ **超时保护** - 10 分钟超时防止任务挂起
- 🔀 **多 PR 工作流** - 自动将大型变更拆分为多个逻辑 PR
- 🧠 **智能 PR 拆分** - 按文件类型和依赖关系智能分组
- 🧵 **Review 评论触发** - 支持 Issue 评论与 PR Review 行内评论
- 🔁 **可靠任务队列** - 有界 worker 池 + 指数退避自动重试
- 🔒 **PR 串行执行** - 同一 PR 命令串行排队，避免分支/评论冲突

## 📊 项目统计

| 指标           | 数值                                         |
| -------------- | -------------------------------------------- |
| **代码量**     | 42 Go 文件，~12,500 行代码                   |
| **测试覆盖率** | 75%+ (Codex 92.6%, PR Splitter 85%+)         |
| **测试文件**   | 21 测试文件，200+ 测试函数                   |
| **编译产物**   | ~12MB 单一二进制文件                         |
| **依赖**       | Minimal - Go 1.25+, Claude CLI/Codex, gh CLI |
| **性能**       | 启动 ~100ms，内存 ~60MB                      |

## 快速开始

### 前置要求

- Go 1.25+
- [Claude Code CLI](https://github.com/anthropics/claude-code) 或 [Codex](https://github.com/codex-rs/codex)
- [GitHub CLI](https://cli.github.com/)
- API Key (Anthropic 或 OpenAI)

### 安装

```bash
# 1. 克隆项目
git clone git@github.com:cexll/swe.git
cd swe

# 2. 安装依赖
go mod download

# 3. 复制环境变量模板
cp .env.example .env

# 4. 编辑 .env 填入你的配置
# GITHUB_APP_ID=your-app-id
# GITHUB_PRIVATE_KEY="your-private-key"
# GITHUB_WEBHOOK_SECRET=your-webhook-secret
# PROVIDER=codex  # or claude
```

### 环境变量

```bash
# GitHub App 配置
GITHUB_APP_ID=123456
GITHUB_PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----\n..."
GITHUB_WEBHOOK_SECRET=your-webhook-secret

# AI Provider 配置 (二选一)
# 选项 1: Codex (推荐)
PROVIDER=codex
CODEX_MODEL=gpt-5-codex
# OPENAI_API_KEY=your-key  # 可选
# OPENAI_BASE_URL=http://...  # 可选

# 选项 2: Claude
# PROVIDER=claude
# ANTHROPIC_API_KEY=sk-ant-xxx
# CLAUDE_MODEL=claude-sonnet-4-5-20250929

# 可选配置
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
> - `DISPATCHER_WORKERS`: 并发 worker 数量（默认 4）
> - `DISPATCHER_QUEUE_SIZE`: 有界任务队列容量，超过即快速返回 503
> - `DISPATCHER_MAX_ATTEMPTS`: 单任务最大执行次数（含首轮）
> - `DISPATCHER_RETRY_SECONDS`: 首次重试延迟（秒）
> - `DISPATCHER_RETRY_MAX_SECONDS`: 指数退避的最大延迟（秒）
> - `DISPATCHER_BACKOFF_MULTIPLIER`: 每次重试的延迟倍数（默认 2）

### 本地运行

```bash
# 加载环境变量
source .env  # 或使用 export 逐个设置

# 运行服务
go run cmd/main.go
```

服务启动后，访问：

- 🏠 服务信息: http://localhost:3000/
- ❤️ 健康检查: http://localhost:3000/health
- 🔗 Webhook: http://localhost:3000/webhook

## 使用方法

### 1. 配置 GitHub App

1. **创建 GitHub App**: https://github.com/settings/apps/new
2. **权限设置**:
   - Repository permissions:
     - ✅ Contents: Read & Write
     - ✅ Issues: Read & Write
     - ✅ Pull requests: Read & Write
   - Subscribe to events:
     - ✅ Issue comments
      - ✅ Pull request review comments
3. **Webhook 设置**:
   - URL: `https://your-domain.com/webhook`
   - Secret: 生成一个随机密钥
   - Content type: `application/json`
4. **安装到仓库**

### 2. 在 Issue／PR 评论（含 Review 行内评论）中触发

在任何 Issue 或 PR 中评论：

```
/code fix the typo in README.md
```

```
/code add error handling to the main function
```

```
/code refactor the database connection code
```

在代码 Review 中也可以对具体行发表评论触发：

```
/code tighten error handling here
```

### 3. Pilot 自动执行

Pilot 会自动完成以下流程：

1. ✅ **Clone 仓库** - 下载最新代码到临时目录
2. ✅ **AI 生成** - 调用 AI provider 生成或直接修改文件
3. ✅ **检测变化** - 使用 `git status` 检测实际文件变化
4. ✅ **Commit** - 提交到新分支 `pilot/<issue-number>-<timestamp>`
5. ✅ **Push** - 推送到远程仓库
6. ✅ **回复评论** - 提供 PR 创建链接

### 4. 查看结果

Pilot 会在原评论下自动回复：

```markdown
### ✅ Task Completed Successfully

**Summary:** Fixed typo in README.md

**Modified Files:** (1)

- `README.md`

**Next Step:**
[🚀 Click here to create Pull Request](https://github.com/owner/repo/compare/main...pilot/123-1234567890?expand=1)

---

_Generated by Pilot SWE_
```

## 🔄 最近更新

### v0.4.0 - 任务队列 & Review 评论 (2025-10)

#### 🎉 新功能

- **Review 评论触发** - `/code` 现在支持 Issue 评论与 PR Review 行内评论
- **可靠任务队列** - 新增 dispatcher，支持有界队列、worker 池与指数退避重试
- **PR 串行执行** - 同一仓库同一 PR 内的任务自动排队避免冲突
- **队列状态提示** - 评论初始状态显示为 `Queued`，worker 启动后自动更新为 `Working`
- **可调度配置** - 新增 `DISPATCHER_*` 环境变量以调整并发、重试策略

### v0.3.0 - 多 PR 工作流 (2025-10)

#### 🎉 新功能

- **多 PR 工作流编排** - 自动将大型变更拆分为多个逻辑 PR
- **智能 PR 拆分器** - 按文件类型、依赖关系和复杂度智能分组
- **拆分计划显示** - 在评论中实时显示拆分计划和进度
- **Makefile 构建系统** - 统一的构建、测试、部署命令
- **增强评论追踪** - 支持多 PR 状态显示和进度更新

#### 🧠 智能拆分逻辑

- **文件分类**：docs、tests、core/internal、cmd 等智能分类
- **阈值控制**：默认单个 PR 不超过 8 个文件或 300 行代码
- **依赖排序**：按优先级排序（docs → tests → core → cmd）
- **自动命名**：根据文件类型和内容自动生成 PR 名称

#### 📊 性能提升

- 新增多 PR 工作流测试：`task_multipr_test.go`
- PR 拆分器测试覆盖率：85%+
- 评论追踪器增强测试：`comment_tracker_split_test.go`

### v0.2.0 - 重大改进 (2025-10)

#### 🎉 新功能

- **文件系统变化检测** - 自动检测 AI provider 的直接文件修改，解决 PR 创建失败问题
- **GitHub CLI 抽象层** - `gh_client.go` 统一封装所有 gh 命令执行
- **安全命令执行器** - `command_runner.go` 防止命令注入攻击
- **评论状态管理** - `comment_state.go` 枚举状态（Pending/InProgress/Completed/Failed）
- **评论追踪器** - `comment_tracker.go` 实时更新 GitHub 评论显示进度

#### 🐛 Bug 修复

- 修复 Codex CLI 参数错误（`--search` 不存在）
- 修复 AI provider 直接修改文件后不创建 PR 的问题
- 修复无限循环问题（Bot 评论触发自身）
- 添加 10 分钟超时防止 Codex 挂起

#### 🚀 性能改进

- 测试覆盖率提升：Codex 20.2% → 92.6%
- 新增 15+ 测试文件，180+ 测试用例
- 总体覆盖率提升至 70%+

#### 📚 文档更新

- 更新 CLAUDE.md 反映新架构
- 添加详细的测试说明
- 更新 API 文档

## 🏗️ 架构设计

### 目录结构

```
swe/
├── cmd/
│   └── main.go                          # HTTP 服务器入口
├── internal/
│   ├── config/
│   │   ├── config.go                    # 配置管理
│   │   └── config_test.go               # 配置测试 (87.5%)
│   ├── webhook/
│   │   ├── handler.go                   # Webhook 事件处理
│   │   ├── verify.go                    # HMAC 签名验证
│   │   ├── types.go                     # Webhook payload 类型
│   │   ├── handler_test.go              # 处理器测试 (90.6%)
│   │   └── verify_test.go               # 验证测试
│   ├── provider/
│   │   ├── provider.go                  # Provider 接口定义
│   │   ├── factory.go                   # Provider 工厂
│   │   ├── factory_test.go              # 工厂测试 (100%)
│   │   ├── claude/                      # Claude Provider
│   │   │   ├── claude.go
│   │   │   └── claude_test.go           # (68.2%)
│   │   └── codex/                       # Codex Provider
│   │       ├── codex.go
│   │       └── codex_test.go            # (92.6%)
│   ├── github/
│   │   ├── auth.go                      # GitHub App 认证 + JWT
│   │   ├── auth_test.go                 # 认证测试
│   │   ├── gh_client.go                 # GitHub CLI 抽象
│   │   ├── gh_client_test.go            # CLI 测试
│   │   ├── command_runner.go            # 安全命令执行
│   │   ├── command_runner_test.go       # 命令执行测试
│   │   ├── comment_state.go             # 评论状态枚举
│   │   ├── comment_state_test.go        # 状态测试
│   │   ├── comment_tracker.go           # 评论追踪器
│   │   ├── comment_tracker_test.go      # 追踪器测试
│   │   ├── comment_tracker_split_test.go # 拆分计划测试
│   │   ├── pr_splitter.go               # PR 拆分器 (多 PR 工作流)
│   │   ├── pr_splitter_test.go          # PR 拆分器测试
│   │   ├── clone.go                     # gh repo clone
│   │   ├── clone_test.go                # Clone 测试
│   │   ├── comment.go                   # gh issue comment
│   │   ├── label.go                     # Label 操作
│   │   ├── pr.go                        # gh pr create
│   │   ├── pr_test.go                   # PR 测试
│   │   └── retry.go                     # 重试逻辑
│   └── executor/
│       ├── task.go                      # 任务执行器（核心流程）
│       ├── task_test.go                 # 任务测试 (39.1%)
│       └── task_multipr_test.go         # 多 PR 工作流测试
├── Dockerfile                           # Docker 构建文件
├── Makefile                             # 构建自动化
├── .env.example                         # 环境变量模板
├── .gitignore                           # Git 忽略文件
├── go.mod                               # Go 模块定义
├── go.sum                               # Go 依赖锁定
├── CLAUDE.md                            # Claude Code 开发指南
└── README.md                            # 项目文档
```

### 架构亮点（Linus 风格）

#### 1. 文件系统变化检测 - 消除假设

```go
// ❌ 旧设计：假设 Provider 返回文件列表
if len(result.Files) == 0 {
    return // 跳过 PR 创建
}

// ✅ 新设计：检测文件系统真实状态
hasChanges, _ := executor.detectGitChanges(workdir)
if hasChanges {
    commitAndPush()  // 创建 PR
}
```

**好品味**：让 git 告诉我们真相，而不是信任 AI 的输出格式。

#### 2. Provider 抽象 - 零分支多态

```go
// 好品味的设计：无 if provider == "claude" 分支
type Provider interface {
    GenerateCode(ctx context.Context, req *CodeRequest) (*CodeResponse, error)
    Name() string
}

// Provider 可以选择：
// 1. 返回 Files 列表 → Executor 应用这些文件
// 2. 直接修改文件系统 → Executor 通过 git 检测
// 两种方式都能正确处理！
```

#### 3. 清晰的数据流

```
GitHub Webhook
      ↓
  Handler (验证签名)
      ↓
  Executor (编排)
      ↓
  Provider (AI 生成/修改)
      ↓
  Git Status (检测变化)
      ↓
  Commit & Push
      ↓
  Comment (反馈)
```

#### 4. 安全的命令执行

```go
// CommandRunner: 防止命令注入
runner := NewSafeCommandRunner()
runner.Run("git", []string{"add", userInput})  // ✅ 安全
// 自动验证命令白名单、参数清理、路径验证
```

### 核心组件

| 组件            | 职责                                           | 文件数 | 测试覆盖率 |
| --------------- | ---------------------------------------------- | ------ | ---------- |
| Webhook Handler | 接收、验证、解析 GitHub 事件                   | 3      | 90.6%      |
| Provider        | AI 代码生成抽象层                              | 6      | 80%+       |
| Executor        | 任务编排（Clone → Generate → Detect → Commit） | 3      | 45%+       |
| GitHub Ops      | Git 操作封装（抽象层）                         | 16     | 65%+       |
| PR Splitter     | 智能 PR 拆分和多工作流编排                      | 2      | 85%+       |
| Config          | 环境变量管理和验证                             | 2      | 87.5%      |
| Comment Tracker | 进度追踪和状态更新                             | 4      | -          |
| Command Runner  | 安全命令执行                                   | 2      | -          |

## 🧪 测试

### 运行测试

```bash
# 运行所有测试
go test ./...

# 运行测试并显示覆盖率
go test ./... -cover

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# 查看详细覆盖率
go tool cover -func=coverage.out
```

### 测试覆盖率

| 包                       | 覆盖率   | 状态        |
| ------------------------ | -------- | ----------- |
| internal/provider        | 100.0%   | ✅ 优秀     |
| internal/provider/codex  | 92.6%    | ✅ 优秀     |
| internal/webhook         | 90.6%    | ✅ 优秀     |
| internal/config          | 87.5%    | ✅ 优秀     |
| internal/provider/claude | 68.2%    | ⚠️ 良好     |
| internal/github          | 62.0%    | ⚠️ 良好     |
| internal/executor        | 39.1%    | ⚠️ 需改进   |
| **总体**                 | **70%+** | **✅ 良好** |

### 测试策略

- **单元测试**: 每个公共函数都有对应测试
- **Mock 测试**: 使用 mock provider 和 command runner
- **集成测试**: 端到端流程测试
- **边界测试**: 错误处理、超时、并发等场景

## 💻 开发

> 💡 **开发者提示**: 查看 [CLAUDE.md](./CLAUDE.md) 获取完整的开发指南，包括架构说明、测试策略和代码规范。

### 构建

```bash
# 使用 Makefile (推荐)
make build                    # 构建二进制文件
make run                      # 运行应用
make test                     # 运行所有测试
make test-coverage           # 运行测试并生成覆盖率报告
make test-coverage-html      # 生成 HTML 覆盖率报告
make fmt                     # 格式化代码
make lint                    # 代码检查
make check                   # 运行所有检查（格式化、检查、测试）
make clean                   # 清理构建文件
make all                     # 完整构建流程

# 手动构建
go build -o pilot-swe cmd/main.go

# 运行
./pilot-swe
```

### 代码格式化

```bash
# 使用 Makefile (推荐)
make fmt                      # 格式化代码
make vet                      # 代码检查
make lint                     # 完整检查（包含格式化检查）
make tidy                     # 整理依赖

# 手动操作
go fmt ./...                  # 格式化代码
go vet ./...                  # 代码检查
go mod tidy                   # 整理依赖
```

### 添加新的 AI Provider

1. 在 `internal/provider/<name>/` 创建目录
2. 实现 `Provider` 接口：
   ```go
   type Provider interface {
       GenerateCode(ctx, req) (*CodeResponse, error)
       Name() string
   }
   ```
3. Provider 可以选择：
   - 返回 `Files` 列表（Executor 会应用这些文件）
   - 直接修改 `req.RepoPath` 中的文件（Executor 会自动检测）
4. 在 `factory.go` 添加 case
5. 添加测试文件
6. 更新文档

## 🐳 部署

### Docker 部署

```bash
# 使用 Makefile (推荐)
make docker-build           # 构建 Docker 镜像
make docker-run             # 运行 Docker 容器（需要 .env 文件）
make docker-stop            # 停止并移除容器
make docker-logs            # 查看容器日志

# 手动 Docker 命令
docker build -t pilot-swe .

# 运行容器
docker run -d \
  -p 3000:3000 \
  -e GITHUB_APP_ID=123456 \
  -e GITHUB_PRIVATE_KEY="$(cat private-key.pem)" \
  -e GITHUB_WEBHOOK_SECRET=secret \
  -e PROVIDER=codex \
  -e CODEX_MODEL=gpt-5-codex \
  --name pilot-swe \
  pilot-swe
```

### Docker Compose

```yaml
version: "3.8"

services:
  pilot-swe:
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

- **Go 1.25+** - 编译运行环境
- **Codex CLI** / **Claude Code CLI** - AI 代码生成
- **GitHub CLI (`gh`)** - Git 操作
- **Gorilla Mux** - HTTP 路由

### AI Provider 支持

当前支持以下 AI provider：

- **Codex** (推荐) - 需要 Codex CLI，可选 `OPENAI_API_KEY`
- **Claude** (Anthropic) - 需要 `ANTHROPIC_API_KEY`

通过环境变量 `PROVIDER=codex` 或 `PROVIDER=claude` 切换。

## ⚡ 当前能力

### ✅ v0.3 已实现

- ✅ 响应 `issue_comment` 事件中的 `/code` 命令
- ✅ HMAC SHA-256 webhook 签名验证（防伪造）
- ✅ 多 Provider 支持：Claude + Codex
- ✅ **智能文件变化检测**（通过 git status）
- ✅ **多 PR 工作流**（自动拆分大型变更）
- ✅ **智能 PR 拆分器**（按文件类型和复杂度分组）
- ✅ **拆分计划显示**（实时显示拆分进度）
- ✅ **超时保护**（10 分钟超时）
- ✅ **Makefile 构建系统**（统一开发命令）
- ✅ **GitHub CLI 抽象层**
- ✅ **安全命令执行器**（防注入）
- ✅ **增强评论追踪系统**（支持多 PR 状态）
- ✅ 自动 clone、修改、commit、push 到新分支
- ✅ 创建 PR 链接并回复到原评论
- ✅ Docker 部署支持
- ✅ 错误自动通知到 GitHub 评论
- ✅ 75%+ 测试覆盖率
- ✅ Bot 评论过滤（防止无限循环）
- ✅ 自动 label 管理

### ⚠️ 当前限制

- ⚠️ 任务队列暂为内存实现，服务重启时排队任务会丢失
- ⚠️ 尚未实现全局速率限制 / 配额管理
- ⚠️ 缺少可视化任务面板与调度监控

### 🚀 迈向 1.0 还差什么

1. **可靠调度与可视化**：队列持久化（Redis/数据库）、任务历史、运行中断点恢复、Web 控制台、结构化日志与指标监控。
2. **上下文富集**：自动汇总所有 Issue/PR 评论、相关提交与关键文件摘要，必要时引入向量检索与“记忆”系统，减少 AI 理解偏差。
3. **质量/安全护栏**：默认执行 lint/test、安全扫描，提供敏感信息检测、速率/权限限制、成本预算与审计日志。
4. **多轮协作体验**：支持任务澄清、子任务拆分、交互式追问，以及“草稿→review→迭代”的循环操作。
5. **弹性与多实例**：调度器拆分为独立服务，支持多 worker 节点水平扩展；完善日志、指标、告警链路。
6. **企业治理**：仓库/团队白名单、角色权限模型、费用控制策略、模型/供应商策略中心化配置。
7. **触发面与集成**：扩展到定时任务、CI/CD 钩子、Repo 事件等，兼容更多工作流。
8. **安全合流**：默认走 Draft PR/Fork 流程，生成详细变更说明与测试报告，强化人工审查和合并前验证。

## 🗺️ 路线图

### v0.4 - 队列与并发（已完成）

- [x] **并发控制** - 每个 PR/Issue 同时只能一个任务
- [x] **任务队列** - 内存队列 + 指数退避重试
- [ ] **速率限制** - 防止滥用（每仓库/小时限制）
- [ ] **日志改进** - 结构化日志（JSON）+ 日志级别

### v0.5 - 功能扩展

- [x] **PR review comments 支持** - 在代码行添加评论触发
- [ ] **上下文富集** - 聚合历史评论、相关提交、文件摘要
- [ ] **多轮协作模式** - 任务澄清、草稿迭代、交互追问
- [ ] **Web UI** - 任务监控、配置管理界面
- [ ] **指标和监控** - Prometheus metrics + 告警

### v0.6 - 企业特性

- [ ] **团队权限管理** - 限制谁可以触发
- [ ] **成本控制** - API 费用预算和告警
- [ ] **审计日志** - 所有操作记录
- [ ] **Webhook 重放** - 手动重试失败的任务
- [ ] **速率限制** - 仓库 / 组织 / 用户维度
- [ ] **安全合流** - Draft PR / Fork 沙箱 + 测试报告输出
- [ ] **模型策略中心** - 不同 repo 配置模型/供应商/阈值

## 🔒 安全注意事项

| 项目             | 状态      | 说明                       |
| ---------------- | --------- | -------------------------- |
| Webhook 签名验证 | ✅ 已实现 | HMAC SHA-256               |
| 常量时间比较     | ✅ 已实现 | 防止时序攻击               |
| 命令注入防护     | ✅ 已实现 | SafeCommandRunner          |
| 超时保护         | ✅ 已实现 | 10 分钟超时                |
| Bot 评论过滤     | ✅ 已实现 | 防止无限循环               |
| API 密钥管理     | ⚠️ 建议   | 使用环境变量或密钥管理服务 |
| 队列持久化       | ⚠️ 规划中 | v0.6 任务（外部存储+重放） |
| 速率限制         | ❌ 待实现 | v0.6 计划                  |
| 并发控制         | ✅ 已实现 | 内存队列 + KeyedMutex 串行 |

## 🛠️ 故障排查

### 1. Webhook 未触发

检查：

- GitHub App 是否正确安装
- Webhook URL 是否可访问
- Webhook secret 是否匹配
- 查看 GitHub App 的 Recent Deliveries
- 如果响应码为 503，说明任务队列已满，稍后重试或调大 `DISPATCHER_QUEUE_SIZE`

### 2. Codex/Claude API 错误

检查：

- API Key 是否正确
- CLI 是否正确安装（`codex --version` 或 `claude --version`）
- API 配额是否用完
- 网络连接是否正常

### 3. Git 操作失败

检查：

- `gh` CLI 是否安装并认证（`gh auth status`）
- GitHub App 是否有 Contents 写权限
- 分支名是否冲突
- 网络连接是否稳定

### 4. PR 未创建

可能原因：

- AI 没有修改任何文件（只返回分析）
- Git 检测到无变化
- Push 失败（权限问题）

查看日志：

```
[Codex] Command completed in 2.5s
No file changes detected in working directory (analysis/answer only)
```

### 5. 任务挂起

- 检查是否触发了 10 分钟超时
- 查看日志中的 `[Codex] Executing` 和 `Command completed` 时间差
- 手动测试 codex 命令是否正常

## 🎯 设计哲学（Linus 风格）

### 1. 简单胜于复杂

- **单一职责：** 每个包只做一件事
- **清晰命名：** `provider.Provider` 而非 `AIService`
- **浅层缩进：** 函数不超过 3 层缩进

### 2. 好品味的代码

```go
// ❌ 坏品味：假设 AI 输出格式
if len(result.Files) == 0 {
    return  // 可能错过直接修改的文件
}

// ✅ 好品味：检查文件系统真实状态
hasChanges := detectGitChanges(workdir)
if hasChanges {
    commitAndPush()  // 不管 AI 怎么改，都能检测到
}
```

### 3. 消除特殊情况

```go
// ✅ 统一处理：Provider 可以选择任何方式修改文件
// 1. 返回 Files → Executor 应用
// 2. 直接修改 → Executor 通过 git 检测
// 两种方式统一用 git status 验证，零特殊分支
```

### 4. 向后兼容

- Provider 接口设计支持未来扩展
- 配置向前兼容（新增字段有默认值）
- API 不做破坏性变更

### 5. 实用主义

- 直接调用 CLI 而非重新实现（站在巨人肩上）
- 使用 `gh` CLI 而非复杂的 GitHub API 库
- 用 `git status` 检测变化而非解析 AI 输出
- 错误直接反馈到 GitHub，而非藏在日志里

## 🤝 贡献指南

欢迎提交 Issue 和 PR！

### 贡献流程

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

### 代码风格

- 使用 `go fmt` 格式化
- 遵循 Linus 的"好品味"原则
- 函数不超过 50 行
- 避免深层嵌套
- 添加单元测试（目标覆盖率 >75%）
- 提交信息使用 [Conventional Commits](https://www.conventionalcommits.org/)

## 📄 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件

## 🙏 致谢

- [Codex](https://github.com/codex-rs/codex) - AI 代码助手
- [Claude Code](https://github.com/anthropics/claude-code) - AI 代码助手
- [GitHub CLI](https://cli.github.com/) - Git 操作工具
- [Gorilla Mux](https://github.com/gorilla/mux) - HTTP 路由库
- Linus Torvalds - "Good taste" 编程哲学

## 📞 联系方式

- **Issues**: [GitHub Issues](https://github.com/cexll/swe/issues)
- **Discussions**: [GitHub Discussions](https://github.com/cexll/swe/discussions)

---

<div align="center">

**如果这个项目对你有帮助，请给个 ⭐️ Star！**

Made with ❤️ by [cexll](https://github.com/cexll)

</div>
