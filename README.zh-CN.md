[English](README.md) | [简体中文](README.zh-CN.md)

# SWE-Agent - 软件工程智能体

[![Go Version](https://img.shields.io/badge/Go-1.25.1-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Test Coverage](https://img.shields.io/badge/coverage-93.4%25-brightgreen)](#测试)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![GitHub](https://img.shields.io/badge/GitHub-cexll%2Fswe-agent-181717?logo=github)](https://github.com/cexll/swe-agent)

GitHub App webhook 服务，通过 `/code` 命令触发 AI 自动完成代码修改任务。

> 🎯 **核心理念**：AI 优先的软件工程，完全的 GitHub 自主性。让修改代码像留言一样简单。
>
> 🚀 **v0.4.2**：简化文档结构，包含更新日志和完整文档。

## 📚 文档

| 文档 | 描述 |
|------|------|
| **[快速入门](docs/quick-start.md)** | 快速开始使用 |
| **[功能特性](docs/features.md)** | 完整功能列表和能力 |
| **[系统架构](docs/architecture.md)** | 系统设计和组件概述 |
| **[开发指南](docs/development.md)** | 构建、测试和贡献 |
| **[更新日志](CHANGELOG.md)** | 版本历史和发布说明 |
| **[CLAUDE.md](CLAUDE.md)** | Claude Code 开发指南 |

## 🚀 快速开始

1. **前置条件**：Go 1.25.1+、Claude/Codex CLI、GitHub CLI
2. **安装**：`git clone https://github.com/cexll/swe-agent && cd swe-agent && go mod download`
3. **配置**：复制 `.env.example` 为 `.env` 并填写 GitHub App 和 AI Provider 设置
4. **运行**：`source .env && go run cmd/main.go`
5. **使用**：在任意 Issue 或 PR 中评论 `/code 修复 bug`

详细说明请查看 [快速入门指南](docs/quick-start.md)。

## ✨ 核心功能

- 🤖 **多模型支持** - Claude Code 和 Codex
- 🔐 **安全校验** - HMAC SHA-256 webhook 验证
- ⚡ **异步处理** - 后台任务执行与进度跟踪
- 📦 **智能变更检测** - 自动检测文件系统变更
- 🔀 **多 PR 工作流** - 将大型改动拆分为逻辑 PR
- 🎯 **PR 上下文感知** - 智能更新现有 PR
- 🛠️ **MCP 集成** - 39 个 GitHub MCP 工具
- ✅ **高测试覆盖率** - 整体覆盖率 93.4%

[探索所有功能](docs/features.md)

## 🏗️ 架构

SWE-Agent 遵循 Linus Torvalds 的"品味"哲学：

- **文件系统变更检测**：信任 `git status` 而非 AI 输出格式
- **零分支多态**：统一的 Provider 接口，无特殊情况
- **安全命令执行**：通过验证命令执行防止注入
- **清晰数据流**：Webhook → Handler → Executor → Provider → Git → Comment

[了解架构详情](docs/architecture.md)

## 🧪 测试

```bash
# 运行所有测试并输出覆盖率
go test ./... -cover

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

整体测试覆盖率：**84.7%** 覆盖所有模块。

[查看开发指南](docs/development.md) 获取详细测试说明。

## 📄 许可证

MIT License - 详见 [LICENSE](LICENSE)

## 🙏 致谢

- [Codex](https://github.com/codex-rs/codex) - AI 编程助手
- [Claude Code](https://github.com/anthropics/claude-code) - AI 编程助手
- [GitHub CLI](https://cli.github.com/) - Git 操作工具
- [Gorilla Mux](https://github.com/gorilla/mux) - HTTP 路由库
- Linus Torvalds - "Good taste" 编程哲学

---

<div align="center">

**如果这个项目对你有帮助，请点个 ⭐️ Star！**

Made with ❤️ by [cexll](https://github.com/cexll)

</div>