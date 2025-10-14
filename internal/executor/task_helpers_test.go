package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/provider/claude"
	"github.com/cexll/swe/internal/webhook"
)

func TestStubExecCommandForTest(t *testing.T) {
	var calls int
	restore := StubExecCommandForTest(func(name string, args ...string) *exec.Cmd {
		calls++
		return exec.Command("bash", "-lc", "true")
	})

	cmd := execCommand("git", "status")
	if cmd == nil {
		t.Fatal("stubbed execCommand should return command")
	}
	if calls != 1 {
		t.Fatalf("expected stub to be invoked once, got %d", calls)
	}

	restore()

	cmd2 := execCommand("echo", "ok")
	if cmd2 == nil {
		t.Fatal("restored execCommand should return command")
	}
	if calls != 1 {
		t.Fatalf("restore should not invoke stub, got %d calls", calls)
	}
}

func TestExecutorWithCloneFunc(t *testing.T) {
	exec := New(nil, nil)
	originalPtr := reflect.ValueOf(exec.cloneFn).Pointer()

	customCalls := 0
	custom := func(repo, branch, token string) (string, func(), error) {
		customCalls++
		return "/tmp/workdir", func() {}, nil
	}

	exec.WithCloneFunc(custom)
	if reflect.ValueOf(exec.cloneFn).Pointer() == originalPtr {
		t.Fatal("cloneFn pointer did not change after WithCloneFunc")
	}

	workdir, cleanup, err := exec.cloneFn("owner/repo", "main", "token")
	if err != nil {
		t.Fatalf("custom cloneFn returned error: %v", err)
	}
	if workdir != "/tmp/workdir" {
		t.Fatalf("workdir = %s, want /tmp/workdir", workdir)
	}
	if cleanup == nil {
		t.Fatal("cleanup should not be nil for custom clone")
	}
	if customCalls != 1 {
		t.Fatalf("custom cloneFn calls = %d, want 1", customCalls)
	}

	exec.WithCloneFunc(nil)
	if reflect.ValueOf(exec.cloneFn).Pointer() != originalPtr {
		t.Fatal("cloneFn pointer should reset to original after WithCloneFunc(nil)")
	}
}

func TestExecutorEnsureAttempt(t *testing.T) {
	exec := New(nil, nil)
	task := &webhook.Task{}

	if attempt := exec.ensureAttempt(task); attempt != 1 {
		t.Fatalf("ensureAttempt() = %d, want 1", attempt)
	}

	task.Attempt = 3
	if attempt := exec.ensureAttempt(task); attempt != 3 {
		t.Fatalf("ensureAttempt() preserved attempt = %d, want 3", attempt)
	}
}

func TestExecutorBuildExecutionContext(t *testing.T) {
	exec := New(nil, nil)
	exec.disallowedTools = "shell"

	task := &webhook.Task{
		Repo:       "owner/repo",
		Branch:     "main",
		Number:     42,
		IssueTitle: "Bug",
		IssueBody:  "Fix it",
		Username:   "tester",
		PromptContext: map[string]string{
			"repository": "custom/repo",
		},
	}

	ctx := exec.buildExecutionContext(task)

	if ctx["issue_title"] != "Bug" {
		t.Fatalf("issue_title = %q, want %q", ctx["issue_title"], "Bug")
	}
	if ctx["disallowed_tools"] != "shell" {
		t.Fatalf("disallowed_tools = %q, want %q", ctx["disallowed_tools"], "shell")
	}
	if ctx["is_pr"] != "false" {
		t.Fatalf("is_pr = %q, want false", ctx["is_pr"])
	}
	if ctx["issue_number"] != "42" {
		t.Fatalf("issue_number = %q, want 42", ctx["issue_number"])
	}
	if ctx["trigger_username"] != "tester" || ctx["trigger_display_name"] != "tester" {
		t.Fatalf("trigger user fields incorrect: %+v", ctx)
	}
}

