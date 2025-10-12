package webhook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cexll/swe/internal/taskstore"
)

// Task represents a pilot task to be executed
type Task struct {
	ID            string
	Repo          string
	Number        int
	Branch        string
	Prompt        string
	PromptSummary string
	IssueTitle    string
	IssueBody     string
	IsPR          bool
	Username      string // User who triggered the task
	Attempt       int    // Current attempt number (managed by dispatcher)
}

// TaskDispatcher enqueues tasks for asynchronous execution
type TaskDispatcher interface {
	Enqueue(task *Task) error
}

// Handler handles GitHub webhook events
type Handler struct {
	webhookSecret  string
	triggerKeyword string
	dispatcher     TaskDispatcher
	issueDeduper   *commentDeduper
	reviewDeduper  *commentDeduper
	store          *taskstore.Store
}

// NewHandler creates a new webhook handler
func NewHandler(webhookSecret, triggerKeyword string, dispatcher TaskDispatcher, store *taskstore.Store) *Handler {
	return &Handler{
		webhookSecret:  webhookSecret,
		triggerKeyword: triggerKeyword,
		dispatcher:     dispatcher,
		issueDeduper:   newCommentDeduper(12 * time.Hour),
		reviewDeduper:  newCommentDeduper(12 * time.Hour),
		store:          store,
	}
}

// Handle handles GitHub webhook events (issue comments, review comments, etc.)
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
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

	// 3. Determine event type
	eventType := r.Header.Get("X-GitHub-Event")
	switch eventType {
	case "issue_comment":
		h.handleIssueComment(w, payload)
	case "pull_request_review_comment":
		h.handleReviewComment(w, payload)
	default:
		log.Printf("Ignoring unsupported event type: %s", eventType)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Event ignored"))
	}
}

func (h *Handler) handleIssueComment(w http.ResponseWriter, payload []byte) {
	// Parse event
	var event IssueCommentEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("Error parsing event: %v", err)
		http.Error(w, "Error parsing event", http.StatusBadRequest)
		return
	}

	// Only handle newly created comments
	if event.Action != "created" {
		log.Printf("Ignoring issue_comment action: %s", event.Action)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Issue comment action ignored"))
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

	// 5.5 Prevent duplicate processing for the same comment ID
	if !h.issueDeduper.markIfNew(event.Comment.ID) {
		log.Printf("Ignoring duplicate issue comment: id=%d", event.Comment.ID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Duplicate comment ignored"))
		return
	}

	// 6. Extract prompt from comment
	customInstruction, found := extractPrompt(event.Comment.Body, h.triggerKeyword)
	if !found {
		log.Printf("No prompt found after trigger keyword")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("No prompt found"))
		return
	}

	// 7. Check if this is a PR or issue
	isPR := event.Issue.PullRequest != nil

	prompt := buildPrompt(event.Issue.Title, event.Issue.Body, customInstruction)
	promptSummary := buildPromptSummary(event.Issue.Title, customInstruction, isPR)

	// 8. Create task
	task := &Task{
		ID:            h.generateTaskID(event.Repository.FullName, event.Issue.Number),
		Repo:          event.Repository.FullName,
		Number:        event.Issue.Number,
		Branch:        event.Repository.DefaultBranch,
		Prompt:        prompt,
		PromptSummary: promptSummary,
		IssueTitle:    event.Issue.Title,
		IssueBody:     event.Issue.Body,
		IsPR:          isPR,
		Username:      event.Comment.User.Login,
	}

	h.createStoreTask(task)

	log.Printf("Received task: repo=%s, number=%d, commentID=%d, user=%s", task.Repo, task.Number, event.Comment.ID, task.Username)

	h.enqueueTask(w, task, prompt)
}

