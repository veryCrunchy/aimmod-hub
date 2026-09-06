package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	hubv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1"
)

func TestBenchmarkCountsDoNotBlockCatalogAndShareRefresh(t *testing.T) {
	var cache benchmarkCountSnapshot
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	load := func(ctx context.Context) ([]*hubv1.BenchmarkListItem, error) {
		calls.Add(1)
		close(started)
		<-release
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return []*hubv1.BenchmarkListItem{{BenchmarkId: 7, PlayerCount: 3}}, nil
	}
	if counts := cache.snapshotAndRefresh(ctx, load); len(counts) != 0 {
		t.Fatal("unexpected initial counts")
	}
	<-started
	for i := 0; i < 100; i++ {
		cache.snapshotAndRefresh(ctx, load)
	}
	if calls.Load() != 1 {
		t.Fatal("duplicate background refresh")
	}
	close(release)
	waitForCountRefresh(t, &cache)
	if got := cache.snapshotAndRefresh(ctx, load)[7]; got != 3 {
		t.Fatalf("count not published: %d", got)
	}
}

func TestBenchmarkCountsKeepGoodSnapshotAfterFailure(t *testing.T) {
	cache := benchmarkCountSnapshot{counts: map[uint32]uint32{7: 3}}
	before := cache.snapshotAndRefresh(context.Background(), func(context.Context) ([]*hubv1.BenchmarkListItem, error) {
		return []*hubv1.BenchmarkListItem{{BenchmarkId: 7, PlayerCount: 1}}, errors.New("provider unavailable")
	})
	waitForCountRefresh(t, &cache)
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if before[7] != 3 || cache.counts[7] != 3 {
		t.Fatal("failed refresh replaced good counts")
	}
	if !cache.nextRefresh.After(time.Now()) {
		t.Fatal("missing failure backoff")
	}
}

func waitForCountRefresh(t *testing.T, c *benchmarkCountSnapshot) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		running := c.refreshing
		c.mu.Unlock()
		if !running {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("background refresh did not finish")
}
