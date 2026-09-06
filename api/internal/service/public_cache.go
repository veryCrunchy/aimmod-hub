package service

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// Only public, read-only results belong here. Callers must not mutate values.
// Bounded TTL and capacity keep player data fresh and arbitrary keys bounded.
type publicResultCache[T any] struct {
	mu      sync.Mutex
	entries map[string]publicCacheEntry[T]
	flight  singleflight.Group
}
type publicCacheEntry[T any] struct {
	value   T
	expires time.Time
}

func (c *publicResultCache[T]) get(ctx context.Context, key string, ttl time.Duration, load func(context.Context) (T, error), maxStale ...time.Duration) (T, error) {
	lookup := func() (T, bool) {
		c.mu.Lock()
		defer c.mu.Unlock()
		entry, ok := c.entries[key]
		return entry.value, ok && time.Now().Before(entry.expires)
	}
	if value, ok := lookup(); ok {
		return value, nil
	}
	// Catalogs may show a recent successful snapshot while one shared refresh
	// runs. Live score endpoints omit this grace period.
	c.mu.Lock()
	stale, exists := c.entries[key]
	c.mu.Unlock()
	canUseStale := exists && len(maxStale) > 0 && time.Now().Before(stale.expires.Add(maxStale[0]))
	result := c.flight.DoChan(key, func() (any, error) {
		if value, ok := lookup(); ok {
			return value, nil
		}
		// One visitor leaving must not cancel work shared with other visitors.
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		value, err := load(loadCtx)
		if err == nil {
			err = loadCtx.Err()
		}
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		if c.entries == nil {
			c.entries = make(map[string]publicCacheEntry[T])
		}
		if len(c.entries) >= 64 { // Evict the oldest expiry, not an unbounded key set.
			oldestKey := ""
			var oldest time.Time
			for k, entry := range c.entries {
				if oldest.IsZero() || entry.expires.Before(oldest) {
					oldestKey, oldest = k, entry.expires
				}
			}
			delete(c.entries, oldestKey)
		}
		c.entries[key] = publicCacheEntry[T]{value, time.Now().Add(ttl)}
		c.mu.Unlock()
		return value, nil
	})
	if canUseStale {
		return stale.value, nil
	}
	select {
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	case result := <-result:
		if result.Err != nil {
			var zero T
			return zero, result.Err
		}
		return result.Val.(T), nil
	}
}
