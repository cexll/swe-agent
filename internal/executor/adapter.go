package executor

import (
	"context"

	"github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/webhook"
)

// Adapter adapts the simplified Executor to the dispatcher.TaskExecutor interface.
// It converts webhook.Task into github.Context and forwards execution.
type Adapter struct {
	inner *Executor
}

// NewAdapter creates a new adapter for the given Executor.
func NewAdapter(inner *Executor) *Adapter {
	return &Adapter{inner: inner}
}

// Execute implements dispatcher.TaskExecutor by translating a webhook.Task into
// a github.Context using the raw webhook payload and event type.
func (a *Adapter) Execute(ctx context.Context, task *webhook.Task) error {
	// Parse original webhook into the normalized github.Context
	ghCtx, err := github.ParseWebhookEvent(task.EventType, task.RawPayload)
	if err != nil {
		return err
	}

	// Inject prepared values from task when available (mode-based pipeline)
	if task.Branch != "" {
		ghCtx.PreparedBranch = task.Branch
	}
	if task.BaseBranch != "" {
		ghCtx.PreparedBaseBranch = task.BaseBranch
	}
	if task.Prompt != "" {
		ghCtx.PreparedPrompt = task.Prompt
	}
	if task.CommentID != 0 {
		ghCtx.PreparedCommentID = task.CommentID
	}

	// Delegate to the real executor
	return a.inner.Execute(ctx, ghCtx)
}
