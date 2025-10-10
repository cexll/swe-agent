package provider

import (
	"context"

	"github.com/cexll/swe/internal/provider/claude"
)

// Provider is the interface that all AI providers must implement
type Provider interface {
	// GenerateCode generates code changes based on the request
	GenerateCode(ctx context.Context, req *claude.CodeRequest) (*claude.CodeResponse, error)

	// Name returns the provider name
	Name() string
}
