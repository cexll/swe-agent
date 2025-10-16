package webhook

import (
	"testing"
	"time"
)

func TestCommentDeduperLifecycle(t *testing.T) {
	d := newCommentDeduper(10 * time.Millisecond)

	if !d.markIfNew(1) {
		t.Fatal("first markIfNew should return true")
	}
	if d.markIfNew(1) {
		t.Fatal("second markIfNew should return false before expiry")
	}

	time.Sleep(15 * time.Millisecond)

	if !d.markIfNew(1) {
		t.Fatal("markIfNew should return true after expiry")
	}
}

func TestCommentDeduperDefaultTTL(t *testing.T) {
	d := newCommentDeduper(0)
	if d.ttl != time.Hour {
		t.Fatalf("default TTL = %s, want 1h", d.ttl)
	}
}
