package executor

import (
	"os/exec"
	"reflect"
	"testing"
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
