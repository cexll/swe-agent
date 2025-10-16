package image

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestDownloader_DownloadAndCache(t *testing.T) {
	// Serve a tiny PNG payload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{0x89, 0x50, 0x4E, 0x47})
	}))
	defer srv.Close()

	d, err := NewDownloader(t.TempDir())
	if err != nil {
		t.Fatalf("NewDownloader: %v", err)
	}

	path1, err := d.Download(context.Background(), srv.URL+"/x.png")
	if err != nil {
		t.Fatalf("Download error: %v", err)
	}
	if _, err := os.Stat(path1); err != nil {
		t.Fatalf("downloaded file missing: %v", err)
	}

	// Second call should hit cache and return same path without server call necessarily
	path2, err := d.Download(context.Background(), srv.URL+"/x.png")
	if err != nil {
		t.Fatalf("cached Download error: %v", err)
	}
	if path1 != path2 {
		t.Fatalf("cache path mismatch: %s vs %s", path1, path2)
	}
}

func TestDownloader_DownloadImages_Multiple(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a.jpg", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("jpg")) })
	mux.HandleFunc("/b.png", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("png")) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	d, err := NewDownloader(t.TempDir())
	if err != nil {
		t.Fatalf("NewDownloader: %v", err)
	}

	urls := []string{srv.URL + "/a.jpg", srv.URL + "/b.png", srv.URL + "/404.gif"}
	m, err := d.DownloadImages(context.Background(), urls)
	if err != nil {
		t.Fatalf("DownloadImages error: %v", err)
	}
	if len(m) != 2 {
		t.Fatalf("expected 2 successes, got %d: %v", len(m), m)
	}
	for u, p := range m {
		if p == "" {
			t.Fatalf("empty path for %s", u)
		}
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("file missing for %s: %v", u, err)
		}
	}
	_ = fmt.Sprintf("") // silence unused fmt in some tooling
}
