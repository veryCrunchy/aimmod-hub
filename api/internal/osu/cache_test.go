package osu

import (
	"context"
	"testing"
	"time"
)

func TestResponseCacheCopiesExpiresAndEvicts(t *testing.T) {
	cache := newResponseCache(time.Minute, 2)
	input := []byte("first")
	cache.set("one", input)
	input[0] = 'X'

	got, ok := cache.get("one")
	if !ok || string(got) != "first" {
		t.Fatalf("cached value = %q, %v; want first, true", got, ok)
	}
	got[0] = 'Y'
	gotAgain, _ := cache.get("one")
	if string(gotAgain) != "first" {
		t.Fatalf("cache returned shared bytes: %q", gotAgain)
	}

	cache.set("two", []byte("second"))
	cache.mu.Lock()
	entry := cache.entries["one"]
	entry.storedAt = time.Now().Add(-time.Hour)
	cache.entries["one"] = entry
	cache.mu.Unlock()
	cache.set("three", []byte("third"))
	if _, ok := cache.get("one"); ok {
		t.Fatal("oldest entry was not evicted")
	}

	cache.mu.Lock()
	entry = cache.entries["two"]
	entry.expiresAt = time.Now().Add(-time.Second)
	cache.entries["two"] = entry
	cache.mu.Unlock()
	if _, ok := cache.get("two"); ok {
		t.Fatal("expired entry was returned")
	}
}

func TestIntervalLimiterHonorsContext(t *testing.T) {
	limiter := newIntervalLimiter(1)
	if err := limiter.wait(context.Background()); err != nil {
		t.Fatalf("first wait: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := limiter.wait(ctx); err == nil {
		t.Fatal("second wait unexpectedly ignored context deadline")
	}
}
