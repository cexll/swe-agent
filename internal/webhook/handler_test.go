package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockDispatcher struct {
	enqueueFunc  func(task *Task) error
	enqueueCalls int
	lastTask     *Task
}

func (m *mockDispatcher) Enqueue(task *Task) error {
	m.enqueueCalls++
	m.lastTask = task
	if m.enqueueFunc != nil {
		return m.enqueueFunc(task)
	}
	return nil
}

func TestExtractPrompt(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		triggerKeyword string
		found          bool
		want           string
	}{
		{
			name:           "simple prompt",
			body:           "/code fix the typo",
			triggerKeyword: "/code",
			found:          true,
			want:           "fix the typo",
		},
		{
			name:           "multiline comment",
			body:           "/code add error handling\nSome more context here",
			triggerKeyword: "/code",
			found:          true,
			want:           "add error handling\nSome more context here",
		},
		{
			name:           "no prompt after keyword - should use issue content",
			body:           "/code",
			triggerKeyword: "/code",
			found:          true,
			want:           "",
		},
		{
			name:           "/code with only whitespace",
			body:           "/code   \n\n  ",
			triggerKeyword: "/code",
			found:          true,
			want:           "",
		},
		{
			name:           "keyword not found",
			body:           "just a comment",
			triggerKeyword: "/code",
			found:          false,
			want:           "",
		},
		{
			name:           "custom trigger keyword",
			body:           "/custom do something",
			triggerKeyword: "/custom",
			found:          true,
			want:           "do something",
		},
		{
			name:           "whitespace handling",
			body:           "/code    fix bug   ",
			triggerKeyword: "/code",
			found:          true,
			want:           "fix bug",
		},
		{
			name:           "keyword in middle of text",
			body:           "Hey @someone\n/code refactor code\nThanks!",
			triggerKeyword: "/code",
			found:          true,
			want:           "refactor code\nThanks!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := extractPrompt(tt.body, tt.triggerKeyword)
			if found != tt.found {
				t.Fatalf("extractPrompt() found = %v, want %v", found, tt.found)
			}
			if got != tt.want {
				t.Errorf("extractPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHandleWebhook_IssueComment(t *testing.T) {
	secret := "test-webhook-secret"
	triggerKeyword := "/code"

	createEvent := func(commentBody string, isPR bool) *IssueCommentEvent {
		issue := Issue{
			Number: 123,
			Title:  "Test Issue",
			Body:   "Issue body",
			State:  "open",
		}
		if isPR {
			issue.PullRequest = &struct {
				URL string `json:"url"`
			}{URL: "https://api.github.com/repos/owner/repo/pulls/123"}
		}

		return &IssueCommentEvent{
			Action: "created",
			Issue:  issue,
			Comment: Comment{
				ID:   1,
				Body: commentBody,
				User: User{Login: "testuser", Type: "User"},
			},
			Repository: Repository{
				FullName:      "owner/repo",
				DefaultBranch: "main",
				Owner:         User{Login: "owner"},
				Name:          "repo",
			},
			Sender: User{Login: "testuser"},
		}
	}

	signPayload := func(payload []byte) string {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		return "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}

	tests := []struct {
		name            string
		event           *IssueCommentEvent
		action          string
		signature       string
		expectedStatus  int
		expectedBody    string
		shouldEnqueue   bool
		expectedSummary string
	}{
		{
			name:            "valid trigger on issue",
			event:           createEvent("/code fix the bug", false),
			expectedStatus:  http.StatusAccepted,
			expectedBody:    "Task queued",
			shouldEnqueue:   true,
			expectedSummary: "**Issue:** Test Issue\n\n**Instruction:**\nfix the bug",
		},
		{
			name:            "valid trigger on PR",
			event:           createEvent("/code refactor code", true),
			expectedStatus:  http.StatusAccepted,
			expectedBody:    "Task queued",
			shouldEnqueue:   true,
			expectedSummary: "**PR:** Test Issue\n\n**Instruction:**\nrefactor code",
		},
		{
			name:           "no trigger keyword",
			event:          createEvent("just a regular comment", false),
			expectedStatus: http.StatusOK,
			expectedBody:   "No trigger keyword found",
			shouldEnqueue:  false,
		},
		{
			name:            "trigger keyword without prompt - should use issue content",
			event:           createEvent("/code", false),
			expectedStatus:  http.StatusAccepted,
			expectedBody:    "Task queued",
			shouldEnqueue:   true,
			expectedSummary: "**Issue:** Test Issue",
		},
		{
			name:           "edited comment ignored",
			event:          createEvent("/code fix bug", false),
			action:         "edited",
			expectedStatus: http.StatusOK,
			expectedBody:   "Issue comment action ignored",
			shouldEnqueue:  false,
		},
		{
			name:           "deleted comment ignored",
			event:          createEvent("/code fix bug", false),
			action:         "deleted",
			expectedStatus: http.StatusOK,
			expectedBody:   "Issue comment action ignored",
			shouldEnqueue:  false,
		},
		{
			name:           "invalid signature",
			event:          createEvent("/code fix bug", false),
			signature:      "sha256=invalidsignature",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid signature",
			shouldEnqueue:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.action != "" {
				tt.event.Action = tt.action
			}

			dispatcher := &mockDispatcher{
				enqueueFunc: func(task *Task) error {
					if task.Repo != "owner/repo" {
						t.Errorf("Task.Repo = %s, want owner/repo", task.Repo)
					}
					if task.Number != 123 {
						t.Errorf("Task.Number = %d, want 123", task.Number)
					}
					if tt.expectedSummary != "" && task.PromptSummary != tt.expectedSummary {
						t.Errorf("Task.PromptSummary = %q, want %q", task.PromptSummary, tt.expectedSummary)
					}
					return nil
				},
			}

			handler := NewHandler(secret, triggerKeyword, dispatcher, nil)

			payload, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("Failed to marshal event: %v", err)
			}

			req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-GitHub-Event", "issue_comment")

			signature := tt.signature
			if signature == "" {
				signature = signPayload(payload)
			}
			req.Header.Set("X-Hub-Signature-256", signature)

			w := httptest.NewRecorder()
			handler.Handle(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d", w.Code, tt.expectedStatus)
			}

			body := w.Body.String()
			if !strings.Contains(body, tt.expectedBody) {
				t.Errorf("Body = %q, want to contain %q", body, tt.expectedBody)
			}

			if tt.shouldEnqueue && dispatcher.enqueueCalls == 0 {
				t.Error("Dispatcher.Enqueue not called when it should have been")
			}
			if !tt.shouldEnqueue && dispatcher.enqueueCalls > 0 {
				t.Error("Dispatcher.Enqueue called when it should not have been")
			}
		})
	}
}

func TestHandleWebhook_IssueComment_DuplicateIgnored(t *testing.T) {
	secret := "test-webhook-secret"
	triggerKeyword := "/code"

	event := &IssueCommentEvent{
		Action: "created",
		Issue: Issue{
			Number: 99,
			Title:  "Duplicate test",
			Body:   "Body",
		},
		Comment: Comment{
			ID:   555,
			Body: "/code do work",
			User: User{Login: "tester", Type: "User"},
		},
		Repository: Repository{
			FullName:      "owner/repo",
			DefaultBranch: "main",
		},
	}

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	dispatcher := &mockDispatcher{}
	handler := NewHandler(secret, triggerKeyword, dispatcher, nil)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", signature)
	req.Header.Set("X-GitHub-Event", "issue_comment")

	w := httptest.NewRecorder()
	handler.Handle(w, req)

	if dispatcher.enqueueCalls != 1 {
		t.Fatalf("Expected first event to enqueue task, got %d", dispatcher.enqueueCalls)
	}

	req2 := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
	req2.Header.Set("X-Hub-Signature-256", signature)
	req2.Header.Set("X-GitHub-Event", "issue_comment")

	w2 := httptest.NewRecorder()
	handler.Handle(w2, req2)

	if dispatcher.enqueueCalls != 1 {
		t.Fatalf("Duplicate event should not enqueue new task, got %d", dispatcher.enqueueCalls)
	}

	if w2.Code != http.StatusOK {
		t.Fatalf("Duplicate event response status = %d, want %d", w2.Code, http.StatusOK)
	}

	if !strings.Contains(w2.Body.String(), "Duplicate comment ignored") {
		t.Fatalf("Duplicate event response body = %q", w2.Body.String())
	}
}

func TestHandleWebhook_ReviewComment(t *testing.T) {
	secret := "test-webhook-secret"
	triggerKeyword := "/code"

	event := &PullRequestReviewCommentEvent{
		Action: "created",
		Comment: ReviewComment{
			ID:   10,
			Body: "/code run linters",
			User: User{Login: "reviewer", Type: "User"},
		},
		PullRequest: PullRequest{
			Number: 42,
			Title:  "Improve performance",
			Body:   "Details about improvements",
		},
		Repository: Repository{
			FullName:      "owner/repo",
			DefaultBranch: "main",
		},
		Sender: User{Login: "reviewer"},
	}
	event.PullRequest.Base.Ref = "feature"

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	signature := func(payload []byte) string {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		return "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}(payload)

	dispatcher := &mockDispatcher{}
	handler := NewHandler(secret, triggerKeyword, dispatcher, nil)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", signature)
	req.Header.Set("X-GitHub-Event", "pull_request_review_comment")

	w := httptest.NewRecorder()
	handler.Handle(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("Status = %d, want %d", w.Code, http.StatusAccepted)
	}

	if dispatcher.enqueueCalls != 1 {
		t.Fatalf("Dispatcher.Enqueue calls = %d, want 1", dispatcher.enqueueCalls)
	}

	if dispatcher.lastTask == nil {
		t.Fatal("Expected task to be enqueued")
	}

	if !dispatcher.lastTask.IsPR {
		t.Error("Expected task.IsPR to be true for review comment")
	}

	if dispatcher.lastTask.Branch != "feature" {
		t.Errorf("Task.Branch = %s, want feature", dispatcher.lastTask.Branch)
	}

	expectedSummary := "**PR:** Improve performance\n\n**Instruction:**\nrun linters"
	if dispatcher.lastTask.PromptSummary != expectedSummary {
		t.Errorf("PromptSummary = %q, want %q", dispatcher.lastTask.PromptSummary, expectedSummary)
	}
}

func TestHandleWebhook_ReviewComment_DuplicateIgnored(t *testing.T) {
	secret := "test-webhook-secret"
	triggerKeyword := "/code"

	event := &PullRequestReviewCommentEvent{
		Action: "created",
		Comment: ReviewComment{
			ID:   888,
			Body: "/code run tests",
			User: User{Login: "reviewer", Type: "User"},
		},
		PullRequest: PullRequest{
			Number: 5,
			Title:  "Refactor",
			Body:   "PR body",
		},
		Repository: Repository{
			FullName:      "owner/repo",
			DefaultBranch: "main",
		},
	}

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	dispatcher := &mockDispatcher{}
	handler := NewHandler(secret, triggerKeyword, dispatcher, nil)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", signature)
	req.Header.Set("X-GitHub-Event", "pull_request_review_comment")

	w := httptest.NewRecorder()
	handler.Handle(w, req)

	if dispatcher.enqueueCalls != 1 {
		t.Fatalf("Expected first review comment to enqueue task, got %d", dispatcher.enqueueCalls)
	}

	req2 := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
	req2.Header.Set("X-Hub-Signature-256", signature)
	req2.Header.Set("X-GitHub-Event", "pull_request_review_comment")

	w2 := httptest.NewRecorder()
	handler.Handle(w2, req2)

	if dispatcher.enqueueCalls != 1 {
		t.Fatalf("Duplicate review comment should not enqueue, got %d", dispatcher.enqueueCalls)
	}

	if w2.Code != http.StatusOK {
		t.Fatalf("Duplicate review response status = %d, want %d", w2.Code, http.StatusOK)
	}

	if !strings.Contains(w2.Body.String(), "Duplicate comment ignored") {
		t.Fatalf("Duplicate review response body = %q", w2.Body.String())
	}
}

func TestHandleWebhook_ReviewComment_NoTrigger(t *testing.T) {
	secret := "test-webhook-secret"
	triggerKeyword := "/code"

	event := &PullRequestReviewCommentEvent{
		Action: "created",
		Comment: ReviewComment{
			ID:   10,
			Body: "no trigger here",
			User: User{Login: "reviewer", Type: "User"},
		},
		PullRequest: PullRequest{
			Number: 42,
			Title:  "Improve performance",
			Body:   "Details about improvements",
		},
		Repository: Repository{
			FullName:      "owner/repo",
			DefaultBranch: "main",
		},
		Sender: User{Login: "reviewer"},
	}

	payload, _ := json.Marshal(event)
	signature := func(payload []byte) string {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		return "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}(payload)

	dispatcher := &mockDispatcher{}
	handler := NewHandler(secret, triggerKeyword, dispatcher, nil)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", signature)
	req.Header.Set("X-GitHub-Event", "pull_request_review_comment")

	w := httptest.NewRecorder()
	handler.Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	if dispatcher.enqueueCalls != 0 {
		t.Fatalf("Dispatcher.Enqueue calls = %d, want 0", dispatcher.enqueueCalls)
	}
}

func TestHandleWebhook_ReviewComment_IgnoresNonCreated(t *testing.T) {
	secret := "test-webhook-secret"
	triggerKeyword := "/code"

	event := &PullRequestReviewCommentEvent{
		Action: "edited",
		Comment: ReviewComment{
			ID:   20,
			Body: "/code still valid",
			User: User{Login: "reviewer", Type: "User"},
		},
		PullRequest: PullRequest{
			Number: 7,
			Title:  "Add feature",
			Body:   "Feature details",
		},
		Repository: Repository{
			FullName:      "owner/repo",
			DefaultBranch: "main",
		},
		Sender: User{Login: "reviewer"},
	}

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)

	dispatcher := &mockDispatcher{}
	handler := NewHandler(secret, triggerKeyword, dispatcher, nil)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	req.Header.Set("X-GitHub-Event", "pull_request_review_comment")

	w := httptest.NewRecorder()
	handler.Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Review comment action ignored") {
		t.Fatalf("Body = %q, expected message mentioning action ignored", body)
	}

	if dispatcher.enqueueCalls != 0 {
		t.Fatalf("Dispatcher.Enqueue calls = %d, want 0", dispatcher.enqueueCalls)
	}
}

func TestHandleWebhook_DispatcherError(t *testing.T) {
	secret := "test-webhook-secret"
	triggerKeyword := "/code"

	event := &IssueCommentEvent{
		Action: "created",
		Issue: Issue{
			Number: 1,
			Title:  "Test",
			Body:   "Test body",
		},
		Comment: Comment{
			ID:   1,
			Body: "/code do thing",
			User: User{Login: "tester", Type: "User"},
		},
		Repository: Repository{
			FullName:      "owner/repo",
			DefaultBranch: "main",
		},
	}

	payload, _ := json.Marshal(event)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)

	dispatcher := &mockDispatcher{
		enqueueFunc: func(task *Task) error {
			return io.ErrUnexpectedEOF
		},
	}

	handler := NewHandler(secret, triggerKeyword, dispatcher, nil)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	req.Header.Set("X-GitHub-Event", "issue_comment")

	w := httptest.NewRecorder()
	handler.Handle(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("Status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandleWebhook_QueueFull(t *testing.T) {
	secret := "test-webhook-secret"
	triggerKeyword := "/code"

	event := &IssueCommentEvent{
		Action: "created",
		Issue: Issue{
			Number: 1,
			Title:  "Test",
			Body:   "Test body",
		},
		Comment: Comment{
			ID:   1,
			Body: "/code do thing",
			User: User{Login: "tester", Type: "User"},
		},
		Repository: Repository{
			FullName:      "owner/repo",
			DefaultBranch: "main",
		},
	}

	payload, _ := json.Marshal(event)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)

	dispatcher := &mockDispatcher{
		enqueueFunc: func(task *Task) error {
			return ErrQueueFull
		},
	}

	handler := NewHandler(secret, triggerKeyword, dispatcher, nil)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	req.Header.Set("X-GitHub-Event", "issue_comment")

	w := httptest.NewRecorder()
	handler.Handle(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("Status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	if dispatcher.enqueueCalls != 1 {
		t.Fatalf("Dispatcher.Enqueue calls = %d, want 1", dispatcher.enqueueCalls)
	}
}

func TestHandleWebhook_SignatureValidation(t *testing.T) {
	secret := "test-webhook-secret"
	handler := NewHandler(secret, "/code", &mockDispatcher{}, nil)

	event := &IssueCommentEvent{
		Action: "created",
		Issue: Issue{
			Number: 123,
			Title:  "Test",
		},
		Comment: Comment{
			Body: "/code test",
			User: User{Login: "user", Type: "User"},
		},
		Repository: Repository{
			FullName:      "owner/repo",
			DefaultBranch: "main",
		},
	}
	payload, _ := json.Marshal(event)

	tests := []struct {
		name           string
		signature      string
		expectedStatus int
	}{
		{
			name:           "missing signature header",
			signature:      "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "wrong signature format",
			signature:      "invalid",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "sha1 signature",
			signature:      "sha1=somehash",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
			req.Header.Set("X-GitHub-Event", "issue_comment")
			if tt.signature != "" {
				req.Header.Set("X-Hub-Signature-256", tt.signature)
			}

			w := httptest.NewRecorder()
			handler.Handle(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d", w.Code, tt.expectedStatus)
			}
		})
	}
}

func TestNewHandler(t *testing.T) {
	secret := "test-secret"
	keyword := "/test"
	dispatcher := &mockDispatcher{}

	handler := NewHandler(secret, keyword, dispatcher, nil)

	if handler == nil {
		t.Fatal("NewHandler() returned nil")
	}

	if handler.webhookSecret != secret {
		t.Errorf("webhookSecret = %s, want %s", handler.webhookSecret, secret)
	}

	if handler.triggerKeyword != keyword {
		t.Errorf("triggerKeyword = %s, want %s", handler.triggerKeyword, keyword)
	}

	if handler.dispatcher != dispatcher {
		t.Error("dispatcher not set correctly")
	}
}

func TestHandleWebhook_ErrorReading(t *testing.T) {
	dispatcher := &mockDispatcher{}
	handler := NewHandler("secret", "/code", dispatcher, nil)

	errReader := &errorReader{err: io.ErrUnexpectedEOF}

	req := httptest.NewRequest("POST", "/webhook", errReader)
	req.Header.Set("X-Hub-Signature-256", "sha256=dummy")
	req.Header.Set("X-GitHub-Event", "issue_comment")

	w := httptest.NewRecorder()
	handler.Handle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleWebhook_UnsupportedEvent(t *testing.T) {
	dispatcher := &mockDispatcher{}
	handler := NewHandler("secret", "/code", dispatcher, nil)

	event := map[string]string{"ping": "pong"}
	payload, _ := json.Marshal(event)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(payload)
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	req.Header.Set("X-GitHub-Event", "ping")

	w := httptest.NewRecorder()
	handler.Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	if dispatcher.enqueueCalls != 0 {
		t.Fatalf("Dispatcher.Enqueue calls = %d, want 0", dispatcher.enqueueCalls)
	}
}

type errorReader struct {
	err error
}

func (r *errorReader) Read(p []byte) (int, error) {
	return 0, r.err
}
