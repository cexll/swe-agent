# SWE-Agent 内部PRD（基于源码逆向）

- 版本：0.1（依据代码现状）
- 更新日期：2025-10-15
- 目的：以“当前实现”为准描述目标、范围、流程、接口、部署与约束，供产品/后端/运维/集成方统一对齐。
- 依据：本仓库源码与测试，关键证据在“文件映射（代码旁证）”。

---

## 1. 产品概述

SWE-Agent 是一个 GitHub App Webhook 服务。开发者在 Issue 或 PR（含行内 Review 评论）中，@Agent 的触发词（默认 `/code`）+ 指令后，Agent 自动：

- 克隆目标仓库到临时工作区；
- 调用 AI Provider（Claude Code 或 Codex）生成/修改代码；
- 检测变更、提交到合适的分支、推送远端；
- 返回 Compare/PR 链接与变更清单；
- 全流程在“同一条跟踪评论”内实时更新状态（排队/执行/完成/失败）。

目标：降低机械性操作成本（分支/提交/PR创建），让“改代码”像留言一样简单，同时保持安全边界与可观测性。

非目标：
- 不自动合并 PR、不替代人类最终审批；
- 不执行高风险批量重构/未知脚本；
- 当前版本不将 CI 结果作为内置门禁（后续版本计划补齐）。

---

## 2. 用户与场景

用户：开源维护者、团队开发者、工具链工程师。

场景示例：
- Issue 小改动：`/code fix typo in README`；
- PR 行内微调：`/code tighten error handling here`；
- 中小规模功能/文档/测试补全；
- 大改动时自动拆分为多个逻辑 PR 并逐个创建。

---

## 3. 核心流程（E2E）

1) Webhook 接收与校验
- 接受 `issue_comment.created` 与 `pull_request_review_comment.created`；
- HMAC-SHA256 验签（常量时序比较），失败 401；
- 过滤 Bot 评论；重复评论 12 小时去重。

2) 权限判定（默认最小化）
- 默认仅 GitHub App 安装者可触发；
- 允许通过 `ALLOW_ALL_USERS=true` 或 `PERMISSION_MODE=open` 放开（仅建议内网/开发使用）。

3) 任务入队与串行化
- 有界队列+工作池；对同一 `repo#issueOrPR` 串行执行；
- 失败指数退避重试；队列满/关闭返回 503。

4) 执行器
- 以安装 token 鉴权；
- 克隆仓库（浅克隆、单分支）；
- 分支策略：
  - 打开中的 PR → 直接检出 PR 源分支并在其上提交；
  - Issue/已关闭 PR → 创建 `swe/<issue|pr>-<num>-<ts>` 新分支；
- Provider 两种路径：
  - 返回文件清单（写盘）；或
  - 直接改工作目录（用 `git status` + 手工比对兜底检测变更）。
- 提交/推送：注入 `remote.origin.pushurl`（HTTPS+token），失败重试；仍失败用 GitHub API commit 兜底；
- PR 优先 `gh pr create`，失败退回 Compare 链接（快速创建 PR）。

5) 跟踪评论与多PR
- 单条评论生命周期：Queued → Working → Completed/Failed；
- 展示分支/PR链接、修改文件、费用；
- 大改动时按 Docs/Internal/Core/Cmd 分类与阈值自动拆分子PR，显示计划与进度。

---

## 4. 功能需求（现状）

- F1 触发词解析：仅处理包含触发词（默认 `/code`）的“创建”动作评论；抽取触发词后的用户指令并构建 Prompt。
- F2 Provider 协议：`GenerateCode(ctx, CodeRequest) -> CodeResponse{Files|Summary|CostUSD}`；共享 Prompt 管理，支持 Claude/Codex 两实现。
- F3 分支策略：PR 打开→用源分支；PR 关闭/Issue→新分支。
- F4 变更检测：优先使用 Provider 返回文件清单；否则 `git status`；再以内容比对兜底（防“直接改盘但无清单”）。
- F5 推送与建PR：注入 pushurl Token；`gh pr create` 失败则提供 Compare 链接。
- F6 跟踪评论：任务阶段、分支/PR、文件清单、费用、多PR计划展示。
- F7 任务队列：有界队列、指数退避；同 PR/Issue 串行。
- F8 最简 Web UI：`/tasks` 列表与详情，展示执行日志。

---

## 5. 接口与协议

- 端口：`PORT`（默认 `8000`）
- 路由：
  - `POST /webhook`：GitHub Webhook 入口；成功排队 202；签名失败 401；不支持事件 200。
  - `GET /health`：健康检查，200 `OK`。
  - `GET /tasks`、`/tasks/{id}`：最小任务面板（内存态）。
- 事件：`issue_comment.created`、`pull_request_review_comment.created`。
- Webhook 鉴权：`X-Hub-Signature-256`（sha256=...）。

---

## 6. 配置与部署

- 核心环境变量（详见 `.env.example` 与 `internal/config/config.go`）：
  - GitHub：`GITHUB_APP_ID`、`GITHUB_PRIVATE_KEY`、`GITHUB_WEBHOOK_SECRET`
  - Provider：`PROVIDER=claude|codex`
  - Claude：`ANTHROPIC_API_KEY`、`CLAUDE_MODEL`
  - Codex：`OPENAI_API_KEY`（可选）、`OPENAI_BASE_URL`（可选）、`CODEX_MODEL`
  - 触发词/端口：`TRIGGER_KEYWORD`（默认 `/code`）、`PORT`
  - 队列：`DISPATCHER_WORKERS|QUEUE_SIZE|MAX_ATTEMPTS|RETRY_SECONDS|RETRY_MAX_SECONDS|BACKOFF_MULTIPLIER`
  - 权限（慎用）：`ALLOW_ALL_USERS`/`PERMISSION_MODE=open`
  - 禁用工具（Claude 透传）：`DISALLOWED_TOOLS`
  - 提交身份：`SWE_AGENT_GIT_NAME`、`SWE_AGENT_GIT_EMAIL`

