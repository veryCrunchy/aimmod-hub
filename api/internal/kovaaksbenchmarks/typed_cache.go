package kovaaksbenchmarks

import "time"

const typedCacheEntries = 256

// pruneTypedCache removes expired values and reserves space for the next write.
// Callers hold the client mutex. The oldest expiry is evicted at capacity.
func pruneTypedCache[T any](entries map[string]T, expiry func(T) time.Time) {
	now := time.Now()
	oldestKey := ""
	var oldest time.Time
	for key, value := range entries {
		at := expiry(value)
		if !at.After(now) {
			delete(entries, key)
			continue
		}
		if oldest.IsZero() || at.Before(oldest) {
			oldest, oldestKey = at, key
		}
	}
	if len(entries) >= typedCacheEntries {
		delete(entries, oldestKey)
	}
}
