package data

import (
	"context"

	gh "github.com/cexll/swe/internal/github"
)

// Fetcher is a thin wrapper providing a stable entrypoint for executors.
type Fetcher struct {
	client *Client
}

// NewFetcher constructs a new Fetcher using the given GraphQL client.
func NewFetcher(c *Client) *Fetcher { return &Fetcher{client: c} }

// Fetch collects GitHub data for the provided webhook context.
func (f *Fetcher) Fetch(ctx context.Context, gctx *gh.Context) (*FetchResult, error) {
	repo := gctx.GetRepositoryFullName()
	number := gctx.GetIssueNumber()
	if gctx.IsPRContext() && gctx.GetPRNumber() != 0 {
		number = gctx.GetPRNumber()
	}
	params := FetchParams{
		Client:          f.client,
		Repository:      repo,
		Number:          number,
		IsPR:            gctx.IsPRContext(),
		TriggerUsername: gctx.GetTriggerUser(),
		// TriggerTime left empty; filtering is best-effort and optional here
	}
	return FetchGitHubData(ctx, params)
}
