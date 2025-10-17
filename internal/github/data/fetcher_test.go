package data

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	gh "github.com/cexll/swe/internal/github"
)

func TestSplitRepo_Valid(t *testing.T) {
	owner, repo, err := splitRepo("owner/repo")
	if err != nil {
		t.Fatalf("splitRepo() returned unexpected error: %v", err)
	}
	if owner != "owner" {
		t.Errorf("owner = %q, want 'owner'", owner)
	}
	if repo != "repo" {
		t.Errorf("repo = %q, want 'repo'", repo)
	}
}

func TestSplitRepo_InvalidFormat(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"no slash", "invalid"},
		{"too many parts", "owner/repo/extra"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := splitRepo(tt.input)
			if err == nil {
				t.Errorf("splitRepo(%q) should return error", tt.input)
				return
			}
			if !strings.Contains(err.Error(), "invalid repository format") {
				t.Errorf("error should mention invalid format, got: %v", err)
			}
		})
	}
}

func TestFilterComments_NoTriggerTime(t *testing.T) {
	comments := []Comment{
		{Body: "first", CreatedAt: "2024-01-01T10:00:00Z"},
		{Body: "second", CreatedAt: "2024-01-02T10:00:00Z"},
	}

	result := FilterComments(comments, "")
	if len(result) != 2 {
		t.Errorf("FilterComments with empty trigger should return all comments, got %d", len(result))
	}
}

func TestFilterComments_FiltersByTriggerTime(t *testing.T) {
	comments := []Comment{
		{
			Body:      "before trigger",
			CreatedAt: "2024-01-01T10:00:00Z",
			UpdatedAt: "2024-01-01T11:00:00Z",
		},
		{
			Body:      "at trigger time",
			CreatedAt: "2024-01-02T10:00:00Z",
		},
		{
			Body:      "after trigger",
			CreatedAt: "2024-01-03T10:00:00Z",
		},
	}

	triggerTime := "2024-01-02T10:00:00Z"
	result := FilterComments(comments, triggerTime)

	if len(result) != 1 {
		t.Errorf("FilterComments should return 1 comment before trigger, got %d", len(result))
	}
	if len(result) > 0 && result[0].Body != "before trigger" {
		t.Errorf("Remaining comment should be 'before trigger', got: %q", result[0].Body)
	}
}

func TestFilterComments_ExcludesEditedAfterTrigger(t *testing.T) {
	now := time.Now()
	before := now.Add(-2 * time.Hour).Format(time.RFC3339)
	trigger := now.Add(-1 * time.Hour).Format(time.RFC3339)
	after := now.Format(time.RFC3339)

	comments := []Comment{
		{
			Body:         "created before, edited after trigger",
			CreatedAt:    before,
			LastEditedAt: after,
		},
		{
			Body:      "created and edited before trigger",
			CreatedAt: before,
			UpdatedAt: before,
		},
	}

	result := FilterComments(comments, trigger)

	if len(result) != 1 {
		t.Errorf("Should return 1 comment, got %d", len(result))
	}
	if len(result) > 0 && !strings.Contains(result[0].Body, "created and edited before") {
		t.Errorf("Wrong comment filtered, got: %q", result[0].Body)
	}
}

func TestFilterComments_InvalidTriggerTime(t *testing.T) {
	comments := []Comment{
		{Body: "test", CreatedAt: "2024-01-01T10:00:00Z"},
	}

	result := FilterComments(comments, "invalid-time-format")

	if len(result) != 1 {
		t.Error("FilterComments with invalid trigger time should return all comments")
	}
}

func TestFilterCommentsToTriggerTime_NoFiltering(t *testing.T) {
	items := []string{"first", "second"}
	got := FilterCommentsToTriggerTime(items, func(s string) (string, string, string) {
		return s, "", ""
	})
	if len(got) != len(items) {
		t.Fatalf("FilterCommentsToTriggerTime should preserve length, got %d", len(got))
	}
	for i := range items {
		if got[i] != items[i] {
			t.Fatalf("element %d mismatch: got %q want %q", i, got[i], items[i])
		}
	}
}

func TestFilterByTime_EdgeCases(t *testing.T) {
	var empty []int
	gotEmpty := filterByTime(empty)
	if gotEmpty != nil {
		t.Fatalf("empty input should return nil, got %#v", gotEmpty)
	}

	values := []int{1, 2, 3}
	gotValues := filterByTime(values)
	if len(gotValues) != len(values) {
		t.Fatalf("length mismatch: got %d want %d", len(gotValues), len(values))
	}
	for i := range values {
		if gotValues[i] != values[i] {
			t.Fatalf("value %d mismatch: got %d want %d", i, gotValues[i], values[i])
		}
	}
	if &gotValues[0] != &values[0] {
		t.Fatalf("filterByTime should return original slice")
	}
}

