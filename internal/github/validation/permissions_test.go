package validation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	gh "github.com/google/go-github/v66/github"
)

// mockTransport redirects api.github.com traffic to our test server.
type mockTransport struct {
	base *url.URL
	c    *http.Client
}

func (mt mockTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.URL.Scheme, r.URL.Host = mt.base.Scheme, mt.base.Host
	r.Host = mt.base.Host
	return mt.c.Transport.RoundTrip(r)
}


func TestPermissions_Checks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/collaborators/u/permission", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"permission": "write"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	base, _ := url.Parse(srv.URL)
	old := http.DefaultTransport
	http.DefaultTransport = mockTransport{base: base, c: srv.Client()}
	defer func() { http.DefaultTransport = old }()

	client := gh.NewClient(nil)
	ok, err := CheckWritePermission(context.Background(), client, "o", "r", "u")
	if err != nil || !ok {
		t.Fatalf("CheckWritePermission write perm => %v, %v", ok, err)
	}

	// admin considered write as well
	mux.HandleFunc("/repos/o/r/collaborators/admin/permission", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"permission": "admin"})
	})
	ok, err = CheckWritePermission(context.Background(), client, "o", "r", "admin")
	if err != nil || !ok {
		t.Fatalf("CheckWritePermission admin perm => %v, %v", ok, err)
	}

	// read should be false
	mux.HandleFunc("/repos/o/r/collaborators/ro/permission", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"permission": "read"})
	})
	ok, err = CheckWritePermission(context.Background(), client, "o", "r", "ro")
	if err != nil || ok {
		t.Fatalf("CheckWritePermission read perm => %v, %v", ok, err)
	}

	// CheckAdminPermission
	isAdmin, err := CheckAdminPermission(context.Background(), client, "o", "r", "admin")
	if err != nil || !isAdmin {
		t.Fatalf("CheckAdminPermission => %v, %v", isAdmin, err)
	}
	isAdmin, err = CheckAdminPermission(context.Background(), client, "o", "r", "u")
	if err != nil || isAdmin {
		t.Fatalf("CheckAdminPermission non-admin => %v, %v", isAdmin, err)
	}
}

func TestEnsureWritePermission_Errors(t *testing.T) {
	mux := http.NewServeMux()
	// simulate API error
	mux.HandleFunc("/repos/o/r/collaborators/bad/permission", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	// simulate read-only
	mux.HandleFunc("/repos/o/r/collaborators/ro/permission", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"permission": "read"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	base, _ := url.Parse(srv.URL)
	old := http.DefaultTransport
	http.DefaultTransport = mockTransport{base: base, c: srv.Client()}
	defer func() { http.DefaultTransport = old }()

	client := gh.NewClient(nil)
	if err := EnsureWritePermission(context.Background(), client, "o", "r", "bad"); err == nil {
		t.Fatalf("expected API error")
	}
	if err := EnsureWritePermission(context.Background(), client, "o", "r", "ro"); err == nil {
		t.Fatalf("expected lack of write error")
	}
}