func TestExecutorPrepareChangePlan_ResponseOnly(t *testing.T) {
	restore := StubExecCommandForTest(func(name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 && args[0] == "status" {
			return exec.Command("bash", "-lc", "printf ''")
		}
		return exec.Command(name, args...)
	})
	defer restore()

	mockGH := github.NewMockGHClient()
	mockGH.UpdateCommentFunc = func(repo string, commentID int, body, token string) error {
		return nil
	}

	exec := NewWithClient(nil, nil, mockGH)
	tracker := github.NewCommentTrackerWithClient("owner/repo", 1, "tester", mockGH)
	tracker.CommentID = 99

	task := &webhook.Task{Repo: "owner/repo", Number: 1}
	result := &claude.CodeResponse{Summary: "Analysis complete"}

	plan, files, handled, err := exec.prepareChangePlan(task, t.TempDir(), result, tracker, "token")
	if err != nil {
		t.Fatalf("prepareChangePlan() unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("prepareChangePlan() handled = false, want true for response-only path")
	}
	if plan != nil {
		t.Fatalf("plan = %#v, want nil when no code changes", plan)
	}
	if len(files) != 0 {
		t.Fatalf("changed files = %d, want 0", len(files))
	}
	if tracker.State.Status != github.StatusCompleted {
		t.Fatalf("tracker status = %v, want %v", tracker.State.Status, github.StatusCompleted)
	}
	if len(mockGH.UpdateCommentCalls) == 0 {
		t.Fatal("expected tracker.Update to be invoked at least once")
	}
}

func TestExecutorPrepareChangePlan_WithChangesTriggersPlan(t *testing.T) {
	paths := []string{
		"docs/readme.md",
		"docs/guide.md",
		"internal/api/service.go",
		"internal/api/service_test.go",
		"internal/core/model.go",
		"internal/core/model_test.go",
		"pkg/core/handler.go",
		"pkg/core/processor.go",
		"pkg/core/validator.go",
	}

	gitOutput := buildGitStatusOutput(paths)

	restore := StubExecCommandForTest(func(name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 && args[0] == "status" {
			return exec.Command("bash", "-lc", fmt.Sprintf("printf '%s'", escapeSingleQuotes(gitOutput)))
		}
		return exec.Command(name, args...)
	})
	defer restore()

	workdir := t.TempDir()
	for _, path := range paths {
		full := filepath.Join(workdir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("failed to create dir for %s: %v", path, err)
		}
		content := "// auto-generated test content\n"
		if strings.HasSuffix(path, ".md") {
			content = "# documentation\n"
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", path, err)
		}
	}

	mockGH := github.NewMockGHClient()
	exec := NewWithClient(nil, nil, mockGH)
	tracker := github.NewCommentTrackerWithClient("owner/repo", 1, "tester", mockGH)

	task := &webhook.Task{Repo: "owner/repo", Number: 1, Prompt: "Update project"}
	result := &claude.CodeResponse{Summary: "Implemented changes"}

	plan, files, handled, err := exec.prepareChangePlan(task, workdir, result, tracker, "token")
	if err != nil {
		t.Fatalf("prepareChangePlan() unexpected error: %v", err)
	}
	if handled {
		t.Fatal("prepareChangePlan() handled = true, want false when changes present")
	}
	if plan == nil {
		t.Fatal("plan is nil, want non-nil for detected changes")
	}
	if len(files) != len(paths) {
		t.Fatalf("changed files = %d, want %d", len(files), len(paths))
	}
	if len(plan.SubPRs) <= 1 {
		t.Fatalf("plan.SubPRs = %d, want more than 1 to ensure split workflow", len(plan.SubPRs))
	}
	if len(mockGH.UpdateCommentCalls) != 0 {
		t.Fatalf("expected no comment updates during plan preparation, got %d", len(mockGH.UpdateCommentCalls))
	}
}

func buildGitStatusOutput(paths []string) string {
	var lines []string
	for _, path := range paths {
		lines = append(lines, fmt.Sprintf("?? %s", path))
	}
	return strings.Join(lines, "\n") + "\n"
}

func escapeSingleQuotes(input string) string {
	return strings.ReplaceAll(input, "'", `'"'"'`)
}
