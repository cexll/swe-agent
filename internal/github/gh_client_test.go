package github

import (
	"fmt"
	"testing"
)

func TestMockGHClient_CreateComment(t *testing.T) {
	mock := NewMockGHClient()
	mock.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		if repo == "error/repo" {
			return 0, fmt.Errorf("test error")
		}
		return 99999, nil
	}

	// Test successful creation
	id, err := mock.CreateComment("owner/repo", 123, "test body", "token")
	if err != nil {
		t.Errorf("CreateComment() unexpected error: %v", err)
	}
	if id != 99999 {
		t.Errorf("CreateComment() id = %d, want 99999", id)
	}

	// Test error case
	_, err = mock.CreateComment("error/repo", 123, "test", "token")
	if err == nil {
		t.Error("CreateComment() should return error for error/repo")
	}

	// Verify calls were tracked
	if len(mock.CreateCommentCalls) != 2 {
		t.Errorf("Expected 2 calls, got %d", len(mock.CreateCommentCalls))
	}

	if mock.CreateCommentCalls[0].Repo != "owner/repo" {
		t.Errorf("First call repo = %q, want 'owner/repo'", mock.CreateCommentCalls[0].Repo)
	}
}

func TestMockGHClient_UpdateComment(t *testing.T) {
	mock := NewMockGHClient()
	mock.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		if commentID == 999 {
			return fmt.Errorf("comment not found")
		}
		return nil
	}

	// Test successful update
	err := mock.UpdateComment("owner/repo", 123, "updated body", "token")
	if err != nil {
		t.Errorf("UpdateComment() unexpected error: %v", err)
	}

	// Test error case
	err = mock.UpdateComment("owner/repo", 999, "test", "token")
	if err == nil {
		t.Error("UpdateComment() should return error for comment 999")
	}

	// Verify calls
	if len(mock.UpdateCommentCalls) != 2 {
		t.Errorf("Expected 2 calls, got %d", len(mock.UpdateCommentCalls))
	}
}

func TestMockGHClient_GetCommentBody(t *testing.T) {
	mock := NewMockGHClient()
	mock.GetCommentBodyFunc = func(repo string, commentID int, token string) (string, error) {
		if commentID == 123 {
			return "test comment body", nil
		}
		return "", fmt.Errorf("comment not found")
	}

	// Test successful get
	body, err := mock.GetCommentBody("owner/repo", 123, "token")
	if err != nil {
		t.Errorf("GetCommentBody() unexpected error: %v", err)
	}
	if body != "test comment body" {
		t.Errorf("GetCommentBody() body = %q, want 'test comment body'", body)
	}

	// Test error case
	_, err = mock.GetCommentBody("owner/repo", 999, "token")
	if err == nil {
		t.Error("GetCommentBody() should return error for comment 999")
	}

	// Verify calls
	if len(mock.GetCommentCalls) != 2 {
		t.Errorf("Expected 2 calls, got %d", len(mock.GetCommentCalls))
	}
}

func TestMockGHClient_AddLabel(t *testing.T) {
	mock := NewMockGHClient()

	err := mock.AddLabel("owner/repo", 123, "bug", "token")
	if err != nil {
		t.Errorf("AddLabel() unexpected error: %v", err)
	}

	// Verify call tracking
	if len(mock.AddLabelCalls) != 1 {
		t.Errorf("Expected 1 call, got %d", len(mock.AddLabelCalls))
	}

	if mock.AddLabelCalls[0].Label != "bug" {
		t.Errorf("Label = %q, want 'bug'", mock.AddLabelCalls[0].Label)
	}
}

func TestMockGHClient_Clone(t *testing.T) {
	mock := NewMockGHClient()
	mock.CloneFunc = func(repo, branch, destDir string) error {
		if branch == "nonexistent" {
			return fmt.Errorf("branch not found")
		}
		return nil
	}

	// Test successful clone
	err := mock.Clone("owner/repo", "main", "/tmp/test")
	if err != nil {
		t.Errorf("Clone() unexpected error: %v", err)
	}

	// Test error case
	err = mock.Clone("owner/repo", "nonexistent", "/tmp/test")
	if err == nil {
		t.Error("Clone() should return error for nonexistent branch")
	}

	// Verify calls
	if len(mock.CloneCalls) != 2 {
		t.Errorf("Expected 2 calls, got %d", len(mock.CloneCalls))
	}
}

