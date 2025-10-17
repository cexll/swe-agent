package webhook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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

	// 4. Only handle comment events (issue_comment, pull_request_review_comment)
	if !isCommentEvent(eventType) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Event ignored"))
		return
	}

	// 5. Parse webhook event into GitHub context
	ghCtx, err := github.ParseWebhookEvent(eventType, payload)
	if err != nil {
		log.Printf("Failed to parse webhook event: %v", err)
		http.Error(w, "Error parsing event", http.StatusBadRequest)
		return
	}

	// 6. Check if this is a created action
	if ghCtx.EventAction != "created" {
		w.WriteHeader(http.StatusOK)
		switch eventType {
		case "issue_comment":
			_, _ = w.Write([]byte("Issue comment action ignored"))
		case "pull_request_review_comment":
			_, _ = w.Write([]byte("Review comment action ignored"))
		default:
			_, _ = w.Write([]byte("Non-created action ignored"))
		}
		return
	}

	// 7. Check if comment is from a bot
	if ghCtx.TriggerComment != nil && isBotComment(payload) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Bot comment ignored"))
		return
	}

	// 8. Check if comment contains trigger keyword
	if !ghCtx.ShouldTrigger(h.triggerKeyword) {
		log.Printf("Comment does not contain trigger keyword '%s'", h.triggerKeyword)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("No trigger keyword found"))
		return
	}

	// 9. Verify permission: check if user is the app installer
	if !h.verifyPermission(ghCtx.Repository.FullName, ghCtx.TriggerUser) {
		log.Printf("Permission denied: user %s is not the app installer", ghCtx.TriggerUser)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Permission denied"))
		return
	}

	// 10. Prevent duplicate processing
	commentID := ghCtx.TriggerComment.ID
	deduper := h.getDeduper(eventType)
	if !deduper.markIfNew(commentID) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Duplicate comment ignored"))
		return
	}

	// 10.5. Obtain GitHub App installation token for CommandMode (if available)
	if h.appAuth != nil {
		repo := ghCtx.Repository.FullName
		if repo == "" {
			repo = fmt.Sprintf("%s/%s", ghCtx.Repository.Owner, ghCtx.Repository.Name)
		}
		token, err := h.appAuth.GetInstallationToken(repo)
		if err != nil {
			log.Printf("Warning: Failed to get installation token: %v (continuing without token)", err)
			// Continue without token (fail-open for robustness)
		} else if token != nil {
			// Inject token into context for CommandMode to use
			ghCtx.Token = token.Token
		}
	}

	// 11. Prepare execution context via CommandMode
	mode := modes.GetCommandMode()
	if mode == nil {
		log.Printf("CommandMode not registered")
		http.Error(w, "Internal configuration error", http.StatusInternalServerError)
		return
	}

	prepareResult, err := mode.Prepare(r.Context(), ghCtx)
	if err != nil {
		log.Printf("Failed to prepare task: %v", err)
		http.Error(w, "Task preparation failed", http.StatusInternalServerError)
		return
	}

	// 12. Create and enqueue task
	prBranch := ""
	prState := ""
	if ghCtx.IsPRContext() {
		prBranch = ghCtx.GetHeadBranch()
		prState = ghCtx.GetPRState()
	}

	// Build a concise prompt summary for UI/tests
	var summaryBuilder strings.Builder
	if ghCtx.IsPRContext() {
		summaryBuilder.WriteString("**PR:** ")
	} else {
		summaryBuilder.WriteString("**Issue:** ")
	}
	summaryBuilder.WriteString(ghCtx.IssueTitle)
	if instr := strings.TrimSpace(ghCtx.ExtractPrompt(h.triggerKeyword)); instr != "" {
		summaryBuilder.WriteString("\n\n**Instruction:**\n")
		summaryBuilder.WriteString(instr)
	}

	t := &Task{
		ID:            h.generateTaskID(ghCtx.Repository.FullName, ghCtx.IssueNumber),
		Repo:          ghCtx.Repository.FullName,
		Number:        ghCtx.IssueNumber,
		Branch:        prepareResult.Branch,
		BaseBranch:    prepareResult.BaseBranch,
		Prompt:        prepareResult.Prompt,
		PromptSummary: summaryBuilder.String(),
		IsPR:          ghCtx.IsPR,
		Username:      ghCtx.TriggerUser,
		CommentID:     prepareResult.CommentID,
		PRBranch:      prBranch,
		PRState:       prState,
		Mode:          mode.Name(),
		RawPayload:    payload,
		EventType:     string(ghCtx.EventName),
	}

	h.createStoreTask(t)

	log.Printf("Received task: repo=%s, number=%d, commentID=%d, user=%s", t.Repo, t.Number, commentID, t.Username)

	h.enqueueTask(w, t)
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

// isCommentEvent checks if the event type is a comment event
func isCommentEvent(eventType string) bool {
	return eventType == "issue_comment" || eventType == "pull_request_review_comment"
}

// isBotComment checks if the comment is from a bot
func isBotComment(payload []byte) bool {
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return false
	}

	// Check comment.user.type
	if comment, ok := data["comment"].(map[string]interface{}); ok {
		if user, ok := comment["user"].(map[string]interface{}); ok {
			if userType, ok := user["type"].(string); ok {
				return userType == "Bot"
			}
		}
	}
	return false
}

// getDeduper returns the appropriate deduper based on event type
func (h *Handler) getDeduper(eventType string) *commentDeduper {
	if eventType == "pull_request_review_comment" {
		return h.reviewDeduper
	}
	return h.issueDeduper
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

	// For review comments, branch is the base branch used by the PR
	if task.EventType == "pull_request_review_comment" {
		task.Branch = task.BaseBranch
	}

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte("Task queued"))
}
