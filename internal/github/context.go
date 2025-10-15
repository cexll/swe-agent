package github

import (
	"encoding/json"
	"fmt"
	"strings"
)

// EventType defines supported GitHub webhook events
type EventType string

const (
	EventIssueComment             EventType = "issue_comment"
	EventIssues                   EventType = "issues"
	EventPullRequest              EventType = "pull_request"
	EventPullRequestReview        EventType = "pull_request_review"
	EventPullRequestReviewComment EventType = "pull_request_review_comment"
)

// EventAction defines GitHub event actions
type EventAction string

const (
	ActionOpened   EventAction = "opened"
	ActionClosed   EventAction = "closed"
	ActionCreated  EventAction = "created"
	ActionEdited   EventAction = "edited"
	ActionAssigned EventAction = "assigned"
	ActionLabeled  EventAction = "labeled"
)

// Context represents parsed GitHub webhook event context
type Context struct {
	EventName   EventType
	EventAction EventAction
	Repository  Repository
	Actor       string

	// Issue/PR identification
	IsPR        bool
	IssueNumber int
	PRNumber    int

	// Branch information
	BaseBranch string
	HeadBranch string

	// Trigger information
	TriggerUser    string
	TriggerComment *Comment

	// Raw payload for additional data
	Payload interface{}
}

// Repository represents a GitHub repository
type Repository struct {
	Owner    string
	Name     string
	FullName string
}

// Comment represents a GitHub comment
type Comment struct {
	ID        int64
	Body      string
	User      string
	CreatedAt string
	UpdatedAt string
}

// Issue represents a GitHub issue
type Issue struct {
	Number    int
	Title     string
	Body      string
	State     string
	Author    string
	CreatedAt string
	UpdatedAt string
	Comments  []Comment
}

// PullRequest represents a GitHub pull request
type PullRequest struct {
	Number      int
	Title       string
	Body        string
	State       string
	Author      string
	BaseRef     string
	HeadRef     string
	Additions   int
	Deletions   int
	CommitCount int
	Files       []File
	Comments    []Comment
	Reviews     []Review
}

// File represents a changed file in a PR
type File struct {
	Path       string
	ChangeType string // added, modified, removed
	Additions  int
	Deletions  int
	SHA        string
}

// Review represents a PR review
type Review struct {
	ID          int64
	Author      string
	State       string
	Body        string
	SubmittedAt string
	Comments    []ReviewComment
}

// ReviewComment represents a comment on a PR review
type ReviewComment struct {
	ID        int64
	Path      string
	Line      int
	Body      string
	Author    string
	CreatedAt string
}

// User represents a GitHub user
type User struct {
	Login string
	Name  string
	Email string
}

// ParseWebhookEvent parses a GitHub webhook event into Context
func ParseWebhookEvent(eventType string, payload []byte) (*Context, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	ctx := &Context{
		EventName: EventType(eventType),
		Payload:   data,
	}

	// Parse repository
	if repo, ok := data["repository"].(map[string]interface{}); ok {
		ctx.Repository = Repository{
			Owner:    getStringField(repo, "owner", "login"),
			Name:     getStringField(repo, "name"),
			FullName: getStringField(repo, "full_name"),
		}
	}

	// Parse sender/actor
	if sender, ok := data["sender"].(map[string]interface{}); ok {
		ctx.Actor = getStringField(sender, "login")
		ctx.TriggerUser = ctx.Actor
	}

	// Parse event-specific data
	switch EventType(eventType) {
	case EventIssueComment:
		return parseIssueComment(ctx, data)
	case EventIssues:
		return parseIssues(ctx, data)
	case EventPullRequest:
		return parsePullRequest(ctx, data)
	case EventPullRequestReview:
		return parsePullRequestReview(ctx, data)
	case EventPullRequestReviewComment:
		return parsePullRequestReviewComment(ctx, data)
	default:
		return nil, fmt.Errorf("unsupported event type: %s", eventType)
	}
}

func parseIssueComment(ctx *Context, data map[string]interface{}) (*Context, error) {
	ctx.EventAction = EventAction(getStringField(data, "action"))

	// Parse comment
	if comment, ok := data["comment"].(map[string]interface{}); ok {
		ctx.TriggerComment = &Comment{
			ID:        int64(getNumberField(comment, "id")),
			Body:      getStringField(comment, "body"),
			User:      getStringField(comment, "user", "login"),
			CreatedAt: getStringField(comment, "created_at"),
			UpdatedAt: getStringField(comment, "updated_at"),
		}
	}

	// Determine if this is an issue or PR
	if issue, ok := data["issue"].(map[string]interface{}); ok {
		ctx.IssueNumber = int(getNumberField(issue, "number"))

		// Check if issue is actually a PR
		if pullRequest, hasPR := issue["pull_request"]; hasPR && pullRequest != nil {
			ctx.IsPR = true
			ctx.PRNumber = ctx.IssueNumber
		}
	}

	return ctx, nil
}

func parseIssues(ctx *Context, data map[string]interface{}) (*Context, error) {
	ctx.EventAction = EventAction(getStringField(data, "action"))
	ctx.IsPR = false

	if issue, ok := data["issue"].(map[string]interface{}); ok {
		ctx.IssueNumber = int(getNumberField(issue, "number"))
	}

	return ctx, nil
}

