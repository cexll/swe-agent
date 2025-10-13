# 统一 Prompt 管理 PRD

作者：@cexll  日期：2025-10-13

## 1. 背景与目标

当前项目内存在多个 AI Provider（Claude、Codex），各自构造系统提示（system prompt）与用户提示（user prompt）的逻辑重复且易漂移，导致：

- 多处拷贝粘贴，维护成本高；
- Prompt 细节不一致，影响行为一致性与可测试性；
- 难以按需升级或 A/B 实验 Prompt。

目标：引入统一的 Prompt 管理组件，所有 Provider 通过该组件获取同一份 Prompt 文案与结构，做到“同一需求，同一 Prompt，跨 Provider 一致”。

## 2. 范围（In/Out of Scope）

In Scope：
- 抽象并下沉系统提示与用户提示的构建逻辑；
- 保持对现有 Provider 的调用接口不变（零侵入/低风险）；
- 不改变 Webhook、Task 构造逻辑，只替代 Provider 内部的 Prompt 拼接实现。

Out of Scope：
- 不在本期引入复杂的模板引擎或外部配置中心（后续可演进）；
- 不重写 Provider 的调用与解析器逻辑；
- 不改变输出格式协议（仍然使用 <file> 与 <summary> 块）。

## 3. 需求细化

功能性需求：
1. 提供统一 API：`BuildSystemPrompt(files, context)` 与 `BuildUserPrompt(taskPrompt)`；
2. `context` 中的 `issue_title/issue_body` 会在主 Prompt 已包含，系统提示中应避免重复；
3. 统一“改动原则/PR 大小最佳实践/输出格式”指令文案；
4. 两个 Provider 获取到的系统提示与用户提示文本一致；
5. 现有测试保持通过。

非功能性需求：
- 简单、可读、无额外依赖；
- 易于后续扩展（如换成模板文件、根据任务类型选择不同 Prompt）。

## 4. 设计方案

新增包：`internal/prompt`

- `BuildSystemPrompt(files []string, context map[string]string) string`
  - 列出仓库文件；
  - 追加 Additional Context（排除 `issue_title/issue_body`，忽略空值）；
  - 统一“改动原则/PR 最佳实践/输出格式要求”文案；
- `BuildUserPrompt(taskPrompt string) string`
  - 固定输出“任务 + 两种结果模式（代码改动/分析）+ 输出格式约束”。

Provider 改动：
- 在 `internal/provider/claude` 与 `internal/provider/codex` 中保留原 `buildSystemPrompt/buildUserPrompt` 函数签名，但内部转调 `internal/prompt`，避免破坏测试；
- 保留 Codex 的 `executionPrefix`（与执行器语义相关），其余 Prompt 文案保持一致。

## 5. 数据结构与接口

保持现状：
- `CodeRequest{ Prompt string, RepoPath string, Context map[string]string }`
- `CodeResponse{ Files []FileChange, Summary string, CostUSD float64 }`

新增：
- 包 `internal/prompt` 对上暴露纯函数，无状态。

## 6. 迁移与兼容性

- 单次提交完成，Provider 外部接口不变；
- 单元测试均应保持通过；
- 如后续需要差异化 Prompt，可在 `internal/prompt` 内增加策略分发（按任务类型、PR/Issue、风险级别等）。

## 7. 风险与缓解

- 风险：Prompt 文案统一后与现有行为不一致；
  - 缓解：沿用现有文案内容，仅消除重复、集中定义；
- 风险：后续需要动态模板；
  - 缓解：当前为纯函数实现，后续可平滑替换为文件模板或远程配置。

## 8. 验收标准

- 两个 Provider 构建出的系统提示与用户提示文本一致（不含 Provider 特有执行前缀）；
- 现有测试全部通过；
- 新增包 `internal/prompt` 可被复用，无循环依赖。

## 9. 参考

- Claude Code Action Prompt 设计与内容（供后续文案增强参考）：
  https://github.com/anthropics/claude-code-action/blob/main/src/create-prompt/index.ts#L471