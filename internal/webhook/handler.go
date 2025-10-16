package webhook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/modes"
	"github.com/cexll/swe/internal/taskstore"
)

// Task represents a pilot task to be executed
type Task struct {
	ID            string
	Repo          string
	Number        int
	Branch        string
	BaseBranch    string
	Prompt        string
	PromptSummary string
	IssueTitle    string
	IssueBody     string
	IsPR          bool
	PRBranch      string // PR's source branch (if it's a PR)
	PRState       string // PR state: "open" or "closed"
	Username      string // User who triggered the task
	Attempt       int    // Current attempt number (managed by dispatcher)
	PromptContext map[string]string
	CommentID     int64  // coordination comment id (when prepared by modes)
	Mode          string // detected mode name
	// Raw webhook preservation for adapter-based execution
	RawPayload []byte
	EventType  string
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
	appAuth        github.AuthProvider
}

// NewHandler creates a new webhook handler
func NewHandler(webhookSecret, triggerKeyword string, dispatcher TaskDispatcher, store *taskstore.Store, appAuth github.AuthProvider) *Handler {
	return &Handler{
		webhookSecret:  webhookSecret,
		triggerKeyword: triggerKeyword,
		dispatcher:     dispatcher,
		issueDeduper:   newCommentDeduper(12 * time.Hour),
		reviewDeduper:  newCommentDeduper(12 * time.Hour),
		store:          store,
		appAuth:        appAuth,
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

	// 3.5 Try new mode-based pipeline first (non-breaking: fallback if no mode)
	if ghCtx, err := github.ParseWebhookEvent(eventType, payload); err == nil {
		if mode := modes.DetectMode(ghCtx); mode != nil {
			// Prepare execution context (comment, branch, prompt)
			prepareResult, err := mode.Prepare(r.Context(), ghCtx)
			if err != nil {
				log.Printf("Failed to prepare via mode %q: %v", mode.Name(), err)
				http.Error(w, "Preparation failed", http.StatusInternalServerError)
				return
			}

			// Create task and enqueue
			t := &Task{
				ID:         h.generateTaskID(ghCtx.Repository.FullName, ghCtx.IssueNumber),
				Repo:       ghCtx.Repository.FullName,
				Number:     ghCtx.IssueNumber,
				Branch:     prepareResult.Branch,
				BaseBranch: prepareResult.BaseBranch,
				Prompt:     prepareResult.Prompt,
				IsPR:       ghCtx.IsPR,
				Username:   ghCtx.TriggerUser,
				CommentID:  prepareResult.CommentID,
				Mode:       mode.Name(),
				// Keep raw webhook for adapter/executor context reconstruction
				RawPayload: payload,
				EventType:  string(ghCtx.EventName),
			}

			if err := h.dispatcher.Enqueue(t); err != nil {
				log.Printf("Failed to enqueue task: %v", err)
				http.Error(w, "Failed to enqueue", http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("Task queued"))
			return
		}
	} else {
		log.Printf("Failed to parse webhook for mode detection: %v", err)
	}
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

	// 5.1 Verify permission: check if user is the app installer
	if !h.verifyPermission(event.Repository.FullName, event.Comment.User.Login) {
		log.Printf("Permission denied: user %s is not the app installer", event.Comment.User.Login)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Permission denied"))
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

	// 8. Create task (preserve raw payload for executor adapter)
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
		PromptContext: buildPromptContextForIssue(event, h.triggerKeyword, isPR),
		RawPayload:    payload,
		EventType:     "issue_comment",
	}

	h.createStoreTask(task)

	// No extra execution mode hints: keep KISS and rely on latest trigger comment

	log.Printf("Received task: repo=%s, number=%d, commentID=%d, user=%s", task.Repo, task.Number, event.Comment.ID, task.Username)

	h.enqueueTask(w, task)
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

	// Verify permission: check if user is the app installer
	if !h.verifyPermission(event.Repository.FullName, event.Comment.User.Login) {
		log.Printf("Permission denied: user %s is not the app installer", event.Comment.User.Login)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Permission denied"))
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
		PRBranch:      event.PullRequest.Head.Ref,
		PRState:       event.PullRequest.State,
		Username:      event.Comment.User.Login,
		PromptContext: buildPromptContextForReview(event, h.triggerKeyword),
		RawPayload:    payload,
		EventType:     "pull_request_review_comment",
	}

	h.createStoreTask(task)

	// No execution mode injection to avoid over-design

	log.Printf("Received review task: repo=%s, number=%d, commentID=%d, user=%s", task.Repo, task.Number, event.Comment.ID, task.Username)

	h.enqueueTask(w, task)
}

func (h *Handler) generateTaskID(repo string, number int) string {
	timestamp := time.Now().UnixNano()
	sanitized := strings.ReplaceAll(repo, "/", "-")
	return fmt.Sprintf("%s-%d-%d", sanitized, number, timestamp)
}

// verifyPermission checks if the user has permission to trigger tasks
// Returns true if user is the GitHub App installer
func (h *Handler) verifyPermission(repo, username string) bool {
	// Allow override via environment for development or lenient deployments
	if strings.EqualFold(strings.TrimSpace(os.Getenv("ALLOW_ALL_USERS")), "true") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("PERMISSION_MODE")), "open") {
		log.Printf("Permission override enabled via env (ALLOW_ALL_USERS/PERMISSION_MODE), allowing user %s", username)
		return true
	}

	if h.appAuth == nil {
		// No auth provider, allow all (for testing)
		log.Printf("Warning: No app auth provider configured, allowing all users")
		return true
	}

	// Get the installation owner
	owner, err := h.appAuth.GetInstallationOwner(repo)
	if err != nil {
		log.Printf("Warning: Failed to get installation owner: %v (allowing request)", err)
		// On error, allow the request (fail-open for robustness)
		return true
	}

	// Check if user matches the installer
	if username != owner {
		log.Printf("Permission check failed: user=%s, installer=%s", username, owner)
		return false
	}

	log.Printf("Permission check passed: user=%s is the installer", username)
	return true
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

	// Ensure newest comment wins: mark older tasks for the same issue as superseded.
	if n := h.store.SupersedeOlder(owner, name, task.Number, task.ID); n > 0 {
		log.Printf("Superseded %d older task(s) for %s#%d", n, task.Repo, task.Number)
		h.store.AddLog(task.ID, "info", fmt.Sprintf("Superseded %d older task(s)", n))
	}
}

