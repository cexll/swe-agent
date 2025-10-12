package github

import (
	"log"
	"strings"
	"time"
)

const (
	// Default retry configuration for GitHub operations
	defaultMaxRetries   = 10
	defaultInitialDelay = 1 * time.Second
)

// retryWithBackoff executes a function with exponential backoff retry
// This eliminates the special case of transient network failures by converting them
// into automatically recoverable normal cases.
func retryWithBackoff(fn func() error) error {
	return retryWithBackoffCustom(defaultMaxRetries, defaultInitialDelay, fn)
}

// retryWithBackoffCustom allows custom retry configuration
func retryWithBackoffCustom(maxRetries int, initialDelay time.Duration, fn func() error) error {
	var lastErr error
	delay := initialDelay

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Sleep before retry (skip on first attempt)
		if attempt > 0 {
			log.Printf("[Retry] Attempt %d/%d after %v delay", attempt+1, maxRetries+1, delay)
			time.Sleep(delay)
			delay *= 2 // Exponential backoff: 1s -> 2s -> 4s
		}

		// Execute the function
		lastErr = fn()
		if lastErr == nil {
			if attempt > 0 {
				log.Printf("[Retry] Succeeded on attempt %d/%d", attempt+1, maxRetries+1)
			}
			return nil // Success
		}

		// Check if error is retryable
		if !isRetryableError(lastErr) {
			log.Printf("[Retry] Non-retryable error, failing immediately: %v", lastErr)
			return lastErr // Don't retry permanent errors
		}

		if attempt < maxRetries {
			log.Printf("[Retry] Retryable error on attempt %d/%d: %v", attempt+1, maxRetries+1, lastErr)
		}
	}

	log.Printf("[Retry] All %d attempts failed, giving up", maxRetries+1)
	return lastErr
}

// isRetryableError determines if an error should trigger a retry
// Returns true for transient network errors, false for permanent errors
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Common transient errors that should be retried:
	// - EOF: connection closed unexpectedly
	// - timeout: request took too long
	// - connection refused: service temporarily unavailable
	// - temporary failure: DNS or network issues
	// - rate limit: GitHub API throttling (less common with App auth)
	retryablePatterns := []string{
		"eof",
		"timeout",
		"connection refused",
		"temporary failure",
		"connection reset",
		"broken pipe",
		"no such host",
		"network is unreachable",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}
