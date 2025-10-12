package github

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error should not retry",
			err:      nil,
			expected: false,
		},
		{
			name:     "EOF error should retry",
			err:      errors.New("Post \"https://api.github.com/graphql\": EOF"),
			expected: true,
		},
		{
			name:     "timeout error should retry",
			err:      errors.New("request timeout after 30s"),
			expected: true,
		},
		{
			name:     "connection refused should retry",
			err:      errors.New("dial tcp: connection refused"),
			expected: true,
		},
		{
			name:     "connection reset should retry",
			err:      errors.New("read tcp: connection reset by peer"),
			expected: true,
		},
		{
			name:     "broken pipe should retry",
			err:      errors.New("write tcp: broken pipe"),
			expected: true,
		},
		{
			name:     "temporary failure should retry",
			err:      errors.New("temporary failure in name resolution"),
			expected: true,
		},
		{
			name:     "network unreachable should retry",
			err:      errors.New("network is unreachable"),
			expected: true,
		},
		{
			name:     "no such host should retry",
			err:      errors.New("dial tcp: lookup api.github.com: no such host"),
			expected: true,
		},
		{
			name:     "authentication error should not retry",
			err:      errors.New("HTTP 401: Bad credentials"),
			expected: false,
		},
		{
			name:     "not found error should not retry",
			err:      errors.New("HTTP 404: Not Found"),
			expected: false,
		},
		{
			name:     "permission denied should not retry",
			err:      errors.New("permission denied"),
			expected: false,
		},
		{
			name:     "case insensitive EOF",
			err:      errors.New("connection closed: eof"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("isRetryableError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestRetryWithBackoffCustom_Success(t *testing.T) {
	attempts := 0
	err := retryWithBackoffCustom(3, 10*time.Millisecond, func() error {
		attempts++
		return nil // Success on first attempt
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

func TestRetryWithBackoffCustom_SuccessAfterRetries(t *testing.T) {
	attempts := 0
	err := retryWithBackoffCustom(3, 10*time.Millisecond, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("EOF") // Retryable error
		}
		return nil // Success on 3rd attempt
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetryWithBackoffCustom_NonRetryableError(t *testing.T) {
	attempts := 0
	expectedErr := errors.New("HTTP 401: Bad credentials")

	err := retryWithBackoffCustom(3, 10*time.Millisecond, func() error {
		attempts++
		return expectedErr // Non-retryable error
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("Expected 401 error, got %v", err)
	}

	if attempts != 1 {
		t.Errorf("Expected 1 attempt (no retry), got %d", attempts)
	}
}

func TestRetryWithBackoffCustom_ExhaustedRetries(t *testing.T) {
	attempts := 0
	expectedErr := errors.New("EOF")

	err := retryWithBackoffCustom(2, 10*time.Millisecond, func() error {
		attempts++
		return expectedErr // Always fail with retryable error
	})

	if err == nil {
		t.Error("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "EOF") {
		t.Errorf("Expected EOF error, got %v", err)
	}

	// Should attempt 3 times total (initial + 2 retries)
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetryWithBackoffCustom_ExponentialBackoff(t *testing.T) {
	attempts := 0
	startTime := time.Now()

	err := retryWithBackoffCustom(2, 50*time.Millisecond, func() error {
		attempts++
		return errors.New("timeout") // Retryable error
	})

	duration := time.Since(startTime)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	// Should wait: 50ms + 100ms = 150ms minimum
	// Allow some tolerance for execution time
	if duration < 150*time.Millisecond {
		t.Errorf("Expected at least 150ms delay, got %v", duration)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetryWithBackoff_DefaultConfiguration(t *testing.T) {
	attempts := 0
	err := retryWithBackoff(func() error {
		attempts++
		if attempts < 2 {
			return errors.New("EOF")
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}
