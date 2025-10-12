package webhook

import (
	"sync"
	"time"
)

type commentDeduper struct {
	mu      sync.Mutex
	entries map[int64]time.Time
	ttl     time.Duration
}

func newCommentDeduper(ttl time.Duration) *commentDeduper {
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &commentDeduper{
		entries: make(map[int64]time.Time),
		ttl:     ttl,
	}
}

// markIfNew returns true if the comment ID has not been seen recently.
// When it returns true, the ID is recorded with an expiry timestamp.
func (d *commentDeduper) markIfNew(id int64) bool {
	now := time.Now()

	d.mu.Lock()
	defer d.mu.Unlock()

	// Remove expired entries
	for key, expiry := range d.entries {
		if now.After(expiry) {
			delete(d.entries, key)
		}
	}

	if expiry, ok := d.entries[id]; ok && now.Before(expiry) {
		return false
	}

	d.entries[id] = now.Add(d.ttl)
	return true
}
