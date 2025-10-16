 ⎿  SWE Agent 完整重构计划

     参考：claude-code-action 架构 + 保留 webhook + 多 provider + 官方 MCP

     ---
     📋 核心目标

     1. ✅ 保留 GitHub Webhook 触发方式（不使用 GitHub Actions）
     2. ✅ 保留多 AI Provider 支持（Codex/Claude）
     3. ✅ 借鉴 claude-code-action 的优秀设计模式
     4. ✅ 使用官方 MCP servers（而非自定义）
     5. ✅ 简化架构，删除 2200+ 行过度设计代码

     ---
     🏗️ 新架构设计

     目录结构

     internal/
     ├── webhook/              # Webhook 处理（保留）
     │   ├── handler.go       # HTTP endpoint, HMAC verification
     │   └── types.go         # Webhook payload types
     │
     ├── github/              # GitHub 集成层（新增 - 参考 claude-code-action）
     │   ├── context.go       # Event 类型定义和解析
     │   ├── client.go        # GraphQL/REST client
     │   ├── fetcher.go       # 数据获取（issue/PR/comments）
     │   ├── formatter.go     # 格式化为 prompt
     │   ├── sanitizer.go     # 内容清理和安全
     │   └── auth.go          # GitHub App JWT（保留）
     │
     ├── prompt/              # Prompt 生成（新增 - 参考 claude-code-action）
     │   ├── builder.go       # Prompt 构建器
     │   ├── templates.go     # System prompt 模板
     │   └── formatter.go     # XML 标签格式化
     │
     ├── provider/            # AI Provider（保留 + 简化）
     │   ├── provider.go      # Interface 定义
     │   ├── codex/          # Codex CLI 实现
     │   └── claude/         # Claude API 实现（future）
     │
     ├── executor/            # 执行编排（大幅简化）
     │   └── task.go         # ~150 lines（从 1400 lines）
     │
     └── config/              # 配置管理（保留）
         └── config.go

     cmd/
     └── main.go              # HTTP 服务器入口（保留）

     删除：
     ❌ internal/agent/       # 800 lines - Agent loop
     ❌ internal/mcp/         # 550 lines - Custom MCP clients
     ❌ internal/github/comment.go, pr.go  # AI 自己处理

     ---
     📦 核心模块设计

     1. GitHub Context System (参考 claude-code-action/src/github/context.ts)

     internal/github/context.go
     package github

     // EventType 定义支持的 GitHub 事件
     type EventType string

     const (
         EventIssueComment     EventType = "issue_comment"
         EventIssues           EventType = "issues"
         EventPullRequest      EventType = "pull_request"
         EventPullRequestReview EventType = "pull_request_review"
     )

     // Context 解析后的 GitHub 事件上下文
     type Context struct {
         EventName    EventType
         EventAction  string
         Repository   Repository
         Actor        string

         // Issue/PR 标识
         IsPR         bool
         IssueNumber  int
         PRNumber     int

         // Branch 信息
         BaseBranch   string
         HeadBranch   string

         // Trigger 信息
         TriggerUser  string
         TriggerComment *Comment

         // Payload (原始 webhook 数据)
         Payload      interface{}
     }

     type Repository struct {
         Owner string
         Name  string
         FullName string
     }

     type Comment struct {
         ID        int64
         Body      string
         User      string
         CreatedAt string
         UpdatedAt string
     }

     // ParseWebhookEvent 解析 webhook payload
     func ParseWebhookEvent(eventType string, payload []byte) (*Context, error)

     2. Data Fetcher (参考 claude-code-action/src/github/data/fetcher.ts)

     internal/github/fetcher.go
     package github

     // Fetcher 负责从 GitHub API 获取完整数据
     type Fetcher struct {
         client *Client
     }

     // FetchResult 包含所有需要的 GitHub 数据
     type FetchResult struct {
         ContextData  interface{}  // Issue 或 PullRequest
         Comments     []Comment
         Reviews      []Review     // For PR only
         ChangedFiles []File       // For PR only
         TriggerUser  *User
     }

     // FetchIssueData 获取 issue 完整数据（包含 comments）
     func (f *Fetcher) FetchIssueData(ctx context.Context, owner, repo string, number int) (*FetchResult, error) {
         // 使用 GraphQL 一次性获取：
         // - Issue title, body, author, state
         // - All comments (filtered by trigger time)
         // - User info
     }

     // FetchPRData 获取 PR 完整数据
     func (f *Fetcher) FetchPRData(ctx context.Context, owner, repo string, number int) (*FetchResult, error) {
         // 使用 GraphQL 一次性获取：
         // - PR title, body, author, branches, state
         // - All comments
         // - All review comments
         // - Changed files
     }

     3. Data Formatter (参考 claude-code-action/src/github/data/formatter.ts)

     internal/github/formatter.go
     package github

     // FormatContext 格式化基本上下文信息
     func FormatContext(ctx *Context, data interface{}) string {
         if ctx.IsPR {
             pr := data.(*PullRequest)
             return fmt.Sprintf(`PR Title: %s
     PR Author: %s
     PR Branch: %s -> %s
     PR State: %s
     PR Additions: %d
     PR Deletions: %d
     Total Commits: %d
     Changed Files: %d files`,
                 pr.Title, pr.Author, pr.HeadRef, pr.BaseRef,
                 pr.State, pr.Additions, pr.Deletions,
                 pr.CommitCount, len(pr.Files))
         }

         issue := data.(*Issue)
         return fmt.Sprintf(`Issue Title: %s
     Issue Author: %s
     Issue State: %s`,
             issue.Title, issue.Author, issue.State)
     }

     // FormatComments 格式化 comments 列表
     func FormatComments(comments []Comment) string {
         var result strings.Builder
         for _, c := range comments {
             result.WriteString(fmt.Sprintf("[%s at %s]: %s\n\n",
                 c.User, c.CreatedAt, Sanitize(c.Body)))
         }
         return result.String()
     }

     // FormatReviewComments 格式化 PR review comments
     func FormatReviewComments(reviews []Review) string

     // FormatChangedFiles 格式化文件变更列表
     func FormatChangedFiles(files []File) string

     4. Prompt Builder (参考 claude-code-action/src/create-prompt/index.ts)

     internal/prompt/builder.go
     package prompt

     // Builder 负责生成完整的 AI prompt
     type Builder struct {
         systemPrompt string
     }

     // BuildPrompt 构建完整 prompt（参考 claude-code-action generateDefaultPrompt）
     func (b *Builder) BuildPrompt(ctx *github.Context, data *github.FetchResult) string {
         var prompt strings.Builder

         // Load system prompt from file
         systemPrompt, _ := os.ReadFile("system-prompt.md")
         prompt.WriteString(string(systemPrompt))
         prompt.WriteString("\n\n---\n\n")

         // Add structured context (XML tags)
         prompt.WriteString("<formatted_context>\n")
         prompt.WriteString(github.FormatContext(ctx, data.ContextData))
         prompt.WriteString("\n</formatted_context>\n\n")

         prompt.WriteString("<pr_or_issue_body>\n")
         prompt.WriteString(github.Sanitize(data.ContextData.Body))
         prompt.WriteString("\n</pr_or_issue_body>\n\n")

         prompt.WriteString("<comments>\n")
         prompt.WriteString(github.FormatComments(data.Comments))
         prompt.WriteString("\n</comments>\n\n")

         if ctx.IsPR {
             prompt.WriteString("<review_comments>\n")
             prompt.WriteString(github.FormatReviewComments(data.Reviews))
             prompt.WriteString("\n</review_comments>\n\n")

             prompt.WriteString("<changed_files>\n")
             prompt.WriteString(github.FormatChangedFiles(data.ChangedFiles))
             prompt.WriteString("\n</changed_files>\n\n")
         }

         // Add metadata tags
         prompt.WriteString(fmt.Sprintf("<repository>%s</repository>\n", ctx.Repository.FullName))
         prompt.WriteString(fmt.Sprintf("<issue_number>%d</issue_number>\n", ctx.IssueNumber))
         prompt.WriteString(fmt.Sprintf("<base_branch>%s</base_branch>\n", ctx.BaseBranch))

         if ctx.TriggerComment != nil {
             prompt.WriteString("<trigger_comment>\n")
             prompt.WriteString(github.Sanitize(ctx.TriggerComment.Body))
             prompt.WriteString("\n</trigger_comment>\n")
         }

         return prompt.String()
     }

     5. Simplified Executor

     internal/executor/task.go (~150 lines)
     package executor

     type Executor struct {
         provider provider.Provider
         auth     github.Authenticator
         fetcher  *github.Fetcher
         builder  *prompt.Builder
     }

     func (e *Executor) Execute(ctx context.Context, webhookCtx *github.Context) error {
         // 1. Authenticate
         token, err := e.auth.GetInstallationToken(ctx, webhookCtx.Repository.FullName)

         // 2. Fetch GitHub data
         data, err := e.fetcher.FetchData(ctx, webhookCtx)

         // 3. Clone repository
         workdir, cleanup := e.cloneRepo(webhookCtx.Repository, webhookCtx.BaseBranch, token)
         defer cleanup()

         // 4. Create feature branch
         branchName := fmt.Sprintf("swe/issue-%d-%d", webhookCtx.IssueNumber, time.Now().Unix())
         e.createBranch(workdir, branchName)

         // 5. Build prompt
         fullPrompt := e.builder.BuildPrompt(webhookCtx, data)

         // 6. Call AI provider (MCP tools pre-configured)
         result, err := e.provider.GenerateCode(ctx, &provider.CodeRequest{
             Prompt:   fullPrompt,
             RepoPath: workdir,
             Context: map[string]string{
                 "repository":    webhookCtx.Repository.FullName,
                 "issue_number":  fmt.Sprintf("%d", webhookCtx.IssueNumber),
                 "branch":        branchName,
                 "base_branch":   webhookCtx.BaseBranch,
                 "github_token":  token.Token,
             },
         })

         // 7. Done! AI handles everything via MCP
         log.Printf("Task completed: %s", result.Summary)
         return nil
     }

     6. Updated system-prompt.md (参考 claude-code-action prompt 结构)

     # SWE Agent System Prompt

     You are an autonomous software engineering agent solving GitHub issues and PRs.

     ## Context Format

     You will receive context in XML tags:

     <formatted_context>
     Issue/PR metadata (title, author, state, etc.)
     </formatted_context>

     <pr_or_issue_body>
     Full issue or PR description
     </pr_or_issue_body>

     <comments>
     All comments on this issue/PR
     </comments>

     <trigger_comment>
     Your specific task instruction
     </trigger_comment>

     ## Your Workflow

     1. **Create Task List**
        - Post a comment with checkbox tasks using `add_issue_comment`
        - Example: "## Progress\n- [ ] Task 1\n- [ ] Task 2"

     2. **Gather Context**
        - All information is provided above in XML tags
        - Read <trigger_comment> for your specific task
        - Use Read tool to examine code files

     3. **Implement Changes**
        - Edit files directly
        - Follow existing code style
        - Make focused, minimal changes

     4. **Test Changes**
        - Run tests if available
        - Verify implementation works

     5. **Commit and Push**
        - Use `git_add` to stage files
        - Use `git_commit` with clear message
        - Git push is handled automatically

     6. **Create Pull Request**
        - Use `create_pull_request` tool
        - Title: "Fix #42: Description"
        - Body: Summary + test plan

     7. **Update Issue**
        - Use `add_issue_comment` to post PR link
        - Update task checklist as you progress

     ## CRITICAL RULES

     - Your console outputs are NOT visible to users
     - ALL communication MUST go through `add_issue_comment`
     - Update your task checklist by posting new comments
     - Use checkbox format: "- [ ]" for incomplete, "- [x]" for complete

     ## Available Tools

     ### Git Tools (mcp-server-git)
     - `git_status`, `git_add`, `git_commit`, `git_diff`, `git_log`, `git_branch`

     ### GitHub Tools (github-mcp-server)
     - `get_issue` - Fetch issue details
     - `add_issue_comment` - Post comments (YOUR ONLY WAY TO COMMUNICATE)
     - `create_pull_request` - Create PR
     - `list_issues` - List issues

     ## Tool Usage Examples

     ### Post Progress Update
     ```json
     {
       "issue_number": 42,
       "body": "## Implementation Progress\n\n- [x] Read issue\n- [x] Plan solution\n- [ ] Implement code\n- [ ] Create PR"
     }

     Create Pull Request

     {
       "title": "Fix #42: Add user authentication",
       "body": "## Summary\nImplemented JWT authentication with bcrypt.\n\n## Changes\n- auth.go: Register/Login functions\n- auth_test.go: Unit tests",
       "base": "main",
       "head": "swe/issue-42-xxx"
     }

     ---

     ## 🔄 实施步骤

     ### Phase 1: 创建 GitHub 数据层（4 小时）

     **文件创建：**
     ```bash
     internal/github/
     ├── context.go       # Event 类型和解析 (~200 lines)
     ├── client.go        # GraphQL client (~100 lines)
     ├── fetcher.go       # 数据获取 (~300 lines)
     ├── formatter.go     # 格式化 (~200 lines)
     └── sanitizer.go     # 内容清理 (~100 lines)

     实现要点：
     1. Context 类型定义（参考 claude-code-action/src/github/context.ts）
     2. GraphQL 查询（参考 queries/github.ts）
     3. Fetcher 实现（参考 data/fetcher.ts）
     4. Formatter 实现（参考 data/formatter.ts）
     5. Sanitizer 实现（参考 utils/sanitizer.ts）

     Phase 2: 实现 Prompt 生成（2 小时）

     文件创建：
     internal/prompt/
     ├── builder.go       # Prompt 构建 (~200 lines)
     ├── templates.go     # 模板管理 (~50 lines)
     └── builder_test.go  # 单元测试

     实现要点：
     1. XML 标签格式（参考 generateDefaultPrompt）
     2. 结构化 prompt 生成
     3. 集成 system-prompt.md
     4. 测试用例验证

     Phase 3: 简化 Executor（1 小时）

     文件修改：
     internal/executor/task.go  # 1400 lines → 150 lines

     实现要点：
     1. 删除 Agent Loop 逻辑
     2. 删除 MCP 客户端管理
     3. 使用新的 Fetcher + Builder
     4. 简化为线性执行流程

     Phase 4: 更新 Provider（30 分钟）

     文件修改：
     internal/provider/provider.go  # 更新接口
     internal/provider/codex/provider.go  # 使用新的 prompt

     实现要点：
     1. Provider 接口添加 Context 参数
     2. Codex provider 传递 GitHub token 给 MCP
     3. 删除旧的 prompt 构建逻辑

     Phase 5: 删除过时代码（30 分钟）

     rm -rf internal/agent/           # 800 lines
     rm -rf internal/mcp/             # 550 lines
     rm internal/github/comment.go    # 150 lines
     rm internal/github/pr.go         # 150 lines
     rm internal/github/clone.go      # 100 lines (简化保留)
     rm -rf cmd/codex-webhook/mock_server.go
     rm -rf cmd/codex-webhook/scenarios/

     Phase 6: 测试验证（2 小时）

     # 1. 单元测试
     go test ./internal/github/...
     go test ./internal/prompt/...

     # 2. 集成测试
     # 创建真实 GitHub issue
     # 触发 webhook
     # 验证完整流程

     # 3. 功能验证
     - ✅ Webhook 正常接收
     - ✅ Context 解析正确
     - ✅ GraphQL 数据获取完整
     - ✅ Prompt 格式正确（XML tags）
     - ✅ AI 调用 MCP 工具
     - ✅ PR 创建成功
     - ✅ Comment 更新正确

     ---
     📊 预期效果

     代码量变化

     | 模块        | Before     | After      | 减少           |
     |-----------|------------|------------|--------------|
     | agent/    | 800 lines  | 0          | -100%        |
     | mcp/      | 550 lines  | 0          | -100%        |
     | github/   | 400 lines  | 900 lines  | +125% (新增功能) |
     | prompt/   | 0          | 250 lines  | 新增           |
     | executor/ | 1400 lines | 150 lines  | -89%         |
     | Total     | 3150 lines | 1300 lines | -59%         |

     架构优势

     Before（当前架构）：
     - ❌ 自定义 Agent Loop（不工作）
     - ❌ 自定义 MCP 客户端（复杂）
     - ❌ 工具调用格式错误
     - ❌ 缺少数据格式化
     - ❌ 缺少安全清理

     After（重构后）：
     - ✅ 简单线性流程
     - ✅ 使用官方 MCP servers
     - ✅ 标准 XML prompt 格式
     - ✅ 完整数据格式化
     - ✅ 安全 sanitization
     - ✅ 参考成熟项目设计

     ---
     🎯 Timeline

     - Phase 1 (GitHub 数据层): 4 小时
     - Phase 2 (Prompt 生成): 2 小时
     - Phase 3 (Executor 简化): 1 小时
     - Phase 4 (Provider 更新): 30 分钟
     - Phase 5 (删除代码): 30 分钟
     - Phase 6 (测试验证): 2 小时

     Total: 10 小时（1.5 工作日）

     ---
     ✅ 成功标准

     1. 代码质量
       - ✅ 删除 50%+ 代码
       - ✅ 类型安全（Context, FetchResult）
       - ✅ 单元测试覆盖 >70%
     2. 功能完整
       - ✅ Webhook 触发正常
       - ✅ GraphQL 数据完整
       - ✅ Prompt 格式正确（XML）
       - ✅ AI 调用工具成功
       - ✅ PR/Comment 创建正常
     3. 架构清晰
       - ✅ 模块职责单一
       - ✅ 参考成熟设计
       - ✅ 易于维护扩展

     ---
     核心理念：借鉴 claude-code-action 的优秀设计，保留我们的简洁架构（webhook + 官方 MCP）。
