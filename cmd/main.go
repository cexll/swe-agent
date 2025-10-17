package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/cexll/swe/internal/config"
	"github.com/cexll/swe/internal/dispatcher"
	"github.com/cexll/swe/internal/executor"
	"github.com/cexll/swe/internal/github"
	_ "github.com/cexll/swe/internal/modes/command" // Register CommandMode
	"github.com/cexll/swe/internal/taskstore"
	"github.com/cexll/swe/internal/web"
	"github.com/cexll/swe/internal/webhook"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

var (
	loadDotEnv         = godotenv.Load
	newTaskStore       = taskstore.NewStore
	newDispatcher      = dispatcher.New
	newWebHandler      = web.NewHandler
	defaultListenServe = http.ListenAndServe
)

func main() {
	if err := run(context.Background(), defaultListenServe); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func run(ctx context.Context, serve func(string, http.Handler) error) error {
	// Load .env file (ignore error if file doesn't exist)
	_ = loadDotEnv()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	log.Printf("Starting SWE-Agent server...")
	log.Printf("Port: %d", cfg.Port)
	log.Printf("Trigger keyword: %s", cfg.TriggerKeyword)
	log.Printf("Provider: %s", cfg.Provider)
	log.Printf("GitHub App ID: %s", cfg.GitHubAppID)
	log.Printf("Dispatcher workers: %d, queue size: %d, max attempts: %d", cfg.DispatcherWorkers, cfg.DispatcherQueueSize, cfg.DispatcherMaxAttempts)

	// Initialize in-memory task store for UI
	taskStore := newTaskStore()

	// Initialize GitHub App authentication
	appAuth := &github.AppAuth{
		AppID:      cfg.GitHubAppID,
		PrivateKey: cfg.GitHubPrivateKey,
	}

	// Initialize AI provider based on configuration
	aiProvider, err := cfg.NewProvider()
	if err != nil {
		return fmt.Errorf("failed to initialize AI provider: %w", err)
	}
	log.Printf("AI Provider: %s", aiProvider.Name())

	// Initialize executor
	exec := executor.New(aiProvider, appAuth)
	// Wrap the new executor with an adapter to satisfy dispatcher.TaskExecutor
	adapted := executor.NewAdapter(exec)

	// Initialize dispatcher (task queue with retries)
	dispatcherConfig := dispatcher.Config{
		Workers:           cfg.DispatcherWorkers,
		QueueSize:         cfg.DispatcherQueueSize,
		MaxAttempts:       cfg.DispatcherMaxAttempts,
		InitialBackoff:    cfg.DispatcherRetryInitial,
		BackoffMultiplier: cfg.DispatcherBackoffMultiplier,
		MaxBackoff:        cfg.DispatcherRetryMax,
	}
	taskDispatcher := newDispatcher(adapted, dispatcherConfig)
	defer taskDispatcher.Shutdown(ctx)

	// Initialize webhook handler
	handler := webhook.NewHandler(cfg.GitHubWebhookSecret, cfg.TriggerKeyword, taskDispatcher, taskStore, appAuth)

	// Initialize web UI handler
	webHandler, err := newWebHandler(taskStore)
	if err != nil {
		return fmt.Errorf("failed to initialize web handler: %w", err)
	}

	// Setup router
	r := mux.NewRouter()

	// Webhook endpoint
	r.HandleFunc("/webhook", handler.Handle).Methods("POST")

	// Task UI endpoints
	r.HandleFunc("/tasks", webHandler.ListTasks).Methods("GET")
	r.HandleFunc("/tasks/{id}", webHandler.TaskDetail).Methods("GET")

	// Health check endpoint
	r.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}).Methods("GET")

	// Root endpoint with info
	r.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "OK")
	}).Methods("GET")

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Server listening on %s", addr)
	log.Printf("Webhook endpoint: http://localhost%s/webhook", addr)
	log.Printf("Health check: http://localhost%s/health", addr)
	log.Printf("Tasks UI: http://localhost%s/tasks", addr)

	if err := serve(addr, r); err != nil {
		return fmt.Errorf("server failed to start: %w", err)
	}

	return nil
}
