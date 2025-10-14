package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cexll/swe/internal/executor"
	"github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/provider/codex"
	"github.com/cexll/swe/internal/taskstore"
	webhookpkg "github.com/cexll/swe/internal/webhook"
)

type fakeAuth struct{}

func (a *fakeAuth) GetInstallationToken(repo string) (*github.InstallationToken, error) {
	return &github.InstallationToken{
		Token:     "fake-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}, nil
}

func (a *fakeAuth) GetInstallationOwner(repo string) (string, error) {
	return "tester", nil
}

type inlineDispatcher struct {
	store    *taskstore.Store
	exec     *executor.Executor
	lastTask string
}

func (d *inlineDispatcher) Enqueue(task *webhookpkg.Task) error {
	d.lastTask = task.ID
	go func() {
		if err := d.exec.Execute(context.Background(), task); err != nil {
			log.Printf("executor error: %v", err)
		}
	}()
	return nil
}

func initLocalRepo() (string, error) {
	dir, err := os.MkdirTemp("", "codex-repo-")
	if err != nil {
		return "", err
	}

	commands := [][]string{
		{"git", "init"},
		{"git", "config", "user.name", "SWE Agent Local"},
		{"git", "config", "user.email", "swe-agent-local@example.com"},
		{"git", "checkout", "-b", "main"},
	}

	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Local Repo\n\n这是一个用于测试 Codex webhook 流程的仓库。\n"), 0o644); err != nil {
		return "", err
	}

	mainGo := filepath.Join(dir, "main.go")
	if err := os.WriteFile(mainGo, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		return "", err
	}

	for _, args := range [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", "initial commit"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git %v failed: %v\n%s", args, err, string(output))
		}
	}

	return dir, nil
}

func main() {
	if _, err := exec.LookPath("codex"); err != nil {
		log.Fatalf("codex CLI not found: %v", err)
	}

	repoDir, err := initLocalRepo()
	if err != nil {
		log.Fatalf("failed to init local repo: %v", err)
	}
	log.Printf("Local repo ready: %s", repoDir)

	provider := codex.NewProvider(os.Getenv("OPENAI_API_KEY"), os.Getenv("OPENAI_BASE_URL"), "gpt-5-codex")

	store := taskstore.NewStore()
	auth := &fakeAuth{}

	mockGH := github.NewMockGHClient()
	mockGH.CreateCommentFunc = func(repo string, number int, body, token string) (int, error) {
		log.Printf("[MockGH] create comment repo=%s number=%d", repo, number)
		return 101, nil
	}
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		log.Printf("[MockGH] update comment %d:\n%s", commentID, body)
		return nil
	}
	mockGH.AddLabelFunc = func(repo string, number int, label, token string) error {
		log.Printf("[MockGH] add label %s", label)
		return nil
	}

	exec := executor.NewWithClient(provider, auth, mockGH)
	exec.WithStore(store)
	exec.WithCloneFunc(func(repo, branch, token string) (string, func(), error) {
		return repoDir, func() {}, nil
	})

	dispatcher := &inlineDispatcher{
		store: store,
		exec:  exec,
	}

	secret := "local-secret"
	handler := webhookpkg.NewHandler(secret, "/code", dispatcher, store, auth)

	event := &webhookpkg.IssueCommentEvent{
		Action: "created",
		Issue: webhookpkg.Issue{
			Number: 1,
			Title:  "Codex Flow Test",
			Body:   "请阅读 README 并给出总结。",
			State:  "open",
		},
		Comment: webhookpkg.Comment{
			ID:   2025,
			Body: "/code 请只做分析，不要修改任何文件或分支。可以运行只读命令（如 cat README.md）来查看内容。请阅读 README.md，给出两句中文总结，并确保最终回答只包含一个 <summary>标签包裹的内容。",
			User: webhookpkg.User{Login: "tester", Type: "User"},
		},
		Repository: webhookpkg.Repository{
			FullName:      "local/test",
			DefaultBranch: "main",
			Owner:         webhookpkg.User{Login: "tester"},
			Name:          "test",
		},
		Sender: webhookpkg.User{Login: "tester"},
	}

	payload, err := json.Marshal(event)
	if err != nil {
		log.Fatalf("marshal event: %v", err)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set("X-GitHub-Event", "issue_comment")
	req.Header.Set("X-Hub-Signature-256", signature)
	req.Header.Set("X-GitHub-Delivery", "local-test-delivery")
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.Handle(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	log.Printf("Webhook handler HTTP status: %s", resp.Status)
	log.Printf("Waiting for Codex execution to finish...")

	waitUntil := time.Now().Add(2 * time.Minute)
	for time.Now().Before(waitUntil) {
		if dispatcher.lastTask != "" {
			if task, ok := store.Get(dispatcher.lastTask); ok {
				if task.Status == taskstore.StatusCompleted || task.Status == taskstore.StatusFailed {
					log.Printf("Final task status: %s", task.Status)
					for _, entry := range task.Logs {
						log.Printf("[TaskLog] %s %s", strings.ToUpper(entry.Level), entry.Message)
					}
					return
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("Timeout waiting for task completion")
}
