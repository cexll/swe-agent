package main

import (
	"log"
	"net/http"
	"os"

	"github.com/cexll/swe/internal/concurrency"
	"github.com/cexll/swe/internal/config"
	"github.com/cexll/swe/internal/executor"
	"github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/provider"
	"github.com/cexll/swe/internal/webhook"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create provider
	p, err := provider.NewProvider(cfg)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	log.Printf("Using provider: %s", p.Name())

	// Create GitHub App auth
	appAuth, err := github.NewAppAuth(cfg.GitHubAppID, cfg.GitHubPrivateKey)
	if err != nil {
		log.Fatalf("Failed to create GitHub App auth: %v", err)
	}

	// Create concurrency manager
	lockMgr := concurrency.NewManager()
	log.Printf("Concurrency control enabled")

	// Create executor with lock manager
	exec := executor.New(p, appAuth, lockMgr)

	// Create webhook handler
	handler := webhook.NewHandler(cfg.GitHubWebhookSecret, cfg.TriggerKeyword, exec)

	// Setup HTTP router
	r := mux.NewRouter()
	r.HandleFunc("/webhook/issue_comment", handler.HandleIssueComment).Methods("POST")
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Starting server on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}