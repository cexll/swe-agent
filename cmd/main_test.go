package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cexll/swe/internal/provider"
	"github.com/cexll/swe/internal/taskstore"
	"github.com/cexll/swe/internal/web"
)

func setRequiredEnv(t *testing.T, provider string) {
	t.Helper()
	t.Setenv("GITHUB_APP_ID", "1234")
	t.Setenv("GITHUB_PRIVATE_KEY", "test-private-key")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("PROVIDER", provider)
	t.Setenv("DISPATCHER_WORKERS", "1")
	t.Setenv("DISPATCHER_QUEUE_SIZE", "1")
	t.Setenv("OPENAI_BASE_URL", "")
}

func chdirToRepoRoot(t *testing.T) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	root := filepath.Dir(cwd)
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir(%s) failed: %v", root, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore cwd failed: %v", err)
		}
	})
}

func TestRun_StartsServerWithValidConfig(t *testing.T) {
	setRequiredEnv(t, "codex")
	t.Setenv("PORT", "4321")
	chdirToRepoRoot(t)

	var servedAddr string
	var servedHandler http.Handler

	serve := func(addr string, handler http.Handler) error {
		servedAddr = addr
		servedHandler = handler
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := run(ctx, serve); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}

	if servedAddr != ":4321" {
		t.Fatalf("serve addr = %q, want :4321", servedAddr)
	}
	if servedHandler == nil {
		t.Fatalf("serve handler is nil")
	}

	// Smoke test a couple of routes to ensure router wiring is intact.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	servedHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/health status = %d, want 200", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	servedHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/ status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); body == "" || body == "{}" {
		t.Fatalf("root body = %q, want non-empty service payload", body)
	}
}

func TestRun_ReturnsErrorWhenServeFails(t *testing.T) {
	setRequiredEnv(t, "codex")
	chdirToRepoRoot(t)

	expected := errors.New("listen failed")
	err := run(context.Background(), func(string, http.Handler) error {
		return expected
	})

	if err == nil {
		t.Fatalf("run() error = nil, want %v", expected)
	}
	if !errors.Is(err, expected) {
		t.Fatalf("run() error = %v, want to wrap %v", err, expected)
	}
}

func TestRun_UnsupportedProvider(t *testing.T) {
	setRequiredEnv(t, "unknown")

	called := false
	err := run(context.Background(), func(string, http.Handler) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("run() error = nil, want unsupported provider error")
	}
	if !called {
		return
	}
	t.Fatalf("serve should not be called when configuration fails")
}

func TestRun_UsesClaudeProvider(t *testing.T) {
	setRequiredEnv(t, "claude")
	t.Setenv("ANTHROPIC_API_KEY", "test-claude-key")
	chdirToRepoRoot(t)

	// Note: Provider is now created via config.NewProvider(), not a global factory
	// This test verifies that the Claude provider configuration path works end-to-end

	var servedAddr string
	err := run(context.Background(), func(addr string, handler http.Handler) error {
		servedAddr = addr
		return nil
	})
	if err != nil {
		t.Fatalf("run() returned error: %v", err)
	}
	if servedAddr == "" {
		t.Fatal("serve addr should not be empty")
	}
}

func TestRun_WebHandlerError(t *testing.T) {
	setRequiredEnv(t, "codex")
	chdirToRepoRoot(t)

	prevWebHandler := newWebHandler
	defer func() { newWebHandler = prevWebHandler }()
	newWebHandler = func(store *taskstore.Store) (*web.Handler, error) {
		return nil, errors.New("inject failure")
	}

	err := run(context.Background(), func(string, http.Handler) error {
		t.Fatalf("serve should not be called on web handler failure")
		return nil
	})
	if err == nil {
		t.Fatal("run() error = nil, want web handler failure")
	}
	if !strings.Contains(err.Error(), "failed to initialize web handler") {
		t.Fatalf("error = %v, want web handler failure", err)
	}
}

type stubProvider struct {
	name string
}

func (s *stubProvider) GenerateCode(ctx context.Context, req *provider.CodeRequest) (*provider.CodeResponse, error) {
	return nil, errors.New("stub")
}

func (s *stubProvider) Name() string {
	return s.name
}
