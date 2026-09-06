package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestPublicCacheSharesWorkAndCancellationIsIndependent(t *testing.T) {
	var cache publicResultCache[int]
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	load := func(ctx context.Context) (int, error) {
		calls.Add(1)
		close(started)
		select {
		case <-release:
			return 42, nil
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { _, err := cache.get(ctx, "public", time.Minute, load); first <- err }()
	<-started
	second := make(chan int, 1)
	go func() {
		value, err := cache.get(context.Background(), "public", time.Minute, load)
		if err != nil {
			second <- -1
		} else {
			second <- value
		}
	}()
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(release)
	if value := <-second; value != 42 {
		t.Fatal(value)
	}
	if value, err := cache.get(context.Background(), "public", time.Minute, load); value != 42 || err != nil || calls.Load() != 1 {
		t.Fatal("warm request repeated load")
	}
}

func TestPublicCacheExpiresAndNeverCachesFailures(t *testing.T) {
	var cache publicResultCache[int]
	ctx := context.Background()
	fail := errors.New("upstream unavailable")
	if _, err := cache.get(ctx, "key", time.Minute, func(context.Context) (int, error) { return 0, fail }); !errors.Is(err, fail) {
		t.Fatal(err)
	}
	value, err := cache.get(ctx, "key", time.Minute, func(context.Context) (int, error) { return 7, nil })
	if err != nil || value != 7 {
		t.Fatal("failure was cached")
	}
	cache.mu.Lock()
	cache.entries["key"] = publicCacheEntry[int]{7, time.Now().Add(-time.Second)}
	cache.mu.Unlock()
	value, err = cache.get(ctx, "key", time.Minute, func(context.Context) (int, error) { return 8, nil })
	if err != nil || value != 8 {
		t.Fatal("expired result was reused")
	}
}

func TestCatalogShowsRecentSnapshotDuringRefresh(t *testing.T) {
	var cache publicResultCache[int]
	cache.entries = map[string]publicCacheEntry[int]{"catalog": {7, time.Now().Add(-time.Second)}}
	started, release := make(chan struct{}), make(chan struct{})
	load := func(context.Context) (int, error) { close(started); <-release; return 8, nil }
	value, err := cache.get(context.Background(), "catalog", time.Minute, load, 4*time.Minute)
	if err != nil || value != 7 {
		t.Fatal("catalog waited instead of showing recent result")
	}
	<-started
	close(release)
	value, err = cache.get(context.Background(), "catalog", time.Minute, load)
	if err != nil || value != 8 {
		t.Fatal("refreshed catalog not published")
	}
}
