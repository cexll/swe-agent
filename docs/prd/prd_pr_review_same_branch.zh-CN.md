# PRD：PR 代码评审与同分支自动修复

## 1. 背景与目标

- 背景：当前系统已支持在 Issue/PR 会话中通过 `/code` 触发自动生成改动并创建新分支 + PR。但对「PR 代码评审(review comment) 场景」不够友好：
  - 无法基于评审评论定位具体问题代码上下文；
  - 不能直接在 PR 的同一分支上修复，导致评审-修复流程割裂。

- 目标：
  1) 在 PR 的「行内评审评论」中使用 `/code <指令>` 可直接触发 SWE 在「该 PR 的 head 分支」上进行修复与提交；
  2) 自动把评审评论中的文件路径、diff hunk、行位置信息注入到模型上下文，定位并修复被标记的问题代码；
  3) 执行完成后在跟踪评论中给出结果与变更摘要；必要时补充建议块或失败原因。

## 2. 范围与非目标

- 范围：
  - 触发源：`pull_request_review_comment` 事件中包含 `/code` 的新创建评论；
  - 修复模式：仅“同分支修复”，即直接向 PR 的 head 分支提交；
  - 上下文：自动收集该条评审评论的 `path`、`diff_hunk`、(可选)行号/position/commit id，并注入到 Prompt。

- 非目标：
  - 不修改现有 issue_comment 的分支新建 + 新 PR 的工作流；
  - 不在本阶段实现复杂冲突解决或自动 rebase；
  - 不实现跨仓库/跨 fork 的写入（权限外的情况只回退为“分析答复”）。

## 3. 用户故事

- 作为评审者，我在 PR 的某行代码上发表评论：`/code 请将这里的错误处理改为返回包装后的错误，并补充单测`。系统应：
  - 识别该评论为触发；
  - 把这条评论关联的文件 `path` 与 `diff_hunk` 带入上下文；
  - 克隆该仓库 PR 的 head 分支；
  - 在本地修改相应文件与测试；
  - 直接向 head 分支提交并推送；
  - 在跟踪评论里回报所做变更和费用（若有）；
  - 若无法安全修改，仅产出“分析答复”，不提交代码。

## 4. 触发与判定规则

- 事件：`pull_request_review_comment` 且 `action == created`；
- 过滤：忽略 Bot；
- 命中：`comment.body` 包含配置的触发词（默认 `/code`）；
- 防抖：沿用 comment ID 去重逻辑；
- 指令：触发词后的剩余文本为“用户指令”，允许为空（仅以上下文修复）。

## 5. 关键信息与数据结构

- 需要从 Webhook 载荷中获取：
  - PR 编号：`pull_request.number`
  - PR 基准分支(base)：`pull_request.base.ref`（用于显示，不用于提交）
  - PR 头部分支(head)：`pull_request.head.ref`（用于克隆/提交目标）
  - 评审评论上下文：`comment.path`、`comment.diff_hunk`、(尽量) `line/start_line/commit_id`/`original_*`

- Task 增强（新增字段，向下兼容）：
  - `Mode`：`"review_in_branch" | "standard"`（默认 `standard`）
  - `ReviewPath`：string（可空）
  - `ReviewDiffHunk`：string（可空）
  - `ReviewCommitID`：string（可空）
  - `ReviewLine` / `ReviewStartLine`：int（可空）

## 6. 行为与流程（PR Review 同分支模式）

1) Webhook Handler：
   - 在 `handleReviewComment` 中：
     - 校验触发；
     - 读取并保存 head 分支：`pull_request.head.ref`；
     - 将 `comment.path`、`comment.diff_hunk` 等注入 Task 字段；
     - `Task.Mode = "review_in_branch"`；
     - `Task.Branch = head.ref`（替代当前使用的 base.ref）。