func TestFetchAllRemainingFiles(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	intPtr := func(v int) *int { return &v }

	type step struct {
		expectCursor string
		nodes        []File
		hasNext      bool
		endCursor    string
		errMsg       string
		status       int
	}

	tests := []struct {
		name          string
		initialCursor string
		steps         []step
		dynamic       func(call int, cursor string) (int, any)
		expectErr     bool
		wantPaths     []string
		wantLen       *int
		wantFirst     string
		wantLast      string
		wantCalls     int
	}{
		{
			name:          "single-page",
			initialCursor: "c1",
			steps: []step{
				{
					expectCursor: "c1",
					nodes:        []File{{Path: "p1.go", ChangeType: "MODIFIED"}},
					hasNext:      false,
				},
			},
			wantPaths: []string{"p1.go"},
			wantCalls: 1,
		},
		{
			name:          "multi-page",
			initialCursor: "c1",
			steps: []step{
				{
					expectCursor: "c1",
					nodes:        []File{{Path: "p1.go"}, {Path: "p2.go"}},
					hasNext:      true,
					endCursor:    "c2",
				},
				{
					expectCursor: "c2",
					nodes:        []File{{Path: "p3.go"}},
					hasNext:      true,
					endCursor:    "c3",
				},
				{
					expectCursor: "c3",
					nodes:        []File{{Path: "p4.go"}},
					hasNext:      false,
				},
			},
			wantPaths: []string{"p1.go", "p2.go", "p3.go", "p4.go"},
			wantCalls: 3,
		},
		{
			name:          "empty-results",
			initialCursor: "start",
			steps: []step{
				{
					expectCursor: "start",
					nodes:        nil,
					hasNext:      false,
				},
			},
			wantLen:   intPtr(0),
			wantCalls: 1,
		},
		{
			name:          "graphql-error",
			initialCursor: "oops",
			steps: []step{
				{
					expectCursor: "oops",
					errMsg:       "boom",
				},
			},
			expectErr: true,
			wantCalls: 1,
		},
		{
			name:          "max-iterations-cap",
			initialCursor: "start",
			dynamic: func(call int, cursor string) (int, any) {
				page := call + 1
				return http.StatusOK, map[string]any{
					"data": map[string]any{
						"repository": map[string]any{
							"pullRequest": map[string]any{
								"files": map[string]any{
									"nodes": []File{
										{Path: fmt.Sprintf("file-%d.go", page)},
									},
									"pageInfo": map[string]any{
										"hasNextPage": true,
										"endCursor":   "loop",
									},
								},
							},
						},
					},
				}
			},
			wantLen:   intPtr(maxPaginationIterations),
			wantFirst: "file-1.go",
			wantLast:  fmt.Sprintf("file-%d.go", maxPaginationIterations),
			wantCalls: maxPaginationIterations,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			ts := newGraphQLServer(t, func(query string, vars map[string]any) (int, any) {
				if !strings.Contains(query, "FetchMoreFiles") {
					t.Fatalf("unexpected query %q", query)
				}
				cursor, _ := vars["cursor"].(string)
				if tt.dynamic != nil {
					call := callCount
					callCount++
					status, body := tt.dynamic(call, cursor)
					if status == 0 {
						status = http.StatusOK
					}
					return status, body
				}
				if callCount >= len(tt.steps) {
					t.Fatalf("unexpected extra call %d", callCount)
				}
				step := tt.steps[callCount]
				callCount++
				if step.expectCursor != "" && cursor != step.expectCursor {
					t.Fatalf("cursor mismatch: got %q want %q", cursor, step.expectCursor)
				}
				if step.errMsg != "" {
					return http.StatusOK, map[string]any{
						"errors": []map[string]any{{"message": step.errMsg}},
					}
				}
				status := step.status
				if status == 0 {
					status = http.StatusOK
				}
				return status, map[string]any{
					"data": map[string]any{
						"repository": map[string]any{
							"pullRequest": map[string]any{
								"files": map[string]any{
									"nodes": step.nodes,
									"pageInfo": map[string]any{
										"hasNextPage": step.hasNext,
										"endCursor":   step.endCursor,
									},
								},
							},
						},
					},
				}
			})
			defer ts.Close()

			client := NewClient(fakeAuth2{})
			client.endpoint = ts.URL

			got, err := fetchAllRemainingFiles(ctx, client, "o", "r", 1, tt.initialCursor)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				if tt.wantCalls > 0 && callCount != tt.wantCalls {
					t.Fatalf("want %d calls, got %d", tt.wantCalls, callCount)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantLen != nil && len(got) != *tt.wantLen {
				t.Fatalf("len(got) = %d want %d", len(got), *tt.wantLen)
			}
			if len(tt.wantPaths) > 0 {
				if len(got) != len(tt.wantPaths) {
					t.Fatalf("len mismatch: got %d want %d", len(got), len(tt.wantPaths))
				}
				for i, wantPath := range tt.wantPaths {
					if got[i].Path != wantPath {
						t.Fatalf("file %d path mismatch: got %q want %q", i, got[i].Path, wantPath)
					}
				}
			}
			if tt.wantFirst != "" {
				if len(got) == 0 || got[0].Path != tt.wantFirst {
					t.Fatalf("first path mismatch: got %v want %q", got, tt.wantFirst)
				}
			}
			if tt.wantLast != "" {
				if len(got) == 0 || got[len(got)-1].Path != tt.wantLast {
					t.Fatalf("last path mismatch: got %v want %q", got, tt.wantLast)
				}
			}
			if tt.wantCalls > 0 && callCount != tt.wantCalls {
				t.Fatalf("want %d calls, got %d", tt.wantCalls, callCount)
			}
		})
	}
}

func TestFilterReviews_NoTriggerTime(t *testing.T) {
	reviews := []Review{
		{Body: "first review", SubmittedAt: "2024-01-01T10:00:00Z"},
		{Body: "second review", SubmittedAt: "2024-01-02T10:00:00Z"},
	}

	result := FilterReviews(reviews, "")
	if len(result) != 2 {
		t.Errorf("FilterReviews with empty trigger should return all reviews, got %d", len(result))
	}
}

func TestFilterReviews_FiltersByTriggerTime(t *testing.T) {
	reviews := []Review{
		{
			Body:        "before trigger",
			SubmittedAt: "2024-01-01T10:00:00Z",
		},
		{
			Body:        "at trigger time",
			SubmittedAt: "2024-01-02T10:00:00Z",
		},
		{
			Body:        "after trigger",
			SubmittedAt: "2024-01-03T10:00:00Z",
		},
	}

	triggerTime := "2024-01-02T10:00:00Z"
	result := FilterReviews(reviews, triggerTime)

	if len(result) != 1 {
		t.Errorf("FilterReviews should return 1 review before trigger, got %d", len(result))
	}
	if len(result) > 0 && result[0].Body != "before trigger" {
		t.Errorf("Remaining review should be 'before trigger', got: %q", result[0].Body)
	}
}

func TestFilterReviews_ExcludesEditedAfterTrigger(t *testing.T) {
	now := time.Now()
	before := now.Add(-2 * time.Hour).Format(time.RFC3339)
	trigger := now.Add(-1 * time.Hour).Format(time.RFC3339)
	after := now.Format(time.RFC3339)

	reviews := []Review{
		{
			Body:         "submitted before, edited after",
			SubmittedAt:  before,
			LastEditedAt: after,
		},
		{
			Body:        "submitted and updated before",
			SubmittedAt: before,
			UpdatedAt:   before,
		},
	}

	result := FilterReviews(reviews, trigger)

	if len(result) != 1 {
		t.Errorf("Should return 1 review, got %d", len(result))
	}
	if len(result) > 0 && !strings.Contains(result[0].Body, "submitted and updated before") {
		t.Errorf("Wrong review filtered, got: %q", result[0].Body)
	}
}

