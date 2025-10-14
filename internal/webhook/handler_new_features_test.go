package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandler_PermissionVerification tests that only the app installer can trigger tasks
func TestHandler_PermissionVerification(t *testing.T) {
	secret := "test-secret"
	dispatcher := &mockDispatcher{}

	tests := []struct {
		name            string
		commentUser     string
		installerUser   string
		expectedAllowed bool
		expectedStatus  int
	}{
		{
			name:            "installer can trigger",
			commentUser:     "installer-user",
			installerUser:   "installer-user",
			expectedAllowed: true,
			expectedStatus:  http.StatusAccepted, // 202 when task is enqueued
		},
		{
			name:            "non-installer cannot trigger",
			commentUser:     "random-user",
			installerUser:   "installer-user",
			expectedAllowed: false,
			expectedStatus:  http.StatusOK, // 200 with rejection message
		},
		{
			name:            "different user blocked",
			commentUser:     "attacker",
			installerUser:   "owner",
			expectedAllowed: false,
			expectedStatus:  http.StatusOK, // 200 with rejection message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset dispatcher
			dispatcher.lastTask = nil

			// Setup mock auth with specific installer
			mockAuth := &mockAppAuth{
				GetInstallationOwnerFunc: func(repo string) (string, error) {
					return tt.installerUser, nil
				},
			}

			handler := NewHandler(secret, "/code", dispatcher, nil, mockAuth)

			event := &IssueCommentEvent{
				Action: "created",
				Issue: Issue{
					Number: 123,
					Title:  "Test issue",
				},
				Comment: Comment{
					ID:   456,
					Body: "/code test command",
					User: User{Login: tt.commentUser},
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

			req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
			req.Header.Set("X-Hub-Signature-256", signature)
			req.Header.Set("X-GitHub-Event", "issue_comment")

			w := httptest.NewRecorder()
			handler.Handle(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Status = %d, want %d", w.Code, tt.expectedStatus)
			}

			// Check if task was dispatched based on permission
			taskDispatched := dispatcher.lastTask != nil
			if taskDispatched != tt.expectedAllowed {
				t.Errorf("Task dispatched = %v, want %v", taskDispatched, tt.expectedAllowed)
			}

			// Verify response body for rejected requests
			if !tt.expectedAllowed {
				body := w.Body.String()
				if !strings.Contains(body, "Permission denied") {
					t.Errorf("Expected 'Permission denied' in response, got: %s", body)
				}
			}
		})
	}
}

// TestHandler_PermissionVerification_AuthError tests handling of auth errors
func TestHandler_PermissionVerification_AuthError(t *testing.T) {
	secret := "test-secret"
	dispatcher := &mockDispatcher{}

	// Mock auth that returns error
	mockAuth := &mockAppAuth{
		GetInstallationOwnerFunc: func(repo string) (string, error) {
			return "", fmt.Errorf("mock auth error") // Simulate error
		},
	}

	handler := NewHandler(secret, "/code", dispatcher, nil, mockAuth)

	event := &IssueCommentEvent{
		Action: "created",
		Issue: Issue{
			Number: 123,
			Title:  "Test issue",
		},
		Comment: Comment{
			ID:   456,
			Body: "/code test",
			User: User{Login: "test-user"},
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

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", signature)
	req.Header.Set("X-GitHub-Event", "issue_comment")

	w := httptest.NewRecorder()
	handler.Handle(w, req)

	// Should still succeed (fail-open behavior for robustness)
	if w.Code != http.StatusAccepted { // 202 when task enqueued
		t.Errorf("Status = %d, want %d", w.Code, http.StatusAccepted)
	}

	// Task should be allowed (fail-open on auth error)
	if dispatcher.lastTask == nil {
		t.Error("Expected task to be dispatched on auth error (fail-open)")
	}
}

// TestHandler_PRBranchInfo tests that PR branch information is correctly extracted
func TestHandler_PRBranchInfo(t *testing.T) {
	secret := "test-secret"

	tests := []struct {
		name           string
		prState        string
		prHeadRef      string
		expectedBranch string
		expectedState  string
	}{
		{
			name:           "open PR with feature branch",
			prState:        "open",
			prHeadRef:      "feature/new-feature",
			expectedBranch: "feature/new-feature",
			expectedState:  "open",
		},
		{
			name:           "closed PR",
			prState:        "closed",
			prHeadRef:      "old-feature",
			expectedBranch: "old-feature",
			expectedState:  "closed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create new handler and dispatcher for each test to avoid duplicate checks
			dispatcher := &mockDispatcher{}
			mockAuth := &mockAppAuth{}
			handler := NewHandler(secret, "/code", dispatcher, nil, mockAuth)

			dispatcher.lastTask = nil

			event := &PullRequestReviewCommentEvent{
				Action: "created",
				Comment: ReviewComment{
					ID:   789,
					Body: "/code review this",
					User: User{Login: "testuser"},
				},
				PullRequest: PullRequest{
					Number: 456,
					Title:  "Test PR",
					State:  tt.prState,
					Base: struct {
						Ref string `json:"ref"`
					}{Ref: "main"},
					Head: struct {
						Ref string `json:"ref"`
					}{Ref: tt.prHeadRef},
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

			req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(payload))
			req.Header.Set("X-Hub-Signature-256", signature)
			req.Header.Set("X-GitHub-Event", "pull_request_review_comment")

			w := httptest.NewRecorder()
			handler.Handle(w, req)

			if w.Code != http.StatusAccepted {
				t.Fatalf("Status = %d, want %d", w.Code, http.StatusAccepted)
			}

			if dispatcher.lastTask == nil {
				t.Fatal("Expected task to be dispatched")
			}

			// Verify PR branch information was captured
			if dispatcher.lastTask.PRBranch != tt.expectedBranch {
				t.Errorf("PRBranch = %s, want %s", dispatcher.lastTask.PRBranch, tt.expectedBranch)
			}
			if dispatcher.lastTask.PRState != tt.expectedState {
				t.Errorf("PRState = %s, want %s", dispatcher.lastTask.PRState, tt.expectedState)
			}
			if !dispatcher.lastTask.IsPR {
				t.Error("Expected IsPR to be true")
			}
		})
	}
}
