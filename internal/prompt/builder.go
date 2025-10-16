package prompt

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/cexll/swe/internal/github"
	ghdata "github.com/cexll/swe/internal/github/data"
	"github.com/cexll/swe/internal/github/image"
)

// DefaultTriggerPhrase is used when no explicit trigger phrase is available.
const DefaultTriggerPhrase = "@assistant"

// LoadSystemPrompt reads the repository-level system prompt from system-prompt.md.
//
// Search strategy:
// 1) Try CWD-relative path (service runs from repo root in normal workflows).
// 2) If not found, try to locate by walking up from CWD until filesystem root.
//
// Returns the file contents on success. If the file cannot be found, returns a
// minimal fallback prompt and a descriptive error so callers may log it.
func LoadSystemPrompt() (string, error) {
	const filename = "system-prompt.md"

	// First, try direct read from current working directory.
	if b, err := os.ReadFile(filename); err == nil {
		return string(b), nil
	}

	// If that fails (e.g. service not started from repo root), walk upwards.
	dir, _ := os.Getwd()
	for {
		candidate := filepath.Join(dir, filename)
		if b, err := os.ReadFile(candidate); err == nil {
			return string(b), nil
		}

		parent := filepath.Dir(dir)
		if parent == dir { // reached filesystem root
			break
		}
		dir = parent
	}

	// Fallback content to avoid hard failures during runtime.
	fallback := "You are an AI assistant. Use the provided GitHub context below to reason and act."
	return fallback, fmt.Errorf("system prompt file %q not found via CWD or parent directories", filename)
}

// BuildPrompt constructs the final model prompt by concatenating:
//   - The system prompt markdown (from system-prompt.md)
//   - A separator line `---`
//   - XML-tagged, formatted GitHub context via ghdata.GenerateXML
//
// It handles both PR and Issue events and includes key metadata tags
// (repository, issue/pr number, event type, trigger comment, etc.).
func BuildPrompt(ctx GitHubContext, fetched *ghdata.FetchResult) string {
	// Load system prompt; don't fail hard if missing.
	systemPrompt, _ := LoadSystemPrompt()

	// Derive event type and human-readable trigger context.
    eventType, triggerCtx := eventTypeAndTriggerContext(ctx)

	// Infer repository full name (owner/name).
	repoFull := ctx.GetRepositoryFullName()
	if strings.TrimSpace(repoFull) == "" {
		if ctx.GetRepositoryOwner() != "" && ctx.GetRepositoryName() != "" {
			repoFull = ctx.GetRepositoryOwner() + "/" + ctx.GetRepositoryName()
		}
	}
	// Determine entity number.
	number := ctx.GetIssueNumber()
	if ctx.IsPRContext() && ctx.GetPRNumber() != 0 {
		number = ctx.GetPRNumber()
	}

	// Trigger username and display name.
	triggerUsername := ctx.GetTriggerUser()
	if triggerUsername == "" {
		triggerUsername = ctx.GetActor()
	}
	var triggerDisplayName string
	if fetched != nil && fetched.TriggerName != nil {
		triggerDisplayName = *fetched.TriggerName
	}

	// Trigger comment body, if available.
	triggerComment := ctx.GetTriggerCommentBody()

	// Build XML using the shared formatter so output matches claude-code-action.
	xml := ghdata.GenerateXML(ghdata.GenerateXMLParams{
		Repository:         repoFull,
		IsPR:               ctx.IsPRContext(),
		Number:             number,
		EventType:          eventType,
		TriggerContext:     triggerCtx,
		TriggerUsername:    triggerUsername,
		TriggerDisplayName: triggerDisplayName,
		TriggerPhrase:      DefaultTriggerPhrase,
		TriggerComment:     triggerComment,
		ClaudeCommentID:    "", // Not tracked here; providers may inject in higher layers
		BaseBranch:         ctx.GetBaseBranch(),

		ContextData:         fetchedContextData(fetched),
		Comments:            fetchedComments(fetched),
		ReviewData:          fetchedReviews(fetched),
		ChangedFilesWithSHA: fetchedChangedWithSHA(fetched),
		ImageURLMap:         fetchedImageMap(fetched),
	})

	var b strings.Builder
	b.WriteString(systemPrompt)
	b.WriteString("\n\n---\n\n")
	b.WriteString(xml)
	return b.String()
}

