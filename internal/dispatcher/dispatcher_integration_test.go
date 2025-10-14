package dispatcher

import (
	"context"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cexll/swe/internal/executor"
	"github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/provider/claude"
	"github.com/cexll/swe/internal/taskstore"
	"github.com/cexll/swe/internal/webhook"
)

type stubProvider struct {
	calls  atomic.Int32
	notify chan struct{}
}

func (p *stubProvider) GenerateCode(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error) {
	p.calls.Add(1)
	if p.notify != nil {
		select {
		case p.notify <- struct{}{}:
		default:
		}
	}
	return &claude.CodeResponse{
		Files: []claude.FileChange{
			{Path: "main.go", Content: "package main\n\nfunc main() {}\n"},
		},
		Summary: "add entrypoint",
		CostUSD: 0.02,
	}, nil
}

func (p *stubProvider) Name() string { return "stub" }

type stubAuth struct{}

func (s *stubAuth) GetInstallationToken(repo string) (*github.InstallationToken, error) {
	return &github.InstallationToken{
		Token:     "stub-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}, nil
}

func (s *stubAuth) GetInstallationOwner(repo string) (string, error) {
	return "installer", nil
}

func TestDispatcherExecutorProviderIntegration(t *testing.T) {
	store := taskstore.NewStore()
	provider := &stubProvider{
		notify: make(chan struct{}, 1),
	}
	auth := &stubAuth{}

	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		return 2024, nil
	}
	mockGH.CreatePRFunc = func(workdir, repo, head, base, title, body string) (string, error) {
		return "https://github.com/" + repo + "/pull/1", nil
	}

	execInst := executor.NewWithClient(provider, auth, mockGH)
	execInst = execInst.WithStore(store)
	execInst = execInst.WithCloneFunc(func(repo, branch, token string) (string, func(), error) {
		dir := t.TempDir()
		return dir, func() {}, nil
	})

	restoreCmd := executor.StubExecCommandForTest(func(name string, args ...string) *exec.Cmd {
		return exec.Command("bash", "-lc", "true")
	})
	defer restoreCmd()

	d := New(execInst, Config{
		Workers:           1,
		QueueSize:         1,
		MaxAttempts:       1,
		InitialBackoff:    5 * time.Millisecond,
		BackoffMultiplier: 2,
		MaxBackoff:        5 * time.Millisecond,
	})
	defer d.Shutdown(context.Background())

	task := &webhook.Task{
		ID:         "integration-task",
		Repo:       "owner/repo",
		Number:     7,
		Branch:     "main",
		Prompt:     "Add main",
		IssueTitle: "Add entrypoint",
		Username:   "installer",
	}

	store.Create(&taskstore.Task{
		ID:     task.ID,
		Status: taskstore.StatusPending,
	})

	if err := d.Enqueue(task); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	select {
	case <-provider.notify:
	case <-time.After(2 * time.Second):
		t.Fatal("provider was not invoked")
	}

	deadline := time.After(2 * time.Second)
	for {
		stored, ok := store.Get("integration-task")
		if ok && stored.Status == taskstore.StatusCompleted {
			break
		}
		select {
		case <-deadline:
			t.Fatal("task store did not reach completed status")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if provider.calls.Load() == 0 {
		t.Fatal("provider was not invoked")
	}
	if len(mockGH.CreateCommentCalls) == 0 {
		t.Fatal("expected CreateComment to be called")
	}
	if len(mockGH.UpdateCommentCalls) == 0 {
		t.Fatal("expected UpdateComment to be called")
	}
}
