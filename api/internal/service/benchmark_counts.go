package service

import (
	"context"
	"sync"
	"time"

	hubv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1"
)

// Rank verification is optional enrichment, never a dependency of catalog reads.
// Snapshots are immutable after publication and retained when a refresh fails.
type benchmarkCountSnapshot struct {
	mu          sync.Mutex
	counts      map[uint32]uint32
	refreshing  bool
	nextRefresh time.Time
}

func (c *benchmarkCountSnapshot) snapshotAndRefresh(ctx context.Context, load func(context.Context) ([]*hubv1.BenchmarkListItem, error)) map[uint32]uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.refreshing && !time.Now().Before(c.nextRefresh) {
		c.refreshing = true
		go func() {
			refreshCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Minute)
			defer cancel()
			items, err := load(refreshCtx)
			if err == nil {
				err = refreshCtx.Err()
			}
			c.mu.Lock()
			defer c.mu.Unlock()
			c.refreshing = false
			c.nextRefresh = time.Now().Add(time.Minute)
			if err != nil {
				return
			}
			counts := make(map[uint32]uint32, len(items))
			for _, item := range items {
				counts[item.BenchmarkId] = item.PlayerCount
			}
			c.counts = counts
			c.nextRefresh = time.Now().Add(15 * time.Minute)
		}()
	}
	return c.counts
}