func TestFetchAllRemainingComments(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	intPtr := func(v int) *int { return &v }

	type step struct {
		expectCursor string
		nodes        []Comment
		hasNext      bool
		endCursor    string
		errMsg       string
	}

	tests := []struct {
		name          string
		initialCursor string
		isPR          bool
		steps         []step
		dynamic       func(call int, cursor string) (int, any)
		expectErr     bool
		wantBodies    []string
		wantLen       *int
		wantFirst     string
		wantLast      string
		wantCalls     int
		expectedQuery string
	}{
		{
			name:          "pr-single-page",
			initialCursor: "c1",
			isPR:          true,
			expectedQuery: "FetchMorePRComments",
			steps: []step{
				{
					expectCursor: "c1",
					nodes: []Comment{
						{ID: "1", Body: "first", Author: Author{Login: "u1"}, CreatedAt: "t1"},
					},
					hasNext: false,
				},
			},
			wantBodies: []string{"first"},
			wantCalls:  1,
		},
		{
			name:          "pr-multi-page",
			initialCursor: "c1",
			isPR:          true,
			expectedQuery: "FetchMorePRComments",
			steps: []step{
				{
					expectCursor: "c1",
					nodes: []Comment{
						{ID: "1", Body: "first", Author: Author{Login: "u1"}, CreatedAt: "t1"},
					},
					hasNext:   true,
					endCursor: "c2",
				},
				{
					expectCursor: "c2",
					nodes: []Comment{
						{ID: "2", Body: "second", Author: Author{Login: "u2"}, CreatedAt: "t2"},
					},
					hasNext:   true,
					endCursor: "c3",
				},
				{
					expectCursor: "c3",
					nodes: []Comment{
						{ID: "3", Body: "third", Author: Author{Login: "u3"}, CreatedAt: "t3"},
					},
					hasNext: false,
				},
			},
			wantBodies: []string{"first", "second", "third"},
			wantCalls:  3,
		},
        {
			name:          "issue-empty",
			initialCursor: "cursor",
			isPR:          false,
			expectedQuery: "FetchMoreIssueComments",
			steps: []step{
				{
					expectCursor: "cursor",
					nodes:        nil,
					hasNext:      false,
				},
			},
			wantLen:   intPtr(0),
			wantCalls: 1,
		},
		{
			name:          "pr-error",
			initialCursor: "bad",
			isPR:          true,
			expectedQuery: "FetchMorePRComments",
			steps: []step{
				{
					expectCursor: "bad",
					errMsg:       "fail",
				},
			},
			expectErr: true,
			wantCalls: 1,
		},
		{
			name:          "pr-max-iterations",
			initialCursor: "start",
			isPR:          true,
			expectedQuery: "FetchMorePRComments",
			dynamic: func(call int, cursor string) (int, any) {
				page := call + 1
				nodes := []Comment{
					{ID: fmt.Sprintf("c-%d", page), Body: fmt.Sprintf("body-%d", page), Author: Author{Login: "user"}, CreatedAt: "t"},
				}
				payload := map[string]any{
					"nodes": nodes,
					"pageInfo": map[string]any{
						"hasNextPage": true,
						"endCursor":   "keep",
					},
				}
				return http.StatusOK, map[string]any{
					"data": map[string]any{
						"repository": map[string]any{
							"pullRequest": map[string]any{"comments": payload},
						},
					},
				}
			},
			wantLen:   intPtr(maxPaginationIterations),
			wantFirst: "body-1",
			wantLast:  fmt.Sprintf("body-%d", maxPaginationIterations),
			wantCalls: maxPaginationIterations,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			ts := newGraphQLServer(t, func(query string, vars map[string]any) (int, any) {
				if !strings.Contains(query, tt.expectedQuery) {
					t.Fatalf("expected query containing %q, got %q", tt.expectedQuery, query)
				}
				cursor, _ := vars["cursor"].(string)
				if tt.dynamic != nil {
					call := callCount
					callCount++
					status, body := tt.dynamic(call, cursor)
					if status == 0 {
						status = http.StatusOK
					}
					return status, body
				}
				if callCount >= len(tt.steps) {
					t.Fatalf("unexpected extra call %d", callCount)
				}
				step := tt.steps[callCount]
				callCount++
				if step.expectCursor != "" && cursor != step.expectCursor {
					t.Fatalf("cursor mismatch: got %q want %q", cursor, step.expectCursor)
				}
				if step.errMsg != "" {
					return http.StatusOK, map[string]any{
						"errors": []map[string]any{{"message": step.errMsg}},
					}
				}
				payload := map[string]any{
					"nodes": step.nodes,
					"pageInfo": map[string]any{
						"hasNextPage": step.hasNext,
						"endCursor":   step.endCursor,
					},
				}
				repo := map[string]any{}
				if tt.isPR {
					repo["pullRequest"] = map[string]any{"comments": payload}
				} else {
					repo["issue"] = map[string]any{"comments": payload}
				}
				return http.StatusOK, map[string]any{
					"data": map[string]any{
						"repository": repo,
					},
				}
			})
			defer ts.Close()

			client := NewClient(fakeAuth2{})
			client.endpoint = ts.URL

			got, err := fetchAllRemainingComments(ctx, client, "o", "r", 5, tt.initialCursor, tt.isPR)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				if tt.wantCalls > 0 && callCount != tt.wantCalls {
					t.Fatalf("want %d calls, got %d", tt.wantCalls, callCount)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantLen != nil && len(got) != *tt.wantLen {
				t.Fatalf("len(got) = %d want %d", len(got), *tt.wantLen)
			}
			if len(tt.wantBodies) > 0 {
				if len(got) != len(tt.wantBodies) {
					t.Fatalf("len mismatch: got %d want %d", len(got), len(tt.wantBodies))
				}
				for i, wantBody := range tt.wantBodies {
					if got[i].Body != wantBody {
						t.Fatalf("comment %d mismatch: got %q want %q", i, got[i].Body, wantBody)
					}
				}
			}
			if tt.wantFirst != "" {
				if len(got) == 0 || got[0].Body != tt.wantFirst {
					t.Fatalf("first body mismatch: got %v want %q", got, tt.wantFirst)
				}
			}
			if tt.wantLast != "" {
				if len(got) == 0 || got[len(got)-1].Body != tt.wantLast {
					t.Fatalf("last body mismatch: got %v want %q", got, tt.wantLast)
				}
			}
			if tt.wantCalls > 0 && callCount != tt.wantCalls {
				t.Fatalf("want %d calls, got %d", tt.wantCalls, callCount)
			}
		})
	}
}