func splitRepo(full string) (string, string) {
	parts := strings.SplitN(full, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return full, ""
}

func (h *Handler) enqueueTask(w http.ResponseWriter, task *Task) {
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

// KISS: no execution mode classifier; resolve via prompt design only

// buildPrompt builds the final prompt by treating the trigger instruction as the primary directive
// and including the issue/PR content as contextual reference.
func buildPrompt(title, body, userInstruction string) string {
	instruction := strings.TrimSpace(userInstruction)
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)

	var builder strings.Builder

	if instruction != "" {
		builder.WriteString(instruction)
	}

	if title != "" || body != "" {
		if builder.Len() > 0 {
			builder.WriteString("\n\n---\n\n")
		}
		builder.WriteString("# Issue Context")
		if title != "" {
			builder.WriteString("\n\n## Title\n")
			builder.WriteString(title)
		}
		if body != "" {
			builder.WriteString("\n\n## Body\n")
			builder.WriteString(body)
		}
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

func buildPromptContextForIssue(event IssueCommentEvent, trigger string, isPR bool) map[string]string {
	context := map[string]string{
		"issue_title":          event.Issue.Title,
		"issue_body":           event.Issue.Body,
		"event_name":           "issue_comment",
		"event_type":           "GENERAL_COMMENT",
		"trigger_phrase":       trigger,
		"trigger_username":     event.Comment.User.Login,
		"trigger_display_name": event.Comment.User.Login,
		"trigger_comment":      event.Comment.Body,
		"trigger_context":      fmt.Sprintf("issue comment with '%s'", trigger),
		"repository":           event.Repository.FullName,
		"base_branch":          event.Repository.DefaultBranch,
		"is_pr":                strconv.FormatBool(isPR),
		"issue_number":         strconv.Itoa(event.Issue.Number),
	}

	// Heuristic: analysis/review-only requests should avoid branch creation
	if isAnalysisOnly(event.Comment.Body) {
		context["analysis_only"] = "true"
	}

	if isPR {
		context["pr_number"] = strconv.Itoa(event.Issue.Number)
	}

	return context
}

func buildPromptContextForReview(event PullRequestReviewCommentEvent, trigger string) map[string]string {
	branch := event.PullRequest.Base.Ref
	if branch == "" {
		branch = event.Repository.DefaultBranch
	}

	return map[string]string{
		"issue_title":          event.PullRequest.Title,
		"issue_body":           event.PullRequest.Body,
		"event_name":           "pull_request_review_comment",
		"event_type":           "REVIEW_COMMENT",
		"trigger_phrase":       trigger,
		"trigger_username":     event.Comment.User.Login,
		"trigger_display_name": event.Comment.User.Login,
		"trigger_comment":      event.Comment.Body,
		"trigger_context":      fmt.Sprintf("PR review comment with '%s'", trigger),
		"repository":           event.Repository.FullName,
		"base_branch":          branch,
		"is_pr":                "true",
		"pr_number":            strconv.Itoa(event.PullRequest.Number),
	}
}
