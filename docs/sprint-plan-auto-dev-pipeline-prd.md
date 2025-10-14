# Sprint Plan — Auto-Dev Pipeline R1

## Sprint Goal
- 发布闭环 Agent R1 最小可用链路：Issue `/clarify` → `/prd` → `/code` → 手动创建 PR。
- 落地核心防护：权限、成本闸门、去重防抖、状态追踪。
- 为 Stage 3/4（Code Review & 修复迭代）铺垫必要的模型和状态接口。

## Cadence
- 周期：2 周（10 个工作日）。
- 每日例会：09:30，汇报阻塞与燃尽。
- 中期检查：第 6 天下午，现场演示 Stage 2 端到端链路。
- 冲刺评审与回顾：第 10 天下午。

## Scope & Stories
| 编号 | 内容 | PRD 对应章节 |
|------|------|--------------|
| S1 | `/clarify` 生成澄清问题（可选阶段） | Stage 0 |
| S2 | `/prd` 输出结构化 PRD（可选阶段） | Stage 1 |
| S3 | `/code` 执行链：克隆、分支、compare 链接、评论模版 | Stage 2 |
| S4 | WorkflowState、成本追踪、去重、防抖、CommentTracker 联动 | 设计原则 & 状态追踪 |
| S5 | 权限/成本闸门配置与拦截提示 | 配置项 |
| S6 | 指标埋点与统一回帖文案 | 监控与指标 |
| S7 | Prompt 管理、跨仓数据结构占位（技术债） | Prompt 模板管理 & Multi-Repo 规划 |

## Work Breakdown (估算)
1. 触发词解析与上下文判定（`/code` Issue/PR 分流）— 1.5 d  
2. WorkflowState 数据结构与存储接口（含 Stage 映射、FixAttempts 限制）— 1.5 d  
3. Clarify/PRD prompt 管道接入 Provider — 2 d  
4. `/code` 执行链（仓库 clone、分支命名、compare 链接、评论模版）— 2 d  
5. CommentTracker & 成本追踪、状态回帖更新 — 1.5 d  
6. 配置读取（成本、权限、测试钩子占位）— 1 d  
7. 端到端集成测试、文档同步、验收脚本 — 1.5 d  
8. 缓冲与缺陷修复 — 1 d

## Ownership
- **Alice**：Clarify/PRD prompt、WorkflowState 存储落地。
- **Bob**：`/code` 执行链实现。
- **Carol**：成本/权限闸门、CommentTracker、文档更新。
- **Scrum Master**：燃尽跟踪、阻塞清理、跨团队依赖协调。

## Milestones
- **Day 3**：Clarify/PRD 指令演示可用。
- **Day 6**：Issue → `/clarify` → `/prd` → `/code` 全链路演示。
- **Day 9**：成本/权限闸门、状态回帖、配置完成。
- **Day 10**：验收清单通过，完成评审与 Retro。

## Definition of Done
- 每个故事需满足 PRD 中对应验收复选框。
- 提供端到端手动验证证据（Issue 评论、日志、截图）。
- 成本/权限闸门需在测试环境演示并记录。

## Risks & Mitigations
- **Provider 额度不足**：预置 Mock、监控配额、提前申请扩容。
- **GitHub API 限流**：实现重试与退避，申请高配 token。
- **WorkflowState 持久化复杂度**：R1 以内存实现，接口保留以便迁移。
- **`/code` 误触发修复流程**：强化事件判定与评论格式校验。

## Dependencies
- GitHub App 权限（Installer 权限、白名单配置）。
- Provider API Key 与调用配额。
- 现有 CommentTracker、taskstore 模块稳定可用。
- 可用于端到端验证的沙箱仓库。

## Metrics & Tracking
- 每日燃尽图更新。
- 阻塞事项维护在看板 `Blocked` 列。
- 收集 Clarify→PRD 转化率、平均 FixAttempts 等原始数据，为后续 KPI 准备。

## Communication
- 产品同步：冲刺开始与中期各一次，高优需求变化及时升级。
- 运维/平台：提前三天确认 GitHub App 配置与 secrets。
- 日常渠道：Slack `#auto-dev-workflow`，每日 18:00 前更新进展。
