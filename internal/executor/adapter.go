package executor

import (
	"context"

	"github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/webhook"
)

// ExecutorAdapter adapts the simplified Executor to the dispatcher.TaskExecutor interface.
// It converts webhook.Task into github.Context and forwards execution.
type ExecutorAdapter struct {
	inner *Executor
}

// NewAdapter creates a new adapter for the given Executor.
func NewAdapter(inner *Executor) *ExecutorAdapter {
	return &ExecutorAdapter{inner: inner}
}

// Execute implements dispatcher.TaskExecutor by translating a webhook.Task into
// a github.Context using the raw webhook payload and event type.
func (a *ExecutorAdapter) Execute(ctx context.Context, task *webhook.Task) error {
	// Parse original webhook into the normalized github.Context
	ghCtx, err := github.ParseWebhookEvent(task.EventType, task.RawPayload)
	if err != nil {
		return err
	}

	// Delegate to the real executor
	return a.inner.Execute(ctx, ghCtx)
}
