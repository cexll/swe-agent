package webhook

import "errors"

var (
	// ErrQueueFull indicates the dispatcher cannot accept new tasks right now.
	ErrQueueFull = errors.New("task queue is full")
	// ErrQueueClosed indicates the dispatcher has been shut down.
	ErrQueueClosed = errors.New("task queue is closed")
)