- 运行：
  - 本地：`make run` 或 `go run cmd/main.go`
  - Docker：`make docker-build` / `make docker-run`
- 依赖：Go 1.25+、GitHub CLI(gh)、Claude Code/Codex CLI（按 Provider）。

---

## 7. 非功能需求（现状）

- 安全
  - Webhook HMAC 校验、常量时序比较；
  - Bot 评论忽略、防重放（12h TTL）；
  - App 安装者校验（可开后门用于内网/开发）。
- 稳定性
  - 有界队列+指数退避；同 PR 串行；
  - Push 重试与 API commit 兜底。
- 性能
  - 浅克隆 depth=1、单分支；Codex 执行 10 分钟超时；
- 可观测性
  - 简易任务 UI 与评论跟踪；尚无指标/Tracing。
- 合规
  - 私钥/Token 走环境变量，不持久化；`.env` 由外部管理。

---

## 8. 约束与默认策略（关键）

- 打开中的 PR：直接在源分支提交；
- Issue/已关闭 PR：新分支 + Compare 链接（可一键创建 PR）；
- Provider 既可返回文件清单，也可直接改盘，均能被检测到；
- 失败回帖包含“错误+可操作提示”（权限/网络/Token/分支常见问题）。

---

## 9. 验收标准（DoD）

- A1 事件处理
  - 未签名/签名不匹配 → 401；非支持事件 → 200 忽略；重复评论不触发；
  - 默认仅安装者可触发（开放模式允许所有人）。
- A2 执行与产物
  - Issue：创建 `swe/issue-<n>-<ts>` 分支并推送成功，生成 Compare 链接；
  - 打开 PR：在源分支新增提交，不创建新 PR；
  - 变更检测覆盖“Provider 文件清单”与“直接改盘”两路径。
- A3 评论与 UI
  - 跟踪评论显示状态、分支/PR、文件清单（若有）、费用（若有）；
  - 多 PR 显示拆分计划及每个子 PR 状态；
  - `/tasks` 可查看任务与日志。
- A4 异常兜底
  - 推送失败→API commit 尝试；
  - 错误日志含提示（权限/网络/Token/分支问题）。

---

## 10. 风险与局限

- 队列为内存实现（重启丢队列）；
- 缺少编译/格式化/测试门禁（推送未验证代码，易“破用户空间”）；
- 无指标、限流与Tracing；
- 多 PR 拆分基于文件/行数的启发式（不做语义/依赖图）。

---

## 11. 路线图建议

- v0.5 质量与交互（P0）
  - 推送前 `go fmt/go vet/go build/go test` 钩子；失败不推；
  - 基础指标（队列水位、执行时长、Provider 失败率）、限流；
  - 跟踪评论补充“失败修复建议模板”。
- v0.6 持久化与观测
  - 任务持久化（Redis/DB）、Webhook 重放、Prom 指标与报警；
- v0.7 认知与规划
  - 仓库结构解析、历史相似变更检索、任务分解/风险评估；
- v0.8 审查与文档
  - 自审与重构建议、PR 描述模板自动化、变更日志自动更新。

---

## 12. 文件映射（代码旁证）

- 服务入口与路由：`cmd/main.go:120-147`
- Webhook 入口/分流：`internal/webhook/handler.go:66-102`
- 触发词解析与任务建模：`internal/webhook/handler.go:165-179,239-248,255-269,347-360`
- 签名校验：`internal/webhook/verify.go`
- 去重与Bot过滤：`internal/webhook/handler.go:121-127,224-237`、`internal/webhook/comment_deduper.go`
- 权限校验（安装者/开放模式）：`internal/webhook/handler.go:286-318`
- 队列与串行化/退避：`internal/dispatcher/dispatcher.go`
- Provider 接口与工厂：`internal/provider/provider.go`、`internal/provider/factory.go`
- Claude/Codex 实现（CLI 调用、超时、输出解析）：`internal/provider/claude/claude.go`、`internal/provider/codex/codex.go`
- Prompt 管理（上下文/系统提示/步骤）：`internal/prompt/manager.go`
- 执行器（克隆/分支/检测/提交/推送/PR链接/错误兜底）：`internal/executor/task.go`
- 克隆优化（浅克隆/单分支）：`internal/github/clone.go`
- PR 创建与 Compare 链接：`internal/github/pr.go`、`internal/executor/task.go:1369-1386`
- API Commit 兜底：`internal/github/apicommit.go`、`internal/executor/task.go:1101-1186`
- 跟踪评论（状态/变更/多PR展示/费用）：`internal/github/comment_tracker.go`
- 任务UI：`internal/web/handler.go`、`templates/*.html`
- 配置项与校验：`internal/config/config.go`、`.env.example`

---

## 13. 术语（简）

- 触发词：用于指令起始的关键短语，默认 `/code`。
- 跟踪评论：Agent 创建并持续更新的单条 GitHub 评论，承载进度与结果。
- Compare 链接：GitHub compare URL，可快速创建 PR。

---

## 14. Taste/Risk 快评（内部）

- Taste：Provider/gh 抽象合理、职责边界清晰，最小可用（KISS）；多 PR 拆分可演进。
- Fatal：缺少“提交前质量门禁”（编译/格式/测试），推送未验证代码，存在破坏用户仓库风险。
- 建议（P0）：
  1) 强制“提交前校验”并将失败结果渗透到跟踪评论；
  2) 加队列持久化与最小限流；
  3) 默认关闭开放权限，生产禁用。

---

（完）