// fetchedContextData safely returns the ContextData or a zero value to satisfy
// the downstream formatter's expectations.
func fetchedContextData(fr *ghdata.FetchResult) interface{} {
	if fr == nil {
		return nil
	}
	return fr.ContextData
}

func fetchedComments(fr *ghdata.FetchResult) []ghdata.Comment {
	if fr == nil {
		return nil
	}
	return fr.Comments
}

func fetchedReviews(fr *ghdata.FetchResult) *struct{ Nodes []ghdata.Review } {
	if fr == nil {
		return nil
	}
	return fr.Reviews
}

func fetchedChangedWithSHA(fr *ghdata.FetchResult) []ghdata.GitHubFileWithSHA {
	if fr == nil {
		return nil
	}
	return fr.ChangedSHA
}

func fetchedImageMap(fr *ghdata.FetchResult) map[string]string {
	if fr == nil {
		return nil
	}
	return fr.ImageURLMap
}

// eventTypeAndTriggerContext mirrors the mapping from claude-code-action's
// getEventTypeAndContext to keep downstream prompts consistent.
func eventTypeAndTriggerContext(ctx GitHubContext) (eventType, triggerContext string) {
	switch ctx.GetEventName() {
    case "pull_request_review_comment":
        return "REVIEW_COMMENT", fmt.Sprintf("PR review comment with '%s'", DefaultTriggerPhrase)
    case "pull_request_review":
        return "PR_REVIEW", fmt.Sprintf("PR review with '%s'", DefaultTriggerPhrase)
    case "issue_comment":
        return "GENERAL_COMMENT", fmt.Sprintf("issue comment with '%s'", DefaultTriggerPhrase)
	case "issues":
		switch ctx.GetEventAction() {
        case "opened":
            return "ISSUE_CREATED", fmt.Sprintf("new issue with '%s' in body", DefaultTriggerPhrase)
		case "labeled":
			// Label value isn't available here; keep a generic context string
			return "ISSUE_LABELED", "issue labeled event"
		case "assigned":
			return "ISSUE_ASSIGNED", "issue assigned event"
		default:
			return "ISSUES", "issues event"
		}
	case "pull_request":
		fallthrough
	default:
		// treat all PR-like events uniformly; include action if present
		if ctx.GetEventName() == "pull_request" || ctx.GetEventName() == "pull_request_target" {
			if ctx.GetEventAction() != "" {
				return "PULL_REQUEST", fmt.Sprintf("pull request %s", ctx.GetEventAction())
			}
			return "PULL_REQUEST", "pull request event"
		}
		// Fallback for any unexpected/unsupported event names
		return strings.ToUpper(ctx.GetEventName()), "generic event"
	}
}

// GitHubContext abstracts the minimal fields needed to build the prompt.
//
// This interface intentionally mirrors data from internal/github.Context while
// avoiding importing that package here to prevent an import cycle
// (internal/prompt -> internal/github -> internal/provider/claude -> internal/prompt).
type GitHubContext interface {
	GetEventName() string
	GetEventAction() string

	GetRepositoryFullName() string
	GetRepositoryOwner() string
	GetRepositoryName() string

	IsPRContext() bool
	GetIssueNumber() int
	GetPRNumber() int

	GetBaseBranch() string
	GetHeadBranch() string

	GetTriggerUser() string
	GetActor() string
	GetTriggerCommentBody() string
}

