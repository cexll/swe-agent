package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockExecutor is a mock implementation of Executor
type mockExecutor struct {
	executeFunc  func(ctx context.Context, task *Task) error
	executeCalls int
}

func (m *mockExecutor) Execute(ctx context.Context, task *Task) error {
	m.executeCalls++
	if m.executeFunc != nil {
		return m.executeFunc(ctx, task)
	}
	return nil
}

func TestExtractPrompt(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		triggerKeyword string
		want           string
	}{
		{
			name:           "simple prompt",
			body:           "/pilot fix the typo",
			triggerKeyword: "/pilot",
			want:           "fix the typo",
		},
		{
			name:           "multiline comment",
			body:           "/pilot add error handling\nSome more context here",
			triggerKeyword: "/pilot",
			want:           "add error handling",
		},
		{
			name:           "no prompt after keyword",
			body:           "/pilot",
			triggerKeyword: "/pilot",
			want:           "",
		},
		{
			name:           "keyword not found",
			body:           "just a comment",
			triggerKeyword: "/pilot",
			want:           "",
		},
		{
			name:           "custom trigger keyword",
			body:           "/custom do something",
			triggerKeyword: "/custom",
			want:           "do something",
		},
		{
			name:           "whitespace handling",
			body:           "/pilot    fix bug   ",
			triggerKeyword: "/pilot",
			want:           "fix bug",
		},
		{
			name:           "keyword in middle of text",
			body:           "Hey @someone\n/pilot refactor code\nThanks!",
			triggerKeyword: "/pilot",
			want:           "refactor code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPrompt(tt.body, tt.triggerKeyword)
			if got != tt.want {
				t.Errorf("extractPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHandleIssueComment(t *testing.T) {
	secret := "test-webhook-secret"
	triggerKeyword := "/pilot"

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
		name           string
		event          *IssueCommentEvent
		signature      string
		expectedStatus int
		expectedBody   string
		shouldExecute  bool
	}{
		{
			name:           "valid trigger on issue",
			event:          createEvent("/pilot fix the bug", false),
			signature:      "", // will be computed
			expectedStatus: http.StatusAccepted,
			expectedBody:   "Task accepted",
			shouldExecute:  true,
		},
		{
			name:           "valid trigger on PR",
			event:          createEvent("/pilot refactor code", true),
			signature:      "",
			expectedStatus: http.StatusAccepted,
			expectedBody:   "Task accepted",
			shouldExecute:  true,
		},
		{
			name:           "no trigger keyword",
			event:          createEvent("just a regular comment", false),
			signature:      "",
			expectedStatus: http.StatusOK,
			expectedBody:   "No trigger keyword found",
			shouldExecute:  false,
		},
		{
			name:           "trigger keyword without prompt",
			event:          createEvent("/pilot", false),
			signature:      "",
			expectedStatus: http.StatusOK,
			expectedBody:   "No prompt found",
			shouldExecute:  false,
		},
		{
			name:           "invalid signature",
			event:          createEvent("/pilot fix bug", false),
			signature:      "sha256=invalidsignature",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Invalid signature",
			shouldExecute:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock executor
			executor := &mockExecutor{
				executeFunc: func(ctx context.Context, task *Task) error {
					// Verify task contents
					if task.Repo != "owner/repo" {
						t.Errorf("Task.Repo = %s, want owner/repo", task.Repo)
					}
					if task.Number != 123 {
						t.Errorf("Task.Number = %d, want 123", task.Number)
					}
					return nil
				},
			}

			handler := NewHandler(secret, triggerKeyword, executor)

			// Marshal event to JSON
			payload, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("Failed to marshal event: %v", err)
			}

			// Create request
			req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))

			// Set signature
			signature := tt.signature
			if signature == "" {
				signature = signPayload(payload)
			}
			req.Header.Set("X-Hub-Signature-256", signature)
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Handle request
			handler.HandleIssueComment(w, req)

			// Check response status
			if w.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d", w.Code, tt.expectedStatus)
			}

			// Check response body
			body := w.Body.String()
			if !strings.Contains(body, tt.expectedBody) {
				t.Errorf("Body = %q, want to contain %q", body, tt.expectedBody)
			}

			// For async execution, wait a bit for goroutine to execute
			if tt.shouldExecute {
				time.Sleep(100 * time.Millisecond)
			}

			// Check if executor was called
			if tt.shouldExecute && executor.executeCalls == 0 {
				t.Error("Executor.Execute() not called when it should have been")
			}
			if !tt.shouldExecute && executor.executeCalls > 0 {
				t.Error("Executor.Execute() called when it should not have been")
			}
		})
	}
}

func TestHandleIssueComment_MalformedPayload(t *testing.T) {
	secret := "test-webhook-secret"
	executor := &mockExecutor{}
	handler := NewHandler(secret, "/pilot", executor)

	tests := []struct {
		name           string
		payload        []byte
		signature      string
		expectedStatus int
	}{
		{
			name:           "invalid JSON",
			payload:        []byte("not json"),
			signature:      "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty payload",
			payload:        []byte(""),
			signature:      "",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate signature for payload
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(tt.payload)
			signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

			req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(tt.payload))
			req.Header.Set("X-Hub-Signature-256", signature)

			w := httptest.NewRecorder()
			handler.HandleIssueComment(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d", w.Code, tt.expectedStatus)
			}
		})
	}
}

