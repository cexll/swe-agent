package executor

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsNonRetryable(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		if IsNonRetryable(nil) {
			t.Fatal("nil should not be marked as non-retryable")
		}
	})

	t.Run("generic error", func(t *testing.T) {
		if IsNonRetryable(errors.New("boom")) {
			t.Fatal("generic errors must not be treated as non-retryable")
		}
	})

	t.Run("direct non-retryable", func(t *testing.T) {
		err := &NonRetryableError{msg: "stop retrying"}
		if !IsNonRetryable(err) {
			t.Fatal("NonRetryableError should be detected")
		}
	})

	t.Run("wrapped non-retryable", func(t *testing.T) {
		err := &NonRetryableError{msg: "wrapped"}
		wrapped := fmt.Errorf("outer: %w", err)
		if !IsNonRetryable(wrapped) {
			t.Fatal("wrapped NonRetryableError should be detected")
		}
	})
}
