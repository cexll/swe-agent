package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

const (
	// UseIssueContentMarker is returned by extractPrompt when no custom prompt is provided
	UseIssueContentMarker = "__USE_ISSUE_CONTENT__"
)

// Task represents a pilot task to be executed
type Task struct {
	Repo       string
	Number     int
	Branch     string
	Prompt     string
	IssueTitle string
	IssueBody  string
	IsPR       bool
	Username   string // User who triggered the task
}

// Executor interface for task execution
type Executor interface {
	Execute(ctx context.Context, task *Task) error
}

// Handler handles GitHub webhook events
type Handler struct {
	webhookSecret  string
	triggerKeyword string
	executor       Executor
}

// NewHandler creates a new webhook handler
func NewHandler(webhookSecret, triggerKeyword string, executor Executor) *Handler {
	return &Handler{
		webhookSecret:  webhookSecret,
		triggerKeyword: triggerKeyword,
		executor:       executor,
	}
}

// HandleIssueComment handles issue_comment webhook events
func (h *Handler) HandleIssueComment(w http.ResponseWriter, r *http.Request) {
	// 1. Read payload
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading payload: %v", err)
		http.Error(w, "Error reading payload", http.StatusBadRequest)
		return
	}

	// 2. Verify signature
	signature := r.Header.Get("X-Hub-Signature-256")
	if err := ValidateSignatureHeader(signature); err != nil {
		log.Printf("Invalid signature header: %v", err)
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	if !VerifySignature(payload, signature, h.webhookSecret) {
		log.Printf("Signature verification failed")
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// 3. Parse event
	var event IssueCommentEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("Error parsing event: %v", err)
		http.Error(w, "Error parsing event", http.StatusBadRequest)
		return
	}

	// 4. Check if comment is from a bot (prevent infinite loops)
	if event.Comment.User.Type == "Bot" {
		log.Printf("Ignoring comment from bot: %s", event.Comment.User.Login)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Bot comment ignored"))
		return
	}

	// 5. Check if comment contains trigger keyword
	if !strings.Contains(event.Comment.Body, h.triggerKeyword) {
		log.Printf("Comment does not contain trigger keyword '%s'", h.triggerKeyword)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("No trigger keyword found"))
		return
	}

	// 6. Extract prompt from comment
	prompt := extractPrompt(event.Comment.Body, h.triggerKeyword)
	if prompt == "" {
		log.Printf("No prompt found after trigger keyword")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("No prompt found"))
		return
	}

	// 6.5. If no custom prompt provided, use issue content as prompt
	if prompt == UseIssueContentMarker {
		// Construct prompt from issue title and body
		prompt = fmt.Sprintf("# %s\n\n%s", event.Issue.Title, event.Issue.Body)
		log.Printf("Using issue content as prompt (title + body, %d chars)", len(prompt))
	}

	// 7. Check if this is a PR or issue
	isPR := event.Issue.PullRequest != nil

	// 8. Create task
	task := &Task{
		Repo:       event.Repository.FullName,
		Number:     event.Issue.Number,
		Branch:     event.Repository.DefaultBranch,
		Prompt:     prompt,
		IssueTitle: event.Issue.Title,
		IssueBody:  event.Issue.Body,
		IsPR:       isPR,
		Username:   event.Comment.User.Login,
	}

	log.Printf("Received task: repo=%s, number=%d, prompt=%s", task.Repo, task.Number, task.Prompt)

	// 9. Execute asynchronously (return 202 immediately)
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(fmt.Sprintf("Task accepted for processing: %s", prompt)))

	// Execute in background
	go func() {
		if err := h.executor.Execute(context.Background(), task); err != nil {
			log.Printf("Error executing task: %v", err)
		}
	}()
}

// extractPrompt extracts the prompt text after the trigger keyword
// Returns UseIssueContentMarker if no custom prompt is provided after the trigger
func extractPrompt(body, triggerKeyword string) string {
	// Find the trigger keyword
	idx := strings.Index(body, triggerKeyword)
	if idx == -1 {
		return ""
	}

	// Get text after trigger keyword
	remaining := strings.TrimSpace(body[idx+len(triggerKeyword):])

	// If no content after trigger keyword, use issue content
	if remaining == "" {
		return UseIssueContentMarker
	}

	// Get first line after trigger
	lines := strings.Split(remaining, "\n")
	if len(lines) == 0 {
		return UseIssueContentMarker
	}

	prompt := strings.TrimSpace(lines[0])
	if prompt == "" {
		return UseIssueContentMarker
	}

	return prompt
}
