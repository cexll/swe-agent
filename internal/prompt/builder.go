package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ghdata "github.com/cexll/swe/internal/github/data"
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
	eventType, triggerCtx := eventTypeAndTriggerContext(ctx, DefaultTriggerPhrase)

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
func eventTypeAndTriggerContext(ctx GitHubContext, triggerPhrase string) (eventType, triggerContext string) {
	switch ctx.GetEventName() {
	case "pull_request_review_comment":
		return "REVIEW_COMMENT", fmt.Sprintf("PR review comment with '%s'", triggerPhrase)
	case "pull_request_review":
		return "PR_REVIEW", fmt.Sprintf("PR review with '%s'", triggerPhrase)
	case "issue_comment":
		return "GENERAL_COMMENT", fmt.Sprintf("issue comment with '%s'", triggerPhrase)
	case "issues":
		switch ctx.GetEventAction() {
		case "opened":
			return "ISSUE_CREATED", fmt.Sprintf("new issue with '%s' in body", triggerPhrase)
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