2) Executor：
   - 克隆仓库到工作目录：`gh repo clone <repo> -b <Task.Branch>`；
   - 构建 Prompt：在现有 Issue/PR 内容基础上，插入一个“Discussion/Context”区块，包含评审评论元信息：
     - File: `<path>`；
     - Diff: 以 ```diff 包裹的 `diff_hunk`；
     - 用户指令（触发词后的文本）。
   - 调用 Provider 生成改动；
   - 检测有无改动：
     - 若无改动：更新跟踪评论（响应-only）；
     - 若有改动：
       - 在同一分支直接 `git add/commit/push`；
       - 不创建新 PR；
       - 更新跟踪评论（展示变更文件、摘要与分支链接）。

3) 失败与回退：
   - 无写权限/推送失败：回退为“分析答复”，在评论中明确原因；
   - 合并冲突/改动失败：回报失败详情与建议（如请作者 rebase）。

## 7. Prompt 注入规范（模型上下文）

```
## Discussion

@<reviewer> (<timestamp>):
_File: <path>_
```diff
<diff_hunk>
```
<用户指令文本>
```

- 所有空行保留；
- 若 `diff_hunk` 为空也要保留 File 行以指示目标文件；
- 若识别到 Line/Commit ID，附加：`Commit: <sha>`、`Line: <n>`。

## 8. 配置与权限

- 触发词：沿用现有 `triggerKeyword`（默认 `/code`）；
- 权限：GitHub App 安装并对目标仓库有写权限；
- CLI 依赖：`gh` 可用；
- Provider：沿用现有 Provider 工厂选择（Claude/Codex）。

## 9. 指标与日志

- 指标：
  - review 同分支任务触发次数 / 成功率；
  - 平均执行时长；
  - 生成改动的文件数、失败原因分布；

- 日志：
  - 在 task store 中记录 `Mode`、分支名、是否提交成功、异常摘要；

## 10. 兼容性与风险

- 兼容性：不改变现有 issue_comment 工作流；仅在 review_comment 场景切换到同分支模式；
- 主要风险：
  - 推送失败（权限/保护分支策略）；
  - diff hunk 偏移导致定位不准（建议结合文件内容做相似性匹配，模型可自行搜索）；
  - 错误提交到 base 分支（必须保证使用 head.ref）。

## 11. 验收标准（AC）

1) 在 PR 的行内评审评论中输入 `/code 修复这里的 nil 判空并补充单测`：
   - 任务创建，`Task.Branch == pull_request.head.ref`；
   - Prompt 中包含 File 与 Diff 区块；
   - 工作目录克隆的是 head 分支；
   - 有改动时：向 head 分支直接 `push`，无新 PR 创建；
   - 跟踪评论包含：分支链接、变更文件列表、摘要与费用；
   - 无改动时：以“响应-only”形式更新评论。

2) 非触发评论（无 `/code`）不会进入流程。

3) Bot 评论不会进入流程。

4) 权限不足时，评论中给出清晰错误与回退策略。

## 12. 交付增量（实现指引）

- internal/webhook/types.go：
  - 为 `PullRequest` 增加 `Head` 字段：`head.ref` 与 `head.sha`；
  - 为 `ReviewComment` 增加必要的 `line` / `start_line` / `commit_id` 字段。

- internal/webhook/handler.go：
  - `handleReviewComment`：
    - 使用 `head.ref` 作为 `Task.Branch`；
    - `Task.Mode = "review_in_branch"`；
    - 注入 `ReviewPath/ReviewDiffHunk/...` 到 Task；
    - 构建 `PromptSummary` 时标示“PR Review 修复模式”。

- internal/executor/task.go：
  - 根据 `Task.Mode` 分支：
    - `review_in_branch`：不创建新分支与 PR；直接 `git add/commit/push` 到 `Task.Branch`；
    - `standard`：维持现状（创建新分支与 PR）。
  - `composeDiscussionSection`：若 `Task.ReviewPath/ReviewDiffHunk` 存在，固定格式插入到 Discussion 区块顶部。

- 测试：
  - 新增 `internal/webhook/handler_test.go` 与 `internal/executor/task_edge_cases_test.go` 用例：
    - 验证 head.ref 选择；
    - 验证 review 模式下不创建 PR；
    - 验证 Prompt 注入包含 File/Diff；
    - 权限/失败回退路径。

## 13. 发布与回滚

- 发布：后端代码改动为无 schema 变更，部署即生效；
- 回滚：完全可回滚至旧版本，旧工作流不受影响（issue_comment 仍走原路径）。

---

附：触发评论样例

```
/code 请将此处 error 包装为 fmt.Errorf 并补充 table-driven 单测
```