func TestMockGHClient_CreatePR(t *testing.T) {
	mock := NewMockGHClient()
	mock.CreatePRFunc = func(workdir, repo, head, base, title, body string) (string, error) {
		if head == "invalid" {
			return "", fmt.Errorf("invalid head branch")
		}
		return fmt.Sprintf("https://github.com/%s/pull/1", repo), nil
	}

	// Test successful PR creation
	url, err := mock.CreatePR("/tmp/repo", "owner/repo", "feature", "main", "Test PR", "description")
	if err != nil {
		t.Errorf("CreatePR() unexpected error: %v", err)
	}

	expectedURL := "https://github.com/owner/repo/pull/1"
	if url != expectedURL {
		t.Errorf("CreatePR() url = %q, want %q", url, expectedURL)
	}

	// Test error case
	_, err = mock.CreatePR("/tmp/repo", "owner/repo", "invalid", "main", "Test", "desc")
	if err == nil {
		t.Error("CreatePR() should return error for invalid branch")
	}

	// Verify calls
	if len(mock.CreatePRCalls) != 2 {
		t.Errorf("Expected 2 calls, got %d", len(mock.CreatePRCalls))
	}
}

func TestSetGHClient(t *testing.T) {
	// Save original client
	originalClient := defaultGHClient
	defer func() { defaultGHClient = originalClient }()

	// Create and set mock client
	mock := NewMockGHClient()
	SetGHClient(mock)

	// Test that the global functions use the mock
	_ = CreateComment("owner/repo", 123, "test", "token")

	// Verify the mock was called
	if len(mock.CreateCommentCalls) != 1 {
		t.Errorf("Expected mock to be called once, got %d calls", len(mock.CreateCommentCalls))
	}

	if mock.CreateCommentCalls[0].Repo != "owner/repo" {
		t.Errorf("Repo = %q, want 'owner/repo'", mock.CreateCommentCalls[0].Repo)
	}
}

func TestCreateCommentWithMock(t *testing.T) {
	// Save original client
	originalClient := defaultGHClient
	defer func() { defaultGHClient = originalClient }()

	mock := NewMockGHClient()
	mock.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 54321, nil
	}
	SetGHClient(mock)

	// Test CreateCommentWithID
	id, err := CreateCommentWithID("test/repo", 456, "body", "token")
	if err != nil {
		t.Errorf("CreateCommentWithID() error = %v", err)
	}

	if id != 54321 {
		t.Errorf("CreateCommentWithID() id = %d, want 54321", id)
	}

	// Verify call
	if len(mock.CreateCommentCalls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.CreateCommentCalls))
	}

	call := mock.CreateCommentCalls[0]
	if call.Repo != "test/repo" || call.Number != 456 || call.Body != "body" {
		t.Errorf("Call params mismatch: repo=%s number=%d body=%s", call.Repo, call.Number, call.Body)
	}
}

func TestUpdateCommentWithMock(t *testing.T) {
	originalClient := defaultGHClient
	defer func() { defaultGHClient = originalClient }()

	mock := NewMockGHClient()
	SetGHClient(mock)

	err := UpdateComment("test/repo", 123, "updated", "token")
	if err != nil {
		t.Errorf("UpdateComment() error = %v", err)
	}

	if len(mock.UpdateCommentCalls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.UpdateCommentCalls))
	}

	call := mock.UpdateCommentCalls[0]
	if call.CommentID != 123 {
		t.Errorf("CommentID = %d, want 123", call.CommentID)
	}
}

func TestGetCommentBodyWithMock(t *testing.T) {
	originalClient := defaultGHClient
	defer func() { defaultGHClient = originalClient }()

	mock := NewMockGHClient()
	mock.GetCommentBodyFunc = func(repo string, commentID int, token string) (string, error) {
		return "mocked body content", nil
	}
	SetGHClient(mock)

	body, err := GetCommentBody("test/repo", 789, "token")
	if err != nil {
		t.Errorf("GetCommentBody() error = %v", err)
	}

	if body != "mocked body content" {
		t.Errorf("GetCommentBody() body = %q, want 'mocked body content'", body)
	}

	if len(mock.GetCommentCalls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.GetCommentCalls))
	}
}

func TestAddLabelWithMock(t *testing.T) {
	originalClient := defaultGHClient
	defer func() { defaultGHClient = originalClient }()

	mock := NewMockGHClient()
	SetGHClient(mock)

	err := AddLabel("test/repo", 100, "enhancement", "token")
	if err != nil {
		t.Errorf("AddLabel() error = %v", err)
	}

	if len(mock.AddLabelCalls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mock.AddLabelCalls))
	}

	call := mock.AddLabelCalls[0]
	if call.Label != "enhancement" {
		t.Errorf("Label = %q, want 'enhancement'", call.Label)
	}
}

