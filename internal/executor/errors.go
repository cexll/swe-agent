package executor

import "errors"

// NonRetryableError marks task failures that should not be retried by the dispatcher.
type NonRetryableError struct {
	msg string
}

func (e *NonRetryableError) Error() string {
	return e.msg
}

// IsNonRetryable reports whether the provided error originated from a non-retryable failure.
func IsNonRetryable(err error) bool {
	if err == nil {
		return false
	}

	var target *NonRetryableError
	return errors.As(err, &target)
}