func (h *Handler) handleReviewComment(w http.ResponseWriter, payload []byte) {
	var event PullRequestReviewCommentEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("Error parsing review comment event: %v", err)
		http.Error(w, "Error parsing event", http.StatusBadRequest)
		return
	}

	// Only handle newly created review comments
	if event.Action != "created" {
		log.Printf("Ignoring pull_request_review_comment action: %s", event.Action)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Review comment action ignored"))
		return
	}

	// Ignore bot comments
	if event.Comment.User.Type == "Bot" {
		log.Printf("Ignoring review comment from bot: %s", event.Comment.User.Login)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Bot comment ignored"))
		return
	}

	// Check trigger keyword
	if !strings.Contains(event.Comment.Body, h.triggerKeyword) {
		log.Printf("Review comment does not contain trigger keyword '%s'", h.triggerKeyword)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("No trigger keyword found"))
		return
	}

	if !h.reviewDeduper.markIfNew(event.Comment.ID) {
		log.Printf("Ignoring duplicate review comment: id=%d", event.Comment.ID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Duplicate comment ignored"))
		return
	}

	customInstruction, found := extractPrompt(event.Comment.Body, h.triggerKeyword)
	if !found {
		log.Printf("No prompt found after trigger keyword in review comment")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("No prompt found"))
		return
	}

	prompt := buildPrompt(event.PullRequest.Title, event.PullRequest.Body, customInstruction)
	promptSummary := buildPromptSummary(event.PullRequest.Title, customInstruction, true)

	branch := event.PullRequest.Base.Ref
	if branch == "" {
		branch = event.Repository.DefaultBranch
	}

	task := &Task{
		ID:            h.generateTaskID(event.Repository.FullName, event.PullRequest.Number),
		Repo:          event.Repository.FullName,
		Number:        event.PullRequest.Number,
		Branch:        branch,
		Prompt:        prompt,
		PromptSummary: promptSummary,
		IssueTitle:    event.PullRequest.Title,
		IssueBody:     event.PullRequest.Body,
		IsPR:          true,
		Username:      event.Comment.User.Login,
	}

	h.createStoreTask(task)

	log.Printf("Received review task: repo=%s, number=%d, commentID=%d, user=%s", task.Repo, task.Number, event.Comment.ID, task.Username)

	h.enqueueTask(w, task, prompt)
}

func (h *Handler) generateTaskID(repo string, number int) string {
	timestamp := time.Now().UnixNano()
	sanitized := strings.ReplaceAll(repo, "/", "-")
	return fmt.Sprintf("%s-%d-%d", sanitized, number, timestamp)
}

func (h *Handler) createStoreTask(task *Task) {
	if h.store == nil {
		return
	}

	owner, name := splitRepo(task.Repo)
	storeTask := &taskstore.Task{
		ID:          task.ID,
		Title:       task.IssueTitle,
		Status:      taskstore.StatusPending,
		RepoOwner:   owner,
		RepoName:    name,
		IssueNumber: task.Number,
		Actor:       task.Username,
	}
	h.store.Create(storeTask)
	h.store.AddLog(task.ID, "info", "Task queued")
}

func splitRepo(full string) (string, string) {
	parts := strings.SplitN(full, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return full, ""
}

func (h *Handler) enqueueTask(w http.ResponseWriter, task *Task, prompt string) {
	if err := h.dispatcher.Enqueue(task); err != nil {
		log.Printf("Failed to enqueue task: %v", err)
		switch {
		case errors.Is(err, ErrQueueFull):
			http.Error(w, "Task queue is busy, try again later", http.StatusServiceUnavailable)
		case errors.Is(err, ErrQueueClosed):
			http.Error(w, "Task queue unavailable", http.StatusServiceUnavailable)
		default:
			http.Error(w, "Failed to enqueue task", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Task queued"))
}

// extractPrompt extracts the prompt text after the trigger keyword.
// Returns the trimmed user instruction and a boolean indicating whether the trigger was found.
func extractPrompt(body, triggerKeyword string) (string, bool) {
	// Find the trigger keyword
	idx := strings.Index(body, triggerKeyword)
	if idx == -1 {
		return "", false
	}

	// Get text after trigger keyword
	remaining := strings.TrimSpace(body[idx+len(triggerKeyword):])

	return remaining, true
}

// buildPrompt builds the final prompt by always including the issue or PR content,
// optionally appending the user instruction if provided.
func buildPrompt(title, body, userInstruction string) string {
	var builder strings.Builder

	builder.WriteString("# Issue: ")
	builder.WriteString(strings.TrimSpace(title))
	builder.WriteString("\n\n")
	builder.WriteString(strings.TrimSpace(body))

	if trimmedInstruction := strings.TrimSpace(userInstruction); trimmedInstruction != "" {
		builder.WriteString("\n\n---\n\n")
		builder.WriteString(trimmedInstruction)
	}

	return builder.String()
}

func buildPromptSummary(title, userInstruction string, isPR bool) string {
	title = strings.TrimSpace(title)
	instruction := summarizeInstruction(userInstruction, 180)

	var builder strings.Builder
	if title != "" {
		if isPR {
			builder.WriteString("**PR:** ")
		} else {
			builder.WriteString("**Issue:** ")
		}
		builder.WriteString(title)
	}

	if instruction != "" {
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString("**Instruction:**\n")
		builder.WriteString(instruction)
	}

	return builder.String()
}

func truncateText(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 || text == "" {
		return ""
	}

	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}

	truncated := strings.TrimSpace(string(runes[:limit]))
	return truncated + "…"
}

func summarizeInstruction(instruction string, limit int) string {
	instruction = strings.TrimSpace(instruction)
	if instruction == "" {
		return ""
	}

	lines := strings.Split(instruction, "\n")
	var parts []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts = append(parts, line)
	}

	if len(parts) == 0 {
		return ""
	}

	joined := strings.Join(parts, " ")
	return truncateText(joined, limit)
}
