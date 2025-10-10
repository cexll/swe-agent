package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/cexll/swe/internal/config"
	"github.com/cexll/swe/internal/executor"
	"github.com/cexll/swe/internal/provider"
	"github.com/cexll/swe/internal/webhook"
	"github.com/gorilla/mux"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Starting Pilot SWE server...")
	log.Printf("Port: %d", cfg.Port)
	log.Printf("Trigger keyword: %s", cfg.TriggerKeyword)
	log.Printf("Claude model: %s", cfg.ClaudeModel)

	// Initialize AI provider (currently Claude, easy to extend)
	aiProvider, err := provider.NewProvider(&provider.Config{
		Name:         "claude",
		ClaudeAPIKey: cfg.ClaudeAPIKey,
		ClaudeModel:  cfg.ClaudeModel,
	})
	if err != nil {
		log.Fatalf("Failed to initialize AI provider: %v", err)
	}
	log.Printf("AI Provider: %s", aiProvider.Name())

	// Initialize executor
	exec := executor.New(aiProvider)

	// Initialize webhook handler
	handler := webhook.NewHandler(cfg.GitHubWebhookSecret, cfg.TriggerKeyword, exec)

	// Setup router
	r := mux.NewRouter()

	// Webhook endpoint
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
		fmt.Fprintf(w, `{"service":"pilot-swe","status":"running","trigger":"%s"}`, cfg.TriggerKeyword)
	}).Methods("GET")

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Server listening on %s", addr)
	log.Printf("Webhook endpoint: http://localhost%s/webhook", addr)
	log.Printf("Health check: http://localhost%s/health", addr)

	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