func TestFetchAllRemainingReviews(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	intPtr := func(v int) *int { return &v }

	type step struct {
		expectCursor string
		nodes        []Review
		hasNext      bool
		endCursor    string
		errMsg       string
	}

	tests := []struct {
		name          string
		initialCursor string
		steps         []step
		dynamic       func(call int, cursor string) (int, any)
		expectErr     bool
		wantIDs       []string
		wantLen       *int
		wantFirst     string
		wantLast      string
		wantCalls     int
	}{
		{
			name:          "single-page",
			initialCursor: "cursor1",
			steps: []step{
				{
					expectCursor: "cursor1",
					nodes: []Review{
						{ID: "r1", Body: "b1"},
					},
					hasNext: false,
				},
			},
			wantIDs:   []string{"r1"},
			wantCalls: 1,
		},
		{
			name:          "multi-page",
			initialCursor: "cursor1",
			steps: []step{
				{
					expectCursor: "cursor1",
					nodes: []Review{
						{ID: "r1"},
						{ID: "r2"},
					},
					hasNext:   true,
					endCursor: "cursor2",
				},
				{
					expectCursor: "cursor2",
					nodes: []Review{
						{ID: "r3"},
					},
					hasNext:   true,
					endCursor: "cursor3",
				},
				{
					expectCursor: "cursor3",
					nodes: []Review{
						{ID: "r4"},
					},
					hasNext: false,
				},
			},
			wantIDs:   []string{"r1", "r2", "r3", "r4"},
			wantCalls: 3,
		},
		{
			name:          "empty-results",
			initialCursor: "start",
			steps: []step{
				{
					expectCursor: "start",
					nodes:        nil,
					hasNext:      false,
				},
			},
			wantLen:   intPtr(0),
			wantCalls: 1,
		},
		{
			name:          "graphql-error",
			initialCursor: "bad",
			steps: []step{
				{
					expectCursor: "bad",
					errMsg:       "error",
				},
			},
			expectErr: true,
			wantCalls: 1,
		},
		{
			name:          "max-iterations",
			initialCursor: "start",
			dynamic: func(call int, cursor string) (int, any) {
				page := call + 1
				return http.StatusOK, map[string]any{
					"data": map[string]any{
						"repository": map[string]any{
							"pullRequest": map[string]any{
								"reviews": map[string]any{
									"nodes": []Review{
										{ID: fmt.Sprintf("review-%d", page)},
									},
									"pageInfo": map[string]any{
										"hasNextPage": true,
										"endCursor":   "loop",
									},
								},
							},
						},
					},
				}
			},
			wantLen:   intPtr(maxPaginationIterations),
			wantFirst: "review-1",
			wantLast:  fmt.Sprintf("review-%d", maxPaginationIterations),
			wantCalls: maxPaginationIterations,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			ts := newGraphQLServer(t, func(query string, vars map[string]any) (int, any) {
				if !strings.Contains(query, "FetchMoreReviews") {
					t.Fatalf("unexpected query %q", query)
				}
				cursor, _ := vars["cursor"].(string)
				if tt.dynamic != nil {
					call := callCount
					callCount++
					status, body := tt.dynamic(call, cursor)
					if status == 0 {
						status = http.StatusOK
					}
					return status, body
				}
				if callCount >= len(tt.steps) {
					t.Fatalf("unexpected extra call %d", callCount)
				}
				step := tt.steps[callCount]
				callCount++
				if step.expectCursor != "" && cursor != step.expectCursor {
					t.Fatalf("cursor mismatch: got %q want %q", cursor, step.expectCursor)
				}
				if step.errMsg != "" {
					return http.StatusOK, map[string]any{
						"errors": []map[string]any{{"message": step.errMsg}},
					}
				}
				return http.StatusOK, map[string]any{
					"data": map[string]any{
						"repository": map[string]any{
							"pullRequest": map[string]any{
								"reviews": map[string]any{
									"nodes": step.nodes,
									"pageInfo": map[string]any{
										"hasNextPage": step.hasNext,
										"endCursor":   step.endCursor,
									},
								},
							},
						},
					},
				}
			})
			defer ts.Close()

			client := NewClient(fakeAuth2{})
			client.endpoint = ts.URL

			got, err := fetchAllRemainingReviews(ctx, client, "o", "r", 9, tt.initialCursor)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				if tt.wantCalls > 0 && callCount != tt.wantCalls {
					t.Fatalf("want %d calls, got %d", tt.wantCalls, callCount)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantLen != nil && len(got) != *tt.wantLen {
				t.Fatalf("len(got) = %d want %d", len(got), *tt.wantLen)
			}
			if len(tt.wantIDs) > 0 {
				if len(got) != len(tt.wantIDs) {
					t.Fatalf("len mismatch: got %d want %d", len(got), len(tt.wantIDs))
				}
				for i, wantID := range tt.wantIDs {
					if got[i].ID != wantID {
						t.Fatalf("review %d mismatch: got %q want %q", i, got[i].ID, wantID)
					}
				}
			}
			if tt.wantFirst != "" {
				if len(got) == 0 || got[0].ID != tt.wantFirst {
					t.Fatalf("first review mismatch: got %v want %q", got, tt.wantFirst)
				}
			}
			if tt.wantLast != "" {
				if len(got) == 0 || got[len(got)-1].ID != tt.wantLast {
					t.Fatalf("last review mismatch: got %v want %q", got, tt.wantLast)
				}
			}
			if tt.wantCalls > 0 && callCount != tt.wantCalls {
				t.Fatalf("want %d calls, got %d", tt.wantCalls, callCount)
			}
		})
	}
}

func TestFetchAllReviewComments(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	intPtr := func(v int) *int { return &v }

	type step struct {
		expectCursor string
		nodes        []ReviewComment
		hasNext      bool
		endCursor    string
		errMsg       string
	}

	tests := []struct {
		name          string
		reviewID      string
		initialCursor string
		steps         []step
		dynamic       func(call int, cursor string) (int, any)
		expectErr     bool
		wantBodies    []string
		wantLen       *int
		wantFirst     string
		wantLast      string
		wantCalls     int
	}{
		{
			name:          "single-page",
			reviewID:      "review-1",
			initialCursor: "cur1",
			steps: []step{
				{
					expectCursor: "cur1",
					nodes: []ReviewComment{
						{Comment: Comment{ID: "c1", Body: "c1", Author: Author{Login: "u1"}, CreatedAt: "t1"}},
					},
					hasNext: false,
				},
			},
			wantBodies: []string{"c1"},
			wantCalls:  1,
		},
		{
			name:          "multi-page",
			reviewID:      "review-2",
			initialCursor: "cur1",
			steps: []step{
				{
					expectCursor: "cur1",
					nodes: []ReviewComment{
						{Comment: Comment{ID: "c1", Body: "c1"}},
					},
					hasNext:   true,
					endCursor: "cur2",
				},
				{
					expectCursor: "cur2",
					nodes: []ReviewComment{
						{Comment: Comment{ID: "c2", Body: "c2"}},
					},
					hasNext:   true,
					endCursor: "cur3",
				},
				{
					expectCursor: "cur3",
					nodes: []ReviewComment{
						{Comment: Comment{ID: "c3", Body: "c3"}},
					},
					hasNext: false,
				},
			},
			wantBodies: []string{"c1", "c2", "c3"},
			wantCalls:  3,
		},
		{
			name:          "empty-results",
			reviewID:      "review-3",
			initialCursor: "cur",
			steps: []step{
				{
					expectCursor: "cur",
					nodes:        nil,
					hasNext:      false,
				},
			},
			wantLen:   intPtr(0),
			wantCalls: 1,
		},
		{
			name:          "graphql-error",
			reviewID:      "review-4",
			initialCursor: "oops",
			steps: []step{
				{
					expectCursor: "oops",
					errMsg:       "bad",
				},
			},
			expectErr: true,
			wantCalls: 1,
		},
		{
			name:          "max-iterations",
			reviewID:      "review-5",
			initialCursor: "start",
			dynamic: func(call int, cursor string) (int, any) {
				page := call + 1
				return http.StatusOK, map[string]any{
					"data": map[string]any{
						"node": map[string]any{
							"comments": map[string]any{
								"nodes": []ReviewComment{
									{Comment: Comment{ID: fmt.Sprintf("c-%d", page), Body: fmt.Sprintf("body-%d", page)}},
								},
								"pageInfo": map[string]any{
									"hasNextPage": true,
									"endCursor":   "loop",
								},
							},
						},
					},
				}
			},
			wantLen:   intPtr(maxPaginationIterations),
			wantFirst: "body-1",
			wantLast:  fmt.Sprintf("body-%d", maxPaginationIterations),
			wantCalls: maxPaginationIterations,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			ts := newGraphQLServer(t, func(query string, vars map[string]any) (int, any) {
				if !strings.Contains(query, "FetchMoreReviewComments") {
					t.Fatalf("unexpected query %q", query)
				}
				if reviewID, _ := vars["reviewId"].(string); reviewID != tt.reviewID {
					t.Fatalf("expected reviewId %q, got %q", tt.reviewID, reviewID)
				}
				cursor, _ := vars["cursor"].(string)
				if tt.dynamic != nil {
					call := callCount
					callCount++
					status, body := tt.dynamic(call, cursor)
					if status == 0 {
						status = http.StatusOK
					}
					return status, body
				}
				if callCount >= len(tt.steps) {
					t.Fatalf("unexpected extra call %d", callCount)
				}
				step := tt.steps[callCount]
				callCount++
				if step.expectCursor != "" && cursor != step.expectCursor {
					t.Fatalf("cursor mismatch: got %q want %q", cursor, step.expectCursor)
				}
				if step.errMsg != "" {
					return http.StatusOK, map[string]any{
						"errors": []map[string]any{{"message": step.errMsg}},
					}
				}
				return http.StatusOK, map[string]any{
					"data": map[string]any{
						"node": map[string]any{
							"comments": map[string]any{
								"nodes": step.nodes,
								"pageInfo": map[string]any{
									"hasNextPage": step.hasNext,
									"endCursor":   step.endCursor,
								},
							},
						},
					},
				}
			})
			defer ts.Close()

			client := NewClient(fakeAuth2{})
			client.endpoint = ts.URL

			got, err := fetchAllReviewComments(ctx, client, "o/r", tt.reviewID, tt.initialCursor)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				if tt.wantCalls > 0 && callCount != tt.wantCalls {
					t.Fatalf("want %d calls, got %d", tt.wantCalls, callCount)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantLen != nil && len(got) != *tt.wantLen {
				t.Fatalf("len(got) = %d want %d", len(got), *tt.wantLen)
			}
			if len(tt.wantBodies) > 0 {
				if len(got) != len(tt.wantBodies) {
					t.Fatalf("len mismatch: got %d want %d", len(got), len(tt.wantBodies))
				}
				for i, wantBody := range tt.wantBodies {
					if got[i].Body != wantBody {
						t.Fatalf("review comment %d mismatch: got %q want %q", i, got[i].Body, wantBody)
					}
				}
			}
			if tt.wantFirst != "" {
				if len(got) == 0 || got[0].Body != tt.wantFirst {
					t.Fatalf("first body mismatch: got %v want %q", got, tt.wantFirst)
				}
			}
			if tt.wantLast != "" {
				if len(got) == 0 || got[len(got)-1].Body != tt.wantLast {
					t.Fatalf("last body mismatch: got %v want %q", got, tt.wantLast)
				}
			}
			if tt.wantCalls > 0 && callCount != tt.wantCalls {
				t.Fatalf("want %d calls, got %d", tt.wantCalls, callCount)
			}
		})
	}
}

