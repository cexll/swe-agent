package executor

import (
    "strings"
    "testing"
    "github.com/cexll/swe/internal/webhook"
)

func TestDeriveHelpfulHints_AuthErrors(t *testing.T) {
    hints := deriveHelpfulHints("API error: 401 Unauthorized", &webhook.Task{})
    if len(hints) == 0 {
        t.Fatalf("expected hints for auth error")
    }
    found := false
    for _, h := range hints {
        if strings.Contains(h, "credentials") || strings.Contains(h, "Contents: Read & Write") {
            found = true
        }
    }
    if !found {
        t.Fatalf("expected credential/install hints, got %v", hints)
    }
}

func TestDeriveHelpfulHints_PermissionDenied(t *testing.T) {
    msg := "remote: Permission to owner/repo denied to user\nHTTP 403"
    hints := deriveHelpfulHints(msg, &webhook.Task{})
    if len(hints) == 0 {
        t.Fatalf("expected hints for permission denied")
    }
    ok := false
    for _, h := range hints {
        if strings.Contains(h, "Push permission denied") {
            ok = true
        }
    }
    if !ok {
        t.Fatalf("expected push permission hint, got %v", hints)
    }
}

func TestDeriveHelpfulHints_TransientNetwork(t *testing.T) {
    hints := deriveHelpfulHints("write tcp: connection reset by peer", &webhook.Task{})
    if len(hints) == 0 {
        t.Fatalf("expected hints for transient network error")
    }
    ok := false
    for _, h := range hints {
        if strings.Contains(h, "Transient network issue") {
            ok = true
        }
    }
    if !ok {
        t.Fatalf("expected transient network hint, got %v", hints)
    }
}

func TestDeriveHelpfulHints_Generic(t *testing.T) {
    hints := deriveHelpfulHints("some random failure", &webhook.Task{})
    if len(hints) == 0 {
        t.Fatalf("expected generic hint when no pattern matches")
    }
}

// no-op helper removed; using strings.Contains directly
