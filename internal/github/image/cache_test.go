package image

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheOperations(t *testing.T) {
	d, err := NewDownloader(t.TempDir())
	if err != nil {
		t.Fatalf("NewDownloader: %v", err)
	}

	// Create two files with different mtimes
	f1 := filepath.Join(d.cacheDir, "a.png")
	f2 := filepath.Join(d.cacheDir, "b.jpg")
	if err := os.WriteFile(f1, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("bb"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Backdate f1 to make it old
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(f1, old, old); err != nil {
		t.Fatal(err)
	}

	size, err := d.GetCacheSize()
	if err != nil || size == 0 {
		t.Fatalf("GetCacheSize error or zero: size=%d err=%v", size, err)
	}

	if err := d.CleanupCache(24 * time.Hour); err != nil {
		t.Fatalf("CleanupCache: %v", err)
	}
	if _, err := os.Stat(f1); !os.IsNotExist(err) {
		t.Fatalf("expected old file removed, stat err=%v", err)
	}

	if err := d.ClearCache(); err != nil {
		t.Fatalf("ClearCache: %v", err)
	}
	entries, _ := os.ReadDir(d.cacheDir)
	if len(entries) != 0 {
		t.Fatalf("expected empty cache dir, got %d entries", len(entries))
	}
}