func TestFormatComments_Empty(t *testing.T) {
	result := FormatComments([]Comment{})
	if result != "" {
		t.Errorf("FormatComments([]) should return empty string, got: %q", result)
	}
}

func TestFormatComments_SingleComment(t *testing.T) {
	comments := []Comment{
		{
			Body:      "test comment body",
			Author:    Author{Login: "testuser"},
			CreatedAt: "2024-01-01T10:00:00Z",
		},
	}

	result := FormatComments(comments)

	if !strings.Contains(result, "testuser") {
		t.Error("Formatted comments should contain author login")
	}
	if !strings.Contains(result, "2024-01-01T10:00:00Z") {
		t.Error("Formatted comments should contain timestamp")
	}
	if !strings.Contains(result, "test comment body") {
		t.Error("Formatted comments should contain body text")
	}
	if !strings.Contains(result, "[testuser at 2024-01-01T10:00:00Z]:") {
		t.Error("Formatted comments should have correct format")
	}
}

func TestFormatComments_MultipleComments(t *testing.T) {
	comments := []Comment{
		{Body: "first", Author: Author{Login: "user1"}, CreatedAt: "2024-01-01T10:00:00Z"},
		{Body: "second", Author: Author{Login: "user2"}, CreatedAt: "2024-01-02T10:00:00Z"},
	}

	result := FormatComments(comments)

	if !strings.Contains(result, "first") {
		t.Error("Should contain first comment")
	}
	if !strings.Contains(result, "second") {
		t.Error("Should contain second comment")
	}
	if !strings.Contains(result, "user1") {
		t.Error("Should contain first user")
	}
	if !strings.Contains(result, "user2") {
		t.Error("Should contain second user")
	}
}

func TestFormatComments_SkipsMinimizedComments(t *testing.T) {
	comments := []Comment{
		{Body: "visible", Author: Author{Login: "user1"}, CreatedAt: "2024-01-01T10:00:00Z", IsMinimized: false},
		{Body: "hidden", Author: Author{Login: "user2"}, CreatedAt: "2024-01-02T10:00:00Z", IsMinimized: true},
	}

	result := FormatComments(comments)

	if !strings.Contains(result, "visible") {
		t.Error("Should contain visible comment")
	}
	if strings.Contains(result, "hidden") {
		t.Error("Should not contain minimized comment")
	}
}

func TestFormatChangedFilesWithSHA_Empty(t *testing.T) {
	result := FormatChangedFilesWithSHA([]GitHubFileWithSHA{})
	if result != "" {
		t.Errorf("FormatChangedFilesWithSHA([]) should return empty string, got: %q", result)
	}
}

func TestFormatChangedFilesWithSHA_SingleFile(t *testing.T) {
	files := []GitHubFileWithSHA{
		{
			File: File{
				Path:       "main.go",
				ChangeType: "MODIFIED",
				Additions:  10,
				Deletions:  5,
			},
			SHA: "abc123",
		},
	}

	result := FormatChangedFilesWithSHA(files)

	if !strings.Contains(result, "main.go") {
		t.Error("Should contain file path")
	}
	if !strings.Contains(result, "MODIFIED") {
		t.Error("Should contain change type")
	}
	if !strings.Contains(result, "+10") {
		t.Error("Should contain additions")
	}
	if !strings.Contains(result, "-5") {
		t.Error("Should contain deletions")
	}
	if !strings.Contains(result, "SHA: abc123") {
		t.Error("Should contain SHA")
	}
}

func TestFormatChangedFilesWithSHA_MultipleFiles(t *testing.T) {
	files := []GitHubFileWithSHA{
		{
			File: File{Path: "file1.go", ChangeType: "ADDED", Additions: 100, Deletions: 0},
			SHA:  "sha1",
		},
		{
			File: File{Path: "file2.go", ChangeType: "DELETED", Additions: 0, Deletions: 50},
			SHA:  "deleted",
		},
	}

	result := FormatChangedFilesWithSHA(files)

	if !strings.Contains(result, "file1.go") {
		t.Error("Should contain first file")
	}
	if !strings.Contains(result, "file2.go") {
		t.Error("Should contain second file")
	}
	if !strings.Contains(result, "ADDED") {
		t.Error("Should contain ADDED change type")
	}
	if !strings.Contains(result, "DELETED") {
		t.Error("Should contain DELETED change type")
	}
	if !strings.Contains(result, "sha1") {
		t.Error("Should contain first SHA")
	}
	if !strings.Contains(result, "deleted") {
		t.Error("Should contain deleted marker")
	}
}

func TestFormatChangedFilesWithSHA_LineFormat(t *testing.T) {
	files := []GitHubFileWithSHA{
		{
			File: File{Path: "test.go", ChangeType: "MODIFIED", Additions: 5, Deletions: 3},
			SHA:  "abc123",
		},
	}

	result := FormatChangedFilesWithSHA(files)

	expected := "- test.go (MODIFIED) +5/-3 SHA: abc123"
	if result != expected {
		t.Errorf("FormatChangedFilesWithSHA() = %q, want %q", result, expected)
	}
}

// -------------------- New tests for fetcher.go behaviors --------------------

type fakeAuth2 struct{}

