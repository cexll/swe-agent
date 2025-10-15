package data

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gh "github.com/cexll/swe/internal/github"
)

type fakeAuth struct{}

func (f fakeAuth) GetInstallationToken(repo string) (*gh.InstallationToken, error) {
	return &gh.InstallationToken{Token: "test-token", ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (f fakeAuth) GetInstallationOwner(repo string) (string, error) { return "owner", nil }

func TestNewClient(t *testing.T) {
	c := NewClient(fakeAuth{})
	if c.httpClient == nil {
		t.Fatal("httpClient should be initialized")
	}
	if c.endpoint == "" {
		t.Fatal("endpoint should be set")
	}
}

func TestClientDo_SuccessAndHeaders(t *testing.T) {
	// echo back minimal envelope
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST")
		}
		// verify headers
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("bad auth header: %q", got)
		}
		if r.Header.Get("Accept") == "" || r.Header.Get("Content-Type") == "" || r.Header.Get("X-GitHub-Api-Version") == "" {
			t.Fatalf("missing standard headers")
		}
		io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"ok": true}})
	}))
	defer ts.Close()

	c := NewClient(fakeAuth{})
	c.endpoint = ts.URL
	// use default http client
	var out struct {
		Ok bool `json:"ok"`
	}
	if err := c.Do(context.Background(), "o/r", "query {}", nil, &out); err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	if !out.Ok {
		t.Fatalf("unexpected decode: %+v", out)
	}
}

func TestClientDo_GraphQLErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": []map[string]string{{"message": "bad"}}})
	}))
	defer ts.Close()
	c := NewClient(fakeAuth{})
	c.endpoint = ts.URL
	if err := c.Do(context.Background(), "o/r", "query {}", nil, nil); err == nil {
		t.Fatalf("expected graphql error")
	}
}

func TestClientDo_HTTPErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("oops"))
	}))
	defer ts.Close()
	c := NewClient(fakeAuth{})
	c.endpoint = ts.URL
	if err := c.Do(context.Background(), "o/r", "q", nil, nil); err == nil {
		t.Fatalf("expected status error")
	}
}