func parsePullRequest(ctx *Context, data map[string]interface{}) (*Context, error) {
	ctx.EventAction = EventAction(getStringField(data, "action"))
	ctx.IsPR = true

	if pr, ok := data["pull_request"].(map[string]interface{}); ok {
		ctx.PRNumber = int(getNumberField(pr, "number"))
		ctx.IssueNumber = ctx.PRNumber

		if base, ok := pr["base"].(map[string]interface{}); ok {
			ctx.BaseBranch = getStringField(base, "ref")
		}
		if head, ok := pr["head"].(map[string]interface{}); ok {
			ctx.HeadBranch = getStringField(head, "ref")
		}
	}

	return ctx, nil
}

func parsePullRequestReview(ctx *Context, data map[string]interface{}) (*Context, error) {
	ctx.EventAction = EventAction(getStringField(data, "action"))
	ctx.IsPR = true

	if pr, ok := data["pull_request"].(map[string]interface{}); ok {
		ctx.PRNumber = int(getNumberField(pr, "number"))
		ctx.IssueNumber = ctx.PRNumber

		if base, ok := pr["base"].(map[string]interface{}); ok {
			ctx.BaseBranch = getStringField(base, "ref")
		}
		if head, ok := pr["head"].(map[string]interface{}); ok {
			ctx.HeadBranch = getStringField(head, "ref")
		}
	}

	// Parse review comment
	if review, ok := data["review"].(map[string]interface{}); ok {
		ctx.TriggerComment = &Comment{
			ID:        int64(getNumberField(review, "id")),
			Body:      getStringField(review, "body"),
			User:      getStringField(review, "user", "login"),
			CreatedAt: getStringField(review, "submitted_at"),
		}
	}

	return ctx, nil
}

func parsePullRequestReviewComment(ctx *Context, data map[string]interface{}) (*Context, error) {
	ctx.EventAction = EventAction(getStringField(data, "action"))
	ctx.IsPR = true

	if pr, ok := data["pull_request"].(map[string]interface{}); ok {
		ctx.PRNumber = int(getNumberField(pr, "number"))
		ctx.IssueNumber = ctx.PRNumber

		if base, ok := pr["base"].(map[string]interface{}); ok {
			ctx.BaseBranch = getStringField(base, "ref")
		}
		if head, ok := pr["head"].(map[string]interface{}); ok {
			ctx.HeadBranch = getStringField(head, "ref")
		}
	}

	// Parse review comment
	if comment, ok := data["comment"].(map[string]interface{}); ok {
		ctx.TriggerComment = &Comment{
			ID:        int64(getNumberField(comment, "id")),
			Body:      getStringField(comment, "body"),
			User:      getStringField(comment, "user", "login"),
			CreatedAt: getStringField(comment, "created_at"),
			UpdatedAt: getStringField(comment, "updated_at"),
		}
	}

	return ctx, nil
}

// ShouldTrigger determines if the event should trigger the agent
func (c *Context) ShouldTrigger(triggerPhrase string) bool {
	if c.TriggerComment == nil {
		return false
	}

	// Check if comment body contains trigger phrase
	return strings.Contains(c.TriggerComment.Body, triggerPhrase)
}

// ExtractPrompt extracts custom prompt from trigger comment
func (c *Context) ExtractPrompt(triggerPhrase string) string {
	if c.TriggerComment == nil {
		return ""
	}

	body := c.TriggerComment.Body
	idx := strings.Index(body, triggerPhrase)
	if idx == -1 {
		return ""
	}

	// Extract text after trigger phrase
	prompt := strings.TrimSpace(body[idx+len(triggerPhrase):])
	return prompt
}

// --- Interface helpers for prompt builder ---

// GetEventName returns the GitHub event name as a string.
func (c *Context) GetEventName() string { return string(c.EventName) }

// GetEventAction returns the GitHub event action as a string.
func (c *Context) GetEventAction() string { return string(c.EventAction) }

// GetRepositoryFullName returns owner/name if available.
func (c *Context) GetRepositoryFullName() string { return c.Repository.FullName }

// GetRepositoryOwner returns the repository owner login.
func (c *Context) GetRepositoryOwner() string { return c.Repository.Owner }

// GetRepositoryName returns the repository name.
func (c *Context) GetRepositoryName() string { return c.Repository.Name }

// IsPRContext reports whether the current context is a PR.
func (c *Context) IsPRContext() bool { return c.IsPR }

// GetIssueNumber returns the issue number for the context (PRs reuse issue numbering).
func (c *Context) GetIssueNumber() int { return c.IssueNumber }

// GetPRNumber returns the pull request number when applicable.
func (c *Context) GetPRNumber() int { return c.PRNumber }

// GetBaseBranch returns the base branch for PRs.
func (c *Context) GetBaseBranch() string { return c.BaseBranch }

// GetHeadBranch returns the head branch for PRs.
func (c *Context) GetHeadBranch() string { return c.HeadBranch }

// GetTriggerUser returns the login of the user that triggered the event.
func (c *Context) GetTriggerUser() string { return c.TriggerUser }

// GetActor returns the actor login from the event payload.
func (c *Context) GetActor() string { return c.Actor }

// GetTriggerCommentBody returns the body of the trigger comment if present.
func (c *Context) GetTriggerCommentBody() string {
	if c.TriggerComment == nil {
		return ""
	}
	return c.TriggerComment.Body
}

// Helper functions for safe map access
func getStringField(data map[string]interface{}, keys ...string) string {
	current := data
	for i, key := range keys {
		if i == len(keys)-1 {
			if val, ok := current[key].(string); ok {
				return val
			}
			return ""
		}
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return ""
		}
	}
	return ""
}

func getNumberField(data map[string]interface{}, keys ...string) float64 {
	current := data
	for i, key := range keys {
		if i == len(keys)-1 {
			if val, ok := current[key].(float64); ok {
				return val
			}
			return 0
		}
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return 0
		}
	}
	return 0
}