func (f fakeAuth2) GetInstallationToken(repo string) (*gh.InstallationToken, error) {
	return &gh.InstallationToken{Token: "t", ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (f fakeAuth2) GetInstallationOwner(repo string) (string, error) { return "o", nil }

// helper to create graphql test server with programmable responses
func newGraphQLServer(t *testing.T, handler func(query string, vars map[string]any) (status int, body any)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errors":[{"message":"bad req"}]}`))
			return
		}
		status, body := handler(req.Query, req.Variables)
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
}

func TestFetchGitHubData_Issue(t *testing.T) {
	ts := newGraphQLServer(t, func(query string, vars map[string]any) (int, any) {
		if strings.Contains(query, "issue(") {
			return 200, map[string]any{"data": map[string]any{"repository": map[string]any{"issue": map[string]any{
				"title":     "Bug",
				"body":      "Body",
				"author":    map[string]any{"login": "bob"},
				"createdAt": "t",
				"state":     "OPEN",
				"comments": map[string]any{"nodes": []any{
					map[string]any{"id": "1", "databaseId": 1, "body": "c1", "author": map[string]any{"login": "u1"}, "createdAt": "t1", "isMinimized": false},
				}},
			}}}}
		}
		if strings.Contains(query, "User(") {
			login, _ := vars["login"].(string)
			name := ""
			if login == "trig" {
				name = "Trigger User"
			}
			return 200, map[string]any{"data": map[string]any{"user": map[string]any{"name": name}}}
		}
		t.Fatalf("unexpected query: %s", query)
		return 200, nil
	})
	defer ts.Close()

	c := NewClient(fakeAuth2{})
	c.endpoint = ts.URL

	res, err := FetchGitHubData(context.Background(), FetchParams{Client: c, Repository: "o/r", Number: 1, IsPR: false, TriggerUsername: "trig"})
	if err != nil {
		t.Fatalf("FetchGitHubData issue: %v", err)
	}
	if _, ok := res.ContextData.(Issue); !ok {
		t.Fatalf("ContextData should be Issue")
	}
	if len(res.Comments) != 1 {
		t.Fatalf("want 1 comment, got %d", len(res.Comments))
	}
	if res.TriggerName == nil || *res.TriggerName != "Trigger User" {
		t.Fatalf("unexpected trigger name: %+v", res.TriggerName)
	}
}

func TestFetchGitHubData_PR(t *testing.T) {
	tmp := t.TempDir()
	fpath := filepath.Join(tmp, "file.go")
	if err := os.WriteFile(fpath, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ts := newGraphQLServer(t, func(query string, vars map[string]any) (int, any) {
		if strings.Contains(query, "pullRequest(") {
			return 200, map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
				"title": "P", "body": "B", "author": map[string]any{"login": "alice"},
				"baseRefName": "main", "headRefName": "f", "headRefOid": "deadbeef", "createdAt": "t", "additions": 1, "deletions": 0, "state": "OPEN",
				"commits": map[string]any{"totalCount": 1, "nodes": []any{map[string]any{"commit": map[string]any{"oid": "c", "message": "m", "author": map[string]any{"name": "n", "email": "e"}}}}},
				"files": map[string]any{"nodes": []any{
					map[string]any{"path": fpath, "additions": 1, "deletions": 0, "changeType": "MODIFIED"},
					map[string]any{"path": "deleted.txt", "additions": 0, "deletions": 0, "changeType": "DELETED"},
				}},
				"comments": map[string]any{"nodes": []any{}},
				"reviews": map[string]any{"nodes": []any{map[string]any{"id": "r1", "databaseId": 1, "author": map[string]any{"login": "rev"}, "body": "ok", "state": "COMMENTED", "submittedAt": "t",
					"comments": map[string]any{"nodes": []any{map[string]any{"id": "c1", "databaseId": 2, "body": "inl", "author": map[string]any{"login": "u"}, "createdAt": "t", "isMinimized": false, "path": "p.go", "line": 10}}},
				}}},
			}}}}
		}
		if strings.Contains(query, "User(") {
			return 200, map[string]any{"data": map[string]any{"user": map[string]any{"name": "Alice"}}}
		}
		t.Fatalf("unexpected query: %s", query)
		return 200, nil
	})
	defer ts.Close()

	c := NewClient(fakeAuth2{})
	c.endpoint = ts.URL
	res, err := FetchGitHubData(context.Background(), FetchParams{Client: c, Repository: "o/r", Number: 2, IsPR: true, TriggerUsername: "alice"})
	if err != nil {
		t.Fatalf("FetchGitHubData pr: %v", err)
	}
	if _, ok := res.ContextData.(PullRequest); !ok {
		t.Fatalf("ContextData should be PR")
	}
	if len(res.Changed) != 2 {
		t.Fatalf("want 2 files, got %d", len(res.Changed))
	}
	if len(res.ChangedSHA) != 2 {
		t.Fatalf("want 2 with sha, got %d", len(res.ChangedSHA))
	}
	// one deleted marker present
	foundDeleted := false
	for _, f := range res.ChangedSHA {
		if f.SHA == "deleted" {
			foundDeleted = true
		}
	}
	if !foundDeleted {
		t.Fatalf("missing deleted marker in ChangedSHA: %+v", res.ChangedSHA)
	}
	// non-deleted has non-empty and not unknown
	reHex := regexp.MustCompile(`^[a-f0-9]{7,64}$`)
	oksha := false
	for _, f := range res.ChangedSHA {
		if f.Path == fpath {
			oksha = f.SHA != "" && f.SHA != "unknown" && reHex.MatchString(f.SHA)
		}
	}
	if !oksha {
		t.Fatalf("unexpected blob sha: %+v", res.ChangedSHA)
	}
	if res.TriggerName == nil || *res.TriggerName != "Alice" {
		t.Fatalf("bad trigger name: %+v", res.TriggerName)
	}
}

func TestFetchGitHubData_GraphQLError(t *testing.T) {
	ts := newGraphQLServer(t, func(query string, vars map[string]any) (int, any) {
		if strings.Contains(query, "pullRequest(") {
			return 200, map[string]any{"errors": []map[string]string{{"message": "nope"}}}
		}
		if strings.Contains(query, "issue(") {
			return 200, map[string]any{"errors": []map[string]string{{"message": "nope"}}}
		}
		return 200, map[string]any{"data": map[string]any{}}
	})
	defer ts.Close()
	c := NewClient(fakeAuth2{})
	c.endpoint = ts.URL
	if _, err := FetchGitHubData(context.Background(), FetchParams{Client: c, Repository: "o/r", Number: 1, IsPR: true}); err == nil {
		t.Fatalf("expected error for PR fetch")
	}
	if _, err := FetchGitHubData(context.Background(), FetchParams{Client: c, Repository: "o/r", Number: 1, IsPR: false}); err == nil {
		t.Fatalf("expected error for Issue fetch")
	}
}

func TestFetchUserDisplayName(t *testing.T) {
	ts := newGraphQLServer(t, func(query string, vars map[string]any) (int, any) {
		login, _ := vars["login"].(string)
		if login == "has" {
			return 200, map[string]any{"data": map[string]any{"user": map[string]any{"name": "Has Name"}}}
		}
		// null name
		return 200, map[string]any{"data": map[string]any{"user": map[string]any{"name": nil}}}
	})
	defer ts.Close()
	c := NewClient(fakeAuth2{})
	c.endpoint = ts.URL
	name, err := FetchUserDisplayName(context.Background(), c, "o/r", "has")
	if err != nil || name == nil || *name != "Has Name" {
		t.Fatalf("unexpected: %v %v", err, name)
	}
	name, err = FetchUserDisplayName(context.Background(), c, "o/r", "none")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != nil {
		t.Fatalf("expected nil name when server returns null")
	}
}

