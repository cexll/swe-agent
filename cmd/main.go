package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/cexll/swe/internal/config"
	"github.com/cexll/swe/internal/executor"
	"github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/provider"
	"github.com/cexll/swe/internal/webhook"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file (ignore error if file doesn't exist)
	_ = godotenv.Load()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Starting Pilot SWE server...")
	log.Printf("Port: %d", cfg.Port)
	log.Printf("Trigger keyword: %s", cfg.TriggerKeyword)
	log.Printf("Provider: %s", cfg.Provider)
	log.Printf("GitHub App ID: %s", cfg.GitHubAppID)

	// Initialize GitHub App authentication
	appAuth := &github.AppAuth{
		AppID:      cfg.GitHubAppID,
		PrivateKey: cfg.GitHubPrivateKey,
	}

	// Initialize AI provider based on configuration
	var aiProvider provider.Provider

	switch cfg.Provider {
	case "claude":
		log.Printf("Claude model: %s", cfg.ClaudeModel)
		aiProvider, err = provider.NewProvider(&provider.Config{
			Name:         "claude",
			ClaudeAPIKey: cfg.ClaudeAPIKey,
			ClaudeModel:  cfg.ClaudeModel,
		})
	case "codex":
		log.Printf("Codex model: %s", cfg.CodexModel)
		if cfg.OpenAIBaseURL != "" {
			log.Printf("Using custom OpenAI Base URL: %s", cfg.OpenAIBaseURL)
		}
		aiProvider, err = provider.NewProvider(&provider.Config{
			Name:          "codex",
			OpenAIAPIKey:  cfg.OpenAIAPIKey,
			OpenAIBaseURL: cfg.OpenAIBaseURL,
			CodexModel:    cfg.CodexModel,
		})
	default:
		log.Fatalf("Unsupported provider: %s", cfg.Provider)
	}

	if err != nil {
		log.Fatalf("Failed to initialize AI provider: %v", err)
	}
	log.Printf("AI Provider: %s", aiProvider.Name())

	// Initialize executor
	exec := executor.New(aiProvider, appAuth)

	// Initialize webhook handler
	handler := webhook.NewHandler(cfg.GitHubWebhookSecret, cfg.TriggerKeyword, exec)

	// Setup router
	r := mux.NewRouter()

	// Webhook endpoints
	r.HandleFunc("/webhook/issue_comment", handler.HandleIssueComment).Methods("POST")
	r.HandleFunc("/webhook/review_comment", handler.HandleReviewComment).Methods("POST")
	
	// Legacy endpoint - keep for backward compatibility
	r.HandleFunc("/webhook", handler.HandleIssueComment).Methods("POST")

	// Health check endpoint
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Root endpoint with info
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"service":"pilot-swe","status":"running","trigger":"%s","endpoints":["/webhook/issue_comment","/webhook/review_comment"]}`, cfg.TriggerKeyword)
	}).Methods("GET")

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Server listening on %s", addr)
	log.Printf("Issue comment webhook: http://localhost%s/webhook/issue_comment", addr)
	log.Printf("Review comment webhook: http://localhost%s/webhook/review_comment", addr)
	log.Printf("Health check: http://localhost%s/health", addr)

	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}