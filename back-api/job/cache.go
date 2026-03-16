package job

import (
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Cache provides thread-safe in-memory storage for job status lookups.
// Used by both in-memory and Redis queue implementations to avoid
// duplicating cache logic.
type Cache struct {
	mu      sync.RWMutex
	jobs    map[uuid.UUID]*Job
	maxSize int // 0 means unlimited
}

// NewCache creates a new job cache with the given initial capacity.
// Use WithMaxSize to set a hard limit on cache entries.
func NewCache(capacity int) *Cache {
	return &Cache{jobs: make(map[uuid.UUID]*Job, capacity)}
}

// WithMaxSize sets the maximum number of entries in the cache.
// When exceeded, finished jobs are evicted oldest-first.
func (c *Cache) WithMaxSize(n int) *Cache {
	c.maxSize = n
	return c
}

// Put stores or updates a job in the cache.
func (c *Cache) Put(j *Job) {
	c.mu.Lock()
	c.jobs[j.ID] = j
	if c.maxSize > 0 && len(c.jobs) > c.maxSize {
		c.evictFinishedLocked()
	}
	c.mu.Unlock()
}

// Get returns a snapshot of the job by ID.
// The copy prevents callers from mutating cached state.
func (c *Cache) Get(id uuid.UUID) (*Job, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	j, ok := c.jobs[id]
	if !ok {
		return nil, false
	}
	return j.snapshot(), true
}

// Delete removes a job from the cache.
func (c *Cache) Delete(id uuid.UUID) {
	c.mu.Lock()
	delete(c.jobs, id)
	c.mu.Unlock()
}

// CleanupOlderThan removes completed/failed jobs older than maxAge.
func (c *Cache) CleanupOlderThan(maxAge time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for id, j := range c.jobs {
		if j.Finished != nil && j.Finished.Before(cutoff) {
			delete(c.jobs, id)
			removed++
		}
	}
	if removed > 0 {
		slog.Debug("cleaned up finished jobs from cache", "removed", removed, "remaining", len(c.jobs))
	}
}

// evictFinishedLocked removes the oldest finished jobs to bring size below maxSize.
// Must be called with c.mu held.
func (c *Cache) evictFinishedLocked() {
	for len(c.jobs) > c.maxSize {
		var oldestID uuid.UUID
		var oldestTime time.Time
		found := false
		for id, j := range c.jobs {
			if j.Finished != nil && (!found || j.Finished.Before(oldestTime)) {
				oldestID = id
				oldestTime = *j.Finished
				found = true
			}
		}
		if !found {
			break // no finished jobs to evict
		}
		delete(c.jobs, oldestID)
	}
}