func TestHandleIssueComment_SignatureValidation(t *testing.T) {
	secret := "test-webhook-secret"
	executor := &mockExecutor{}
	handler := NewHandler(secret, "/pilot", executor)

	event := &IssueCommentEvent{
		Action: "created",
		Issue: Issue{
			Number: 123,
			Title:  "Test",
		},
		Comment: Comment{
			Body: "/pilot test",
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
			if tt.signature != "" {
				req.Header.Set("X-Hub-Signature-256", tt.signature)
			}

			w := httptest.NewRecorder()
			handler.HandleIssueComment(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d", w.Code, tt.expectedStatus)
			}
		})
	}
}

func TestNewHandler(t *testing.T) {
	secret := "test-secret"
	keyword := "/test"
	executor := &mockExecutor{}

	handler := NewHandler(secret, keyword, executor)

	if handler == nil {
		t.Fatal("NewHandler() returned nil")
	}

	if handler.webhookSecret != secret {
		t.Errorf("webhookSecret = %s, want %s", handler.webhookSecret, secret)
	}

	if handler.triggerKeyword != keyword {
		t.Errorf("triggerKeyword = %s, want %s", handler.triggerKeyword, keyword)
	}

	if handler.executor != executor {
		t.Error("executor not set correctly")
	}
}

// TestHandleIssueComment_ErrorReading tests error handling when reading request body
func TestHandleIssueComment_ErrorReading(t *testing.T) {
	executor := &mockExecutor{}
	handler := NewHandler("secret", "/pilot", executor)

	// Create a reader that always fails
	errReader := &errorReader{err: io.ErrUnexpectedEOF}

	req := httptest.NewRequest("POST", "/webhook", errReader)
	req.Header.Set("X-Hub-Signature-256", "sha256=test")

	w := httptest.NewRecorder()
	handler.HandleIssueComment(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// errorReader is a reader that always returns an error
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}

// Additional edge case tests
func TestTask_Validation(t *testing.T) {
	// Test Task structure
	task := &Task{
		Repo:       "owner/repo",
		Number:     123,
		Branch:     "main",
		Prompt:     "fix bug",
		IssueTitle: "Bug report",
		IssueBody:  "Description",
		IsPR:       false,
	}

	if task.Repo == "" {
		t.Error("Repo should not be empty")
	}
	if task.Number <= 0 {
		t.Error("Number should be positive")
	}
	if task.Branch == "" {
		t.Error("Branch should not be empty")
	}
	if task.Prompt == "" {
		t.Error("Prompt should not be empty")
	}
}

func TestExtractPrompt_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		triggerKeyword string
		want           string
	}{
		{
			name:           "unicode characters",
			body:           "/pilot 修复错误",
			triggerKeyword: "/pilot",
			want:           "修复错误",
		},
		{
			name:           "multiple trigger keywords",
			body:           "/pilot first\n/pilot second",
			triggerKeyword: "/pilot",
			want:           "first",
		},
		{
			name:           "trigger at end of message",
			body:           "Some text before\n/pilot fix this",
			triggerKeyword: "/pilot",
			want:           "fix this",
		},
		{
			name:           "empty lines after trigger",
			body:           "/pilot",
			triggerKeyword: "/pilot",
			want:           "",
		},
		{
			name:           "very long prompt",
			body:           "/pilot " + strings.Repeat("a", 1000),
			triggerKeyword: "/pilot",
			want:           strings.Repeat("a", 1000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPrompt(tt.body, tt.triggerKeyword)
			if got != tt.want {
				t.Errorf("extractPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHandleIssueComment_ConcurrentRequests(t *testing.T) {
	secret := "test-secret"
	executor := &mockExecutor{}
	handler := NewHandler(secret, "/pilot", executor)

	event := &IssueCommentEvent{
		Action: "created",
		Issue: Issue{
			Number: 123,
			Title:  "Test",
		},
		Comment: Comment{
			Body: "/pilot test concurrent",
		},
		Repository: Repository{
			FullName:      "owner/repo",
			DefaultBranch: "main",
		},
	}

	payload, _ := json.Marshal(event)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	// Send multiple concurrent requests
	const numRequests = 5
	responses := make([]*httptest.ResponseRecorder, numRequests)

	for i := 0; i < numRequests; i++ {
		req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
		req.Header.Set("X-Hub-Signature-256", signature)

		responses[i] = httptest.NewRecorder()
		handler.HandleIssueComment(responses[i], req)
	}

	// All requests should be accepted
	for i, w := range responses {
		if w.Code != http.StatusAccepted {
			t.Errorf("Request %d: Status = %d, want %d", i, w.Code, http.StatusAccepted)
		}
	}

	// Wait for all goroutines to finish
	time.Sleep(200 * time.Millisecond)

	// All requests should have triggered execution
	if executor.executeCalls != numRequests {
		t.Errorf("Execute called %d times, want %d", executor.executeCalls, numRequests)
	}
}