func TestGitHashObject(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "x.txt")
	if err := os.WriteFile(p, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sha, err := gitHashObject(p)
	if err != nil || sha == "" {
		t.Fatalf("unexpected: %v %q", err, sha)
	}
	if _, err = gitHashObject(filepath.Join(tmp, "nope.txt")); err == nil {
		t.Fatalf("expected error for missing file")
	}
}

func TestFetchGitHubData_PR_Pagination(t *testing.T) {
	tmp := t.TempDir()
	file1 := filepath.Join(tmp, "page1.go")
	file2 := filepath.Join(tmp, "page2.go")
	if err := os.WriteFile(file1, []byte("package x\nconst a = 1\n"), 0o644); err != nil {
		t.Fatalf("write file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("package x\nconst b = 2\n"), 0o644); err != nil {
		t.Fatalf("write file2: %v", err)
	}

	var (
		fileCalls         int
		prCommentCalls    int
		reviewCalls       int
		reviewCommentCalls int
	)

	ts := newGraphQLServer(t, func(query string, vars map[string]any) (int, any) {
		switch {
		case strings.Contains(query, "FetchMoreReviewComments"):
			reviewCommentCalls++
			return 200, map[string]any{"data": map[string]any{"node": map[string]any{"comments": map[string]any{
				"pageInfo": map[string]any{"hasNextPage": false, "endCursor": ""},
				"nodes": []any{
					map[string]any{"id": "rc2", "databaseId": 201, "body": "follow up", "author": map[string]any{"login": "rc2"}, "createdAt": "2024-01-03T00:00:00Z", "isMinimized": false, "path": "file.go", "line": 11},
				},
			}}}}
		case strings.Contains(query, "FetchMoreReviews"):
			reviewCalls++
			return 200, map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{"reviews": map[string]any{
				"pageInfo": map[string]any{"hasNextPage": false, "endCursor": ""},
				"nodes": []any{map[string]any{
					"id":          "review-2",
					"databaseId":  101,
					"author":      map[string]any{"login": "rev2"},
					"body":        "another look",
					"state":       "APPROVED",
					"submittedAt": "2024-01-02T00:00:00Z",
					"comments": map[string]any{"pageInfo": map[string]any{"hasNextPage": false, "endCursor": ""}, "nodes": []any{}},
				}},
			}}}}}
		case strings.Contains(query, "FetchMorePRComments"):
			prCommentCalls++
			return 200, map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{"comments": map[string]any{
				"pageInfo": map[string]any{"hasNextPage": false, "endCursor": ""},
				"nodes":    []any{map[string]any{"id": "c2", "databaseId": 2, "body": "second", "author": map[string]any{"login": "carol"}, "createdAt": "2024-01-02T00:00:00Z", "isMinimized": false}},
			}}}}}
		case strings.Contains(query, "FetchMoreFiles"):
			fileCalls++
			return 200, map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{"files": map[string]any{
				"pageInfo": map[string]any{"hasNextPage": false, "endCursor": ""},
				"nodes":    []any{map[string]any{"path": file2, "additions": 5, "deletions": 0, "changeType": "MODIFIED"}},
			}}}}}
		case strings.Contains(query, "pullRequest("):
			return 200, map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
				"title":        "Paginated",
				"body":         "Body",
				"author":       map[string]any{"login": "alice"},
				"baseRefName":  "main",
				"headRefName":  "feature",
				"headRefOid":   "deadbeef",
				"createdAt":    "2024-01-01T00:00:00Z",
				"additions":    10,
				"deletions":    2,
				"state":        "OPEN",
				"commits":      map[string]any{"totalCount": 1, "nodes": []any{map[string]any{"commit": map[string]any{"oid": "c1", "message": "m", "author": map[string]any{"name": "n", "email": "e"}}}}},
				"files":        map[string]any{"pageInfo": map[string]any{"hasNextPage": true, "endCursor": "file-c2"}, "nodes": []any{map[string]any{"path": file1, "additions": 5, "deletions": 0, "changeType": "MODIFIED"}}},
				"comments":     map[string]any{"pageInfo": map[string]any{"hasNextPage": true, "endCursor": "comment-c2"}, "nodes": []any{map[string]any{"id": "c1", "databaseId": 1, "body": "first", "author": map[string]any{"login": "bob"}, "createdAt": "2024-01-01T00:00:00Z", "isMinimized": false}}},
				"reviews":      map[string]any{"pageInfo": map[string]any{"hasNextPage": true, "endCursor": "review-c2"}, "nodes": []any{map[string]any{
					"id":          "review-1",
					"databaseId":  100,
					"author":      map[string]any{"login": "rev"},
					"body":        "looks good",
					"state":       "COMMENTED",
					"submittedAt": "2024-01-01T00:00:00Z",
					"comments": map[string]any{"pageInfo": map[string]any{"hasNextPage": true, "endCursor": "rc-c2"}, "nodes": []any{
						map[string]any{"id": "rc1", "databaseId": 200, "body": "needs tweak", "author": map[string]any{"login": "rc"}, "createdAt": "2024-01-01T00:00:00Z", "isMinimized": false, "path": "file.go", "line": 10},
					}},
				}}},
			}}}}
		case strings.Contains(query, "User("):
			return 200, map[string]any{"data": map[string]any{"user": map[string]any{"name": "Trigger User"}}}
		default:
			t.Fatalf("unexpected query: %s", query)
		}
		return 200, nil
	})
	defer ts.Close()

	c := NewClient(fakeAuth2{})
	c.endpoint = ts.URL

	res, err := FetchGitHubData(context.Background(), FetchParams{
		Client:          c,
		Repository:      "o/r",
		Number:          10,
		IsPR:            true,
		TriggerUsername: "trigger",
	})
	if err != nil {
		t.Fatalf("FetchGitHubData pagination: %v", err)
	}
	if fileCalls != 1 {
		t.Fatalf("expected 1 FetchMoreFiles call, got %d", fileCalls)
	}
	if prCommentCalls != 1 {
		t.Fatalf("expected 1 FetchMorePRComments call, got %d", prCommentCalls)
	}
	if reviewCalls != 1 {
		t.Fatalf("expected 1 FetchMoreReviews call, got %d", reviewCalls)
	}
	if reviewCommentCalls != 1 {
		t.Fatalf("expected 1 FetchMoreReviewComments call, got %d", reviewCommentCalls)
	}
	if len(res.Changed) != 2 {
		t.Fatalf("expected 2 files, got %d: %#v", len(res.Changed), res.Changed)
	}
	if len(res.Comments) != 2 {
		t.Fatalf("expected 2 PR comments, got %d: %#v", len(res.Comments), res.Comments)
	}
	if res.Reviews == nil || len(res.Reviews.Nodes) != 2 {
		t.Fatalf("expected 2 reviews, got %#v", res.Reviews)
	}
	if len(res.Reviews.Nodes[0].Comments.Nodes) != 2 {
		t.Fatalf("expected 2 review comments on first review, got %d", len(res.Reviews.Nodes[0].Comments.Nodes))
	}
	if res.TriggerName == nil || *res.TriggerName != "Trigger User" {
		t.Fatalf("bad trigger name: %+v", res.TriggerName)
	}
}