// BuildFullPrompt 构建完整 Prompt（基于模板）
// 参考 claude-code-action 的 generateDefaultPrompt
func BuildFullPrompt(ctx context.Context, ghCtx *github.Context, commentID int64, branch string) (string, error) {
	// 1. 下载评论中的图片（非阻塞：失败仅打印警告）
	imageInfo := ""
	if body := ghCtx.GetTriggerCommentBody(); strings.TrimSpace(body) != "" {
		imageURLs := image.ExtractImageURLs(body)
		if len(imageURLs) > 0 {
			downloader, err := image.NewDownloader("")
			if err != nil {
				fmt.Printf("Warning: Failed to create image downloader: %v\n", err)
			} else {
				urlMap, err := downloader.DownloadImages(ctx, imageURLs)
				if err != nil {
					fmt.Printf("Warning: Failed to download images: %v\n", err)
				} else if len(urlMap) > 0 {
					imageInfo = buildImageInfo(urlMap)
				}
			}
		}
	}

	// 2. 准备数据
	data := PromptData{
		FormattedContext: formatContext(ghCtx),
		IssueBody:        "", // 无 issue/PR body 字段，保持为空以兼容模板
		Comments:         formatComments(ghCtx),
		EventType:        "GENERAL_COMMENT",
		IsPR:             ghCtx.IsPRContext(),
		TriggerContext:   "issue comment with '/code'",
		Repository:       fmt.Sprintf("%s/%s", ghCtx.GetRepositoryOwner(), ghCtx.GetRepositoryName()),
		IssueNumber:      ghCtx.GetIssueNumber(),
		CommentID:        commentID,
		Owner:            ghCtx.GetRepositoryOwner(),
		Repo:             ghCtx.GetRepositoryName(),
		Branch:           branch,
		BaseBranch:       ghCtx.GetBaseBranch(),
		ImageInfo:        imageInfo,
	}

	// 解析模板
	tmpl, err := template.New("prompt").Parse(DefaultPromptTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// 执行模板
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// buildImageInfo 构建图片信息文本
func buildImageInfo(urlMap map[string]string) string {
	if len(urlMap) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n<images_info>\n")
	sb.WriteString("Images have been downloaded from comments and saved to disk. You can use the Read tool to view these images.\n\n")
	sb.WriteString("Image mappings:\n")

	for originalURL, localPath := range urlMap {
		sb.WriteString(fmt.Sprintf("- Original: %s\n", originalURL))
		sb.WriteString(fmt.Sprintf("  Local: %s\n", localPath))
	}

	sb.WriteString("</images_info>")
	return sb.String()
}

// formatContext 格式化 GitHub 上下文（尽量不引入新字段，保持健壮）
func formatContext(ctx *github.Context) string {
	repoOwner := ctx.GetRepositoryOwner()
	repoName := ctx.GetRepositoryName()
	number := ctx.GetIssueNumber()
	if ctx.IsPRContext() && ctx.GetPRNumber() != 0 {
		number = ctx.GetPRNumber()
	}
	title := ""
	author := ctx.GetActor()
	created := ""
	if cbody := ctx.GetTriggerCommentBody(); cbody != "" {
		// No timestamp field on Context; keep empty string to avoid misleading data
		_ = cbody
	}

	return fmt.Sprintf(`Repository: %s/%s
Issue/PR: #%d
Title: %s
Author: %s
Created: %s`,
		repoOwner,
		repoName,
		number,
		title,
		author,
		created,
	)
}

// formatComments 格式化评论列表（当前仅使用触发评论作为上下文）
func formatComments(ctx *github.Context) string {
	body := ctx.GetTriggerCommentBody()
	if strings.TrimSpace(body) == "" {
		return "No comments"
	}
	user := ctx.GetTriggerUser()
	if user == "" {
		user = ctx.GetActor()
	}
	return fmt.Sprintf(`Comment by %s:
%s`, user, body)
}