// Test default behaviors (no custom func set)

func TestMockGHClient_CreateComment_DefaultBehavior(t *testing.T) {
	mock := NewMockGHClient()
	// Don't set CreateCommentFunc - test default behavior

	id, err := mock.CreateComment("owner/repo", 123, "test body", "token")
	if err != nil {
		t.Errorf("CreateComment() default behavior should not error: %v", err)
	}

	if id != 12345 {
		t.Errorf("CreateComment() default id = %d, want 12345", id)
	}

	if len(mock.CreateCommentCalls) != 1 {
		t.Errorf("Expected 1 call tracked, got %d", len(mock.CreateCommentCalls))
	}
}

func TestMockGHClient_UpdateComment_DefaultBehavior(t *testing.T) {
	mock := NewMockGHClient()
	// Don't set UpdateCommentFunc - test default behavior

	err := mock.UpdateComment("owner/repo", 123, "updated body", "token")
	if err != nil {
		t.Errorf("UpdateComment() default behavior should not error: %v", err)
	}

	if len(mock.UpdateCommentCalls) != 1 {
		t.Errorf("Expected 1 call tracked, got %d", len(mock.UpdateCommentCalls))
	}
}

func TestMockGHClient_GetCommentBody_DefaultBehavior(t *testing.T) {
	mock := NewMockGHClient()
	// Don't set GetCommentBodyFunc - test default behavior

	body, err := mock.GetCommentBody("owner/repo", 123, "token")
	if err != nil {
		t.Errorf("GetCommentBody() default behavior should not error: %v", err)
	}

	if body != "mock comment body" {
		t.Errorf("GetCommentBody() default body = %q, want 'mock comment body'", body)
	}

	if len(mock.GetCommentCalls) != 1 {
		t.Errorf("Expected 1 call tracked, got %d", len(mock.GetCommentCalls))
	}
}

func TestMockGHClient_AddLabel_CustomFunction(t *testing.T) {
	mock := NewMockGHClient()
	mock.AddLabelFunc = func(repo string, number int, label, token string) error {
		if label == "forbidden" {
			return fmt.Errorf("forbidden label")
		}
		return nil
	}

	// Test custom function success
	err := mock.AddLabel("owner/repo", 123, "bug", "token")
	if err != nil {
		t.Errorf("AddLabel() unexpected error: %v", err)
	}

	// Test custom function error
	err = mock.AddLabel("owner/repo", 123, "forbidden", "token")
	if err == nil {
		t.Error("AddLabel() should return error for forbidden label")
	}

	if len(mock.AddLabelCalls) != 2 {
		t.Errorf("Expected 2 calls tracked, got %d", len(mock.AddLabelCalls))
	}
}

func TestMockGHClient_Clone_DefaultBehavior(t *testing.T) {
	mock := NewMockGHClient()
	// Don't set CloneFunc - test default behavior

	err := mock.Clone("owner/repo", "main", "/tmp/test")
	if err != nil {
		t.Errorf("Clone() default behavior should not error: %v", err)
	}

	if len(mock.CloneCalls) != 1 {
		t.Errorf("Expected 1 call tracked, got %d", len(mock.CloneCalls))
	}

	call := mock.CloneCalls[0]
	if call.Branch != "main" {
		t.Errorf("Clone() branch = %q, want 'main'", call.Branch)
	}
}

func TestMockGHClient_CreatePR_DefaultBehavior(t *testing.T) {
	mock := NewMockGHClient()
	// Don't set CreatePRFunc - test default behavior

	url, err := mock.CreatePR("/tmp/repo", "owner/repo", "feature", "main", "Test PR", "description")
	if err != nil {
		t.Errorf("CreatePR() default behavior should not error: %v", err)
	}

	expectedURL := "https://github.com/owner/repo/pull/1"
	if url != expectedURL {
		t.Errorf("CreatePR() default url = %q, want %q", url, expectedURL)
	}

	if len(mock.CreatePRCalls) != 1 {
		t.Errorf("Expected 1 call tracked, got %d", len(mock.CreatePRCalls))
	}

	call := mock.CreatePRCalls[0]
	if call.Head != "feature" {
		t.Errorf("CreatePR() head = %q, want 'feature'", call.Head)
	}
}
