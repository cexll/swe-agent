package webhook_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cexll/swe/internal/dispatcher"
	"github.com/cexll/swe/internal/executor"
	"github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/provider/claude"
	"github.com/cexll/swe/internal/taskstore"
	webhookpkg "github.com/cexll/swe/internal/webhook"
)

type integrationDispatcher struct {
	lastTask *webhookpkg.Task
	run      func(task *webhookpkg.Task) error
}

func (d *integrationDispatcher) Enqueue(task *webhookpkg.Task) error {
	d.lastTask = task
	if d.run != nil {
		return d.run(task)
	}
	return nil
}

type fakeProvider struct {
	name     string
	response *claude.CodeResponse
}

func (p *fakeProvider) GenerateCode(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
	respCopy := *p.response
	return &respCopy, nil
}

func (p *fakeProvider) Name() string {
	return p.name
}

type fakeAuth struct {
	owner string
}

func (a *fakeAuth) GetInstallationToken(repo string) (*github.InstallationToken, error) {
	return &github.InstallationToken{
		Token:     "fake-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}, nil
}

func (a *fakeAuth) GetInstallationOwner(repo string) (string, error) {
	return a.owner, nil
}

func initGitRepo(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	dir := t.TempDir()
	commands := [][]string{
		{"git", "init"},
		{"git", "config", "user.name", "SWE Agent Tests"},
		{"git", "config", "user.email", "swe-agent-tests@example.com"},
		{"git", "checkout", "-b", "main"},
	}

	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git command %v failed: %v\n%s", args, err, output)
		}
	}

	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# swe-agent\n"), 0o644); err != nil {
		t.Fatalf("failed to seed README: %v", err)
	}

	for _, args := range [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", "initial commit"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git command %v failed: %v\n%s", args, err, output)
		}
	}

	return dir
}

func TestIssueWebhookEndToEndProviders(t *testing.T) {
	secret := "test-secret"
	trigger := "/code"

	type testCase struct {
		name         string
		providerName string
		summary      string
	}

	cases := []testCase{
		{
			name:         "claude analysis only",
			providerName: "claude",
			summary:      "分析 PRD 文档并确认实现可行性，无需代码改动。",
		},
		{
			name:         "codex analysis only",
			providerName: "codex",
			summary:      "完成 docs/auto-dev-pipeline-prd-v0.1.md 的可行性分析，暂不提交代码。",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repoDir := initGitRepo(t)
			store := taskstore.NewStore()
			auth := &fakeAuth{owner: "cexll"}

			mockGH := github.NewMockGHClient()
			mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
				return 101, nil
			}
			mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
				return nil
			}
			mockGH.AddLabelFunc = func(repo string, number int, label, token string) error {
				return nil
			}

			provider := &fakeProvider{
				name: tc.providerName,
				response: &claude.CodeResponse{
					Files:   nil,
					Summary: tc.summary,
					CostUSD: 0.1234,
				},
			}

			exec := executor.NewWithClient(provider, auth, mockGH)
			exec.WithStore(store)
			exec.WithCloneFunc(func(repo, branch, token string) (string, func(), error) {
				return repoDir, func() {}, nil
			})

			dispatcher := &integrationDispatcher{
				run: func(task *webhookpkg.Task) error {
					return exec.Execute(context.Background(), task)
				},
			}

			handler := webhookpkg.NewHandler(secret, trigger, dispatcher, store, auth)

			event := &webhookpkg.IssueCommentEvent{
				Action: "created",
				Issue: webhookpkg.Issue{
					Number: 13,
					Title:  "查看 PRD 文档",
					Body:   "请分析 docs 目录下 PRD 的实现可行性。",
					State:  "open",
				},
				Comment: webhookpkg.Comment{
					ID:   3400501061,
					Body: "/code 查看prd文档 docs 目录 分析实现可行性",
					User: webhookpkg.User{Login: "cexll", Type: "User"},
				},
				Repository: webhookpkg.Repository{
					FullName:      "cexll/swe-agent",
					DefaultBranch: "main",
					Owner:         webhookpkg.User{Login: "cexll"},
					Name:          "swe-agent",
				},
				Sender: webhookpkg.User{Login: "cexll"},
			}

			payload, err := json.Marshal(event)
			if err != nil {
				t.Fatalf("failed to marshal event: %v", err)
			}

			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(payload)
			signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
			req.Header.Set("X-Hub-Signature-256", signature)
			req.Header.Set("X-GitHub-Event", "issue_comment")

			rr := httptest.NewRecorder()
			handler.Handle(rr, req)

			if rr.Code != http.StatusAccepted {
				t.Fatalf("status = %d, want %d (body: %s)", rr.Code, http.StatusAccepted, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), "Task queued") {
				t.Fatalf("unexpected response body: %s", rr.Body.String())
			}

			if dispatcher.lastTask == nil {
				t.Fatal("dispatcher did not capture task")
			}

			if len(mockGH.UpdateCommentCalls) == 0 {
				t.Fatalf("expected UpdateComment to be called at least once")
			}
			finalBody := mockGH.UpdateCommentCalls[len(mockGH.UpdateCommentCalls)-1].Body

			if !strings.Contains(finalBody, tc.summary) {
				t.Fatalf("final comment missing summary %q:\n%s", tc.summary, finalBody)
			}
			if !strings.Contains(finalBody, "SWE Agent finished @cexll's task") {
				t.Fatalf("final comment missing completion header:\n%s", finalBody)
			}

			if entry, ok := store.Get(dispatcher.lastTask.ID); !ok {
				t.Fatalf("task %s not found in store", dispatcher.lastTask.ID)
			} else if entry.Status != taskstore.StatusCompleted {
				t.Fatalf("task status = %s, want %s", entry.Status, taskstore.StatusCompleted)
			}
		})
	}
}