func TestFetchGitHubData_IssuePagination(t *testing.T) {
	ts := newGraphQLServer(t, func(query string, vars map[string]any) (int, any) {
		switch {
		case strings.Contains(query, "FetchMoreIssueComments"):
			return 200, map[string]any{"data": map[string]any{"repository": map[string]any{"issue": map[string]any{"comments": map[string]any{
				"pageInfo": map[string]any{"hasNextPage": false, "endCursor": ""},
				"nodes": []any{
					map[string]any{"id": "ic2", "databaseId": 2, "body": "second", "author": map[string]any{"login": "user2"}, "createdAt": "2024-01-02T00:00:00Z", "isMinimized": false},
				},
			}}}}}
		case strings.Contains(query, "issue("):
			return 200, map[string]any{"data": map[string]any{"repository": map[string]any{"issue": map[string]any{
				"title":     "Issue",
				"body":      "Body",
				"author":    map[string]any{"login": "reporter"},
				"createdAt": "2024-01-01T00:00:00Z",
				"state":     "OPEN",
				"comments": map[string]any{"pageInfo": map[string]any{"hasNextPage": true, "endCursor": "issue-c2"}, "nodes": []any{
					map[string]any{"id": "ic1", "databaseId": 1, "body": "first", "author": map[string]any{"login": "user1"}, "createdAt": "2024-01-01T00:00:00Z", "isMinimized": false},
				}},
			}}}}
		case strings.Contains(query, "User("):
			return 200, map[string]any{"data": map[string]any{"user": map[string]any{"name": "Issue Trigger"}}}
		default:
			t.Fatalf("unexpected query: %s", query)
		}
		return 200, nil
	})
	defer ts.Close()

	c := NewClient(fakeAuth2{})
	c.endpoint = ts.URL

	res, err := FetchGitHubData(context.Background(), FetchParams{
		Client:          c,
		Repository:      "o/r",
		Number:          11,
		IsPR:            false,
		TriggerUsername: "trigger",
	})
	if err != nil {
		t.Fatalf("FetchGitHubData issue pagination: %v", err)
	}
	if len(res.Comments) != 2 {
		t.Fatalf("expected 2 issue comments, got %d: %#v", len(res.Comments), res.Comments)
	}
	if res.TriggerName == nil || *res.TriggerName != "Issue Trigger" {
		t.Fatalf("bad trigger name: %+v", res.TriggerName)
	}
}

func TestFetchGitHubData_PaginationErrors(t *testing.T) {
	t.Helper()
	type scenario struct {
		name      string
		isPR      bool
		failQuery string
		expect    string
	}
	cases := []scenario{
		{name: "files", isPR: true, failQuery: "FetchMoreFiles", expect: "fetch remaining files"},
		{name: "pr comments", isPR: true, failQuery: "FetchMorePRComments", expect: "fetch remaining PR comments"},
		{name: "reviews", isPR: true, failQuery: "FetchMoreReviews", expect: "fetch remaining reviews"},
		{name: "review comments", isPR: true, failQuery: "FetchMoreReviewComments", expect: "fetch remaining review comments for review review-err"},
		{name: "issue comments", isPR: false, failQuery: "FetchMoreIssueComments", expect: "fetch remaining issue comments"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ts := newGraphQLServer(t, func(query string, vars map[string]any) (int, any) {
				if strings.Contains(query, tc.failQuery) {
					return 200, map[string]any{"errors": []map[string]any{{"message": tc.failQuery + " failed"}}}
				}
				if tc.isPR && strings.Contains(query, "pullRequest(") {
					hasNext := map[string]bool{
						"files":          tc.failQuery == "FetchMoreFiles",
						"prComments":     tc.failQuery == "FetchMorePRComments",
						"reviews":        tc.failQuery == "FetchMoreReviews",
						"reviewComments": tc.failQuery == "FetchMoreReviewComments",
					}
					return 200, map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
						"title":       "Err PR",
						"body":        "Body",
						"author":      map[string]any{"login": "alice"},
						"baseRefName": "main",
						"headRefName": "feature",
						"headRefOid":  "deadbeef",
						"createdAt":   "2024-01-01T00:00:00Z",
						"additions":   1,
						"deletions":   0,
						"state":       "OPEN",
						"commits":     map[string]any{"totalCount": 0, "nodes": []any{}},
						"files": map[string]any{"pageInfo": map[string]any{"hasNextPage": hasNext["files"], "endCursor": ternary(hasNext["files"], "file-cursor", "")}, "nodes": []any{
							map[string]any{"path": "deleted.txt", "additions": 0, "deletions": 0, "changeType": "DELETED"},
						}},
						"comments": map[string]any{"pageInfo": map[string]any{"hasNextPage": hasNext["prComments"], "endCursor": ternary(hasNext["prComments"], "comment-cursor", "")}, "nodes": []any{
							map[string]any{"id": "c1", "databaseId": 1, "body": "c1", "author": map[string]any{"login": "user"}, "createdAt": "2024-01-01T00:00:00Z", "isMinimized": false},
						}},
						"reviews": map[string]any{"pageInfo": map[string]any{"hasNextPage": hasNext["reviews"], "endCursor": ternary(hasNext["reviews"], "review-cursor", "")}, "nodes": []any{
							map[string]any{
								"id":          "review-err",
								"databaseId":  10,
								"author":      map[string]any{"login": "rev"},
								"body":        "b",
								"state":       "COMMENTED",
								"submittedAt": "2024-01-01T00:00:00Z",
								"comments": map[string]any{"pageInfo": map[string]any{"hasNextPage": hasNext["reviewComments"], "endCursor": ternary(hasNext["reviewComments"], "review-comment-cursor", "")}, "nodes": []any{
									map[string]any{"id": "rc1", "databaseId": 2, "body": "rc", "author": map[string]any{"login": "rev"}, "createdAt": "2024-01-01T00:00:00Z", "isMinimized": false, "path": "p.go", "line": 5},
								}},
							},
						}},
					}}}}
				}
				if !tc.isPR && strings.Contains(query, "issue(") {
					return 200, map[string]any{"data": map[string]any{"repository": map[string]any{"issue": map[string]any{
						"title":     "Err Issue",
						"body":      "Body",
						"author":    map[string]any{"login": "alice"},
						"createdAt": "2024-01-01T00:00:00Z",
						"state":     "OPEN",
						"comments": map[string]any{"pageInfo": map[string]any{"hasNextPage": true, "endCursor": "issue-cursor"}, "nodes": []any{
							map[string]any{"id": "ic1", "databaseId": 1, "body": "comment", "author": map[string]any{"login": "user"}, "createdAt": "2024-01-01T00:00:00Z", "isMinimized": false},
						}},
					}}}}
				}
				return 200, map[string]any{"data": map[string]any{}}
			})
			defer ts.Close()

			c := NewClient(fakeAuth2{})
			c.endpoint = ts.URL

			params := FetchParams{
				Client:     c,
				Repository: "o/r",
				Number:     42,
				IsPR:       tc.isPR,
			}
			_, err := FetchGitHubData(context.Background(), params)
			if err == nil || !strings.Contains(err.Error(), tc.expect) {
				t.Fatalf("expected error containing %q, got %v", tc.expect, err)
			}
		})
	}
}

// ternary is a tiny helper to keep test data declarations compact.
func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}
