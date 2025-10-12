package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/stellarlink/pilot-swe/internal/config"
	"github.com/stellarlink/pilot-swe/internal/executor"
	"github.com/stellarlink/pilot-swe/internal/store"
	"github.com/stellarlink/pilot-swe/internal/web"
	"github.com/stellarlink/pilot-swe/internal/webhook"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize task store
	taskStore := store.NewTaskStore()

	// Initialize executor
	exec, err := executor.NewExecutor(cfg, taskStore)
	if err != nil {
		log.Fatalf("Failed to create executor: %v", err)
	}

	// Initialize webhook handler
	webhookHandler := webhook.NewHandler(cfg, exec)

	// Initialize web handler
	webHandler, err := web.NewHandler(taskStore)
	if err != nil {
		log.Fatalf("Failed to create web handler: %v", err)
	}

	// Setup router
	r := mux.NewRouter()

	// Register routes
	r.HandleFunc("/webhook", webhookHandler.Handle).Methods("POST")
	webHandler.RegisterRoutes(r)

	// Create server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting server on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}