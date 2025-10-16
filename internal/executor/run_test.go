package executor

import "testing"

func TestRun_SuccessAndFailure(t *testing.T) {
	if err := run("bash", "-lc", "echo ok"); err != nil {
		t.Fatalf("run echo: %v", err)
	}
	if err := run("bash", "-lc", "exit 1"); err == nil {
		t.Fatalf("expected failure")
	}
}