func TestIssueWebhookE2EWithDispatcher(t *testing.T) {
	secret := "test-secret"
	trigger := "/code"

	repoDir := initGitRepo(t)
	store := taskstore.NewStore()
	auth := &fakeAuth{owner: "cexll"}

	finalSummary := "跨阶段梳理完成，当前仅输出分析结果。"
	finalBodyCh := make(chan string, 1)

	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 2024, nil
	}
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		if strings.Contains(body, finalSummary) && strings.Contains(body, "**SWE Agent finished @cexll's task") {
			select {
			case finalBodyCh <- body:
			default:
			}
		}
		return nil
	}
	mockGH.AddLabelFunc = func(repo string, number int, label, token string) error {
		return nil
	}

	provider := &fakeProvider{
		name: "claude",
		response: &claude.CodeResponse{
			Files:   nil,
			Summary: finalSummary,
			CostUSD: 0.5678,
		},
	}

	exec := executor.NewWithClient(provider, auth, mockGH)
	exec.WithStore(store)
	exec.WithCloneFunc(func(repo, branch, token string) (string, func(), error) {
		return repoDir, func() {}, nil
	})

	disp := dispatcher.New(exec, dispatcher.Config{
		Workers:        1,
		QueueSize:      2,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     20 * time.Millisecond,
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		disp.Shutdown(ctx)
	})

	handler := webhookpkg.NewHandler(secret, trigger, disp, store, auth)

	event := &webhookpkg.IssueCommentEvent{
		Action: "created",
		Issue: webhookpkg.Issue{
			Number: 77,
			Title:  "Stage review",
			Body:   "确认工作流可行性，无需落盘代码。",
			State:  "open",
		},
		Comment: webhookpkg.Comment{
			ID:   9988,
			Body: "/code 分析当前工作流产出，确认下一步",
			User: webhookpkg.User{Login: "cexll", Type: "User"},
		},
		Repository: webhookpkg.Repository{
			FullName:      "cexll/swe-agent",
			DefaultBranch: "main",
			Owner:         webhookpkg.User{Login: "cexll"},
			Name:          "swe-agent",
		},
		Sender: webhookpkg.User{Login: "cexll"},
	}

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", signature)
	req.Header.Set("X-GitHub-Event", "issue_comment")

	rr := httptest.NewRecorder()
	handler.Handle(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (body: %s)", rr.Code, http.StatusAccepted, rr.Body.String())
	}

	select {
	case body := <-finalBodyCh:
		if !strings.Contains(body, finalSummary) {
			t.Fatalf("final body missing summary %q:\n%s", finalSummary, body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for dispatcher to finish")
	}

	entry := getTaskByNumber(t, store, 77)
	if entry.Status != taskstore.StatusCompleted {
		t.Fatalf("store entry status = %s, want %s", entry.Status, taskstore.StatusCompleted)
	}
}

func TestPRReviewWebhookE2EWithDispatcher(t *testing.T) {
	secret := "test-secret"
	trigger := "/code"

	remote, initialHash := setupRemoteWithPR(t)
	store := taskstore.NewStore()
	auth := &fakeAuth{owner: "cexll"}

	finalSummary := "在 feature/workflow 分支更新 README 内容并同步进度。"
	finalBodyCh := make(chan string, 1)

	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 3030, nil
	}
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		if strings.Contains(body, finalSummary) &&
			strings.Contains(body, "`feature/workflow`") &&
			strings.Contains(body, "SWE Agent finished @cexll's task") {
			select {
			case finalBodyCh <- body:
			default:
			}
		}
		return nil
	}
	mockGH.AddLabelFunc = func(repo string, number int, label, token string) error {
		return nil
	}

	provider := &fakeProvider{
		name: "codex",
		response: &claude.CodeResponse{
			Files: []claude.FileChange{
				{
					Path:    "README.md",
					Content: "# swe-agent\n\nUpdated via PR reviewer flow.\n",
				},
			},
			Summary: finalSummary,
			CostUSD: 0.4321,
		},
	}

	exec := executor.NewWithClient(provider, auth, mockGH)
	exec.WithStore(store)
	exec.WithCloneFunc(func(repo, branch, token string) (string, func(), error) {
		parent := t.TempDir()
		workdir := filepath.Join(parent, "workspace")
		runGit(t, "", "git", "clone", remote, workdir)
		runGit(t, workdir, "git", "checkout", branch)
		cleanup := func() {
			os.RemoveAll(parent)
		}
		return workdir, cleanup, nil
	})

	disp := dispatcher.New(exec, dispatcher.Config{
		Workers:        1,
		QueueSize:      2,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     20 * time.Millisecond,
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		disp.Shutdown(ctx)
	})

	handler := webhookpkg.NewHandler(secret, trigger, disp, store, auth)

	event := &webhookpkg.PullRequestReviewCommentEvent{
		Action: "created",
		Comment: webhookpkg.ReviewComment{
			ID:   5501,
			Body: "/code 同步 README 更新",
			User: webhookpkg.User{Login: "cexll", Type: "User"},
		},
		PullRequest: webhookpkg.PullRequest{
			Number: 99,
			Title:  "feat: workflow enhancements",
			Body:   "确保文档同步最新工作流。",
			State:  "open",
		},
		Repository: webhookpkg.Repository{
			FullName:      "cexll/swe-agent",
			DefaultBranch: "main",
			Owner:         webhookpkg.User{Login: "cexll"},
			Name:          "swe-agent",
		},
		Sender: webhookpkg.User{Login: "cexll"},
	}
	event.PullRequest.Base.Ref = "main"
	event.PullRequest.Head.Ref = "feature/workflow"

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal review event: %v", err)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", signature)
	req.Header.Set("X-GitHub-Event", "pull_request_review_comment")

	rr := httptest.NewRecorder()
	handler.Handle(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (body: %s)", rr.Code, http.StatusAccepted, rr.Body.String())
	}

	select {
	case body := <-finalBodyCh:
		if !strings.Contains(body, finalSummary) {
			t.Fatalf("final body missing summary %q:\n%s", finalSummary, body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for PR review dispatcher to finish")
	}

	entry := getTaskByNumber(t, store, 99)
	if entry.Status != taskstore.StatusCompleted {
		t.Fatalf("store entry status = %s, want %s", entry.Status, taskstore.StatusCompleted)
	}

	newHash := runGit(t, "", "git", "--git-dir", remote, "rev-parse", "feature/workflow")
	if newHash == initialHash {
		t.Fatalf("expected feature/workflow to advance, hash still %s", newHash)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	if len(args) == 0 {
		t.Fatalf("runGit requires arguments")
	}
	cmd := exec.Command(args[0], args[1:]...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func setupRemoteWithPR(t *testing.T) (string, string) {
	t.Helper()

	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	runGit(t, "", "git", "init", "--bare", remote)

	seed := filepath.Join(root, "seed")
	runGit(t, "", "git", "clone", remote, seed)
	runGit(t, seed, "git", "config", "user.name", "Seed User")
	runGit(t, seed, "git", "config", "user.email", "seed@example.com")
	runGit(t, seed, "git", "checkout", "-b", "main")

	readme := filepath.Join(seed, "README.md")
	if err := os.WriteFile(readme, []byte("# swe-agent\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, seed, "git", "add", ".")
	runGit(t, seed, "git", "commit", "-m", "seed main")
	runGit(t, seed, "git", "push", "origin", "main")

	runGit(t, seed, "git", "checkout", "-b", "feature/workflow")
	featureFile := filepath.Join(seed, "FEATURE.md")
	if err := os.WriteFile(featureFile, []byte("feature branch\n"), 0o644); err != nil {
		t.Fatalf("write FEATURE.md: %v", err)
	}
	runGit(t, seed, "git", "add", ".")
	runGit(t, seed, "git", "commit", "-m", "seed feature branch")
	runGit(t, seed, "git", "push", "-u", "origin", "feature/workflow")

	initialHash := runGit(t, seed, "git", "rev-parse", "feature/workflow")
	return remote, initialHash
}

func getTaskByNumber(t *testing.T, store *taskstore.Store, number int) *taskstore.Task {
	t.Helper()
	for _, task := range store.List() {
		if task.IssueNumber == number {
			return task
		}
	}
	t.Fatalf("task with number %d not found in store (tasks=%d)", number, len(store.List()))
	return nil
}
