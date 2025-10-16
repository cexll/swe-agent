package webhook

import (
	"os"
	"testing"

	gh "github.com/google/go-github/v66/github"

	ghinternal "github.com/cexll/swe/internal/github"
	ghtesting "github.com/cexll/swe/internal/github/testing"
)

// TestMain sets up a mock GitHub client for all webhook tests to avoid real API calls.
func TestMain(m *testing.M) {
	client, cleanup := ghtesting.NewMockGitHubClient()
	// Inject factory to always return the mock client
	ghinternal.SetGitHubClientFactory(func(token string) *gh.Client { return client })
	code := m.Run()
	cleanup()
	os.Exit(code)
}
