package osu

import (
	"sync"
	"time"
)

type cacheEntry struct {
	value     []byte
	expiresAt time.Time
	storedAt  time.Time
}

type responseCache struct {
	mu         sync.Mutex
	entries    map[string]cacheEntry
	ttl        time.Duration
	maxEntries int
}

func newResponseCache(ttl time.Duration, maxEntries int) *responseCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if maxEntries <= 0 {
		maxEntries = 256
	}
	return &responseCache{
		entries:    make(map[string]cacheEntry),
		ttl:        ttl,
		maxEntries: maxEntries,
	}
}

func (c *responseCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return append([]byte(nil), entry.value...), true
}

func (c *responseCache) set(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for existingKey, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, existingKey)
		}
	}
	if _, replacing := c.entries[key]; !replacing && len(c.entries) >= c.maxEntries {
		oldestKey := ""
		var oldest time.Time
		for existingKey, entry := range c.entries {
			if oldestKey == "" || entry.storedAt.Before(oldest) {
				oldestKey = existingKey
				oldest = entry.storedAt
			}
		}
		delete(c.entries, oldestKey)
	}
	c.entries[key] = cacheEntry{
		value:     append([]byte(nil), value...),
		expiresAt: now.Add(c.ttl),
		storedAt:  now,
	}
}
