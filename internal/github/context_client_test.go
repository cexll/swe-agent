package github

import "testing"

func TestNewGitHubClient(t *testing.T) {
	c := &Context{}
	cli := c.NewGitHubClient()
	if cli == nil {
		t.Fatalf("nil client")
	}
}
