package osu

import (
	"context"
	"sync"
	"time"
)

type intervalLimiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func newIntervalLimiter(requestsPerSecond float64) *intervalLimiter {
	if requestsPerSecond <= 0 {
		requestsPerSecond = 4
	}
	return &intervalLimiter{interval: time.Duration(float64(time.Second) / requestsPerSecond)}
}

func (l *intervalLimiter) wait(ctx context.Context) error {
	l.mu.Lock()
	now := time.Now()
	start := now
	if l.next.After(start) {
		start = l.next
	}
	l.next = start.Add(l.interval)
	l.mu.Unlock()

	delay := time.Until(start)
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
