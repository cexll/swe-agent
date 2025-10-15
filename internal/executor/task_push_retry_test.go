package executor

import (
    "os/exec"
    "testing"
)

func TestCommitAndPush_RetryTransientPush_SucceedsOnThirdAttempt(t *testing.T) {
    var pushCalls int

    restore := StubExecCommandForTest(func(name string, args ...string) *exec.Cmd {
        if name == "git" {
            if len(args) > 0 && args[0] == "push" {
                pushCalls++
                if pushCalls < 3 {
                    // Simulate transient failure with stderr marker included in error string
                    return exec.Command("bash", "-lc", "echo 'eof' 1>&2; exit 1")
                }
                return exec.Command("bash", "-lc", "true")
            }
            // Succeed for all other git commands (config, add, commit, etc.)
            return exec.Command("bash", "-lc", "true")
        }
        return exec.Command(name, args...)
    })
    defer restore()

    exec := New(nil, nil)
    // Empty repo/token to skip pushurl configuration complexity in test
    if err := exec.commitAndPush(t.TempDir(), "", "feature/test", "msg", false, ""); err != nil {
        t.Fatalf("commitAndPush() error = %v, want success after retries", err)
    }
    if pushCalls != 3 {
        t.Fatalf("git push attempts = %d, want 3", pushCalls)
    }
}

func TestCommitAndPush_NonRetryablePush_FailsImmediately(t *testing.T) {
    var pushCalls int

    restore := StubExecCommandForTest(func(name string, args ...string) *exec.Cmd {
        if name == "git" {
            if len(args) > 0 && args[0] == "push" {
                pushCalls++
                // Simulate non-retryable permission error (403)
                return exec.Command("bash", "-lc", "echo 'HTTP 403 Forbidden' 1>&2; exit 1")
            }
            return exec.Command("bash", "-lc", "true")
        }
        return exec.Command(name, args...)
    })
    defer restore()

    exec := New(nil, nil)
    err := exec.commitAndPush(t.TempDir(), "", "feature/test", "msg", false, "")
    if err == nil {
        t.Fatalf("commitAndPush() error = nil, want error for non-retryable failure")
    }
    if pushCalls != 1 {
        t.Fatalf("git push attempts = %d, want 1", pushCalls)
    }
}

