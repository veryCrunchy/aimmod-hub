package osu

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type intervalLimiter struct {
	mu           sync.Mutex
	interval     time.Duration
	next         time.Time
	blockedUntil time.Time
	waiters      []*limiterWaiter
}

type limiterWaiter struct {
	ready  chan struct{}
	search bool
}

func newIntervalLimiter(requestsPerSecond float64) *intervalLimiter {
	if requestsPerSecond <= 0 {
		requestsPerSecond = 1
	}
	return &intervalLimiter{interval: time.Duration(float64(time.Second) / requestsPerSecond)}
}

func (l *intervalLimiter) wait(ctx context.Context) error {
	return l.waitFor(ctx, false)
}

func (l *intervalLimiter) waitFor(ctx context.Context, search bool) error {
	// Only the head of the queue may claim the next slot. Racing every caller's
	// timer lets newer traffic repeatedly overtake a waiting interactive search.
	if err := ctx.Err(); err != nil {
		return err
	}
	waiter := &limiterWaiter{ready: make(chan struct{}), search: search}
	l.mu.Lock()
	l.waiters = append(l.waiters, waiter)
	if len(l.waiters) == 1 {
		close(waiter.ready)
	}
	l.mu.Unlock()
	defer l.removeWaiter(waiter)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-waiter.ready:
	}
	// Reserve only actual starts. Cancellation removes a waiter without leaving
	// a reservation that would delay subsequent requests.
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		l.mu.Lock()
		now := time.Now()
		if now.Before(l.blockedUntil) {
			l.mu.Unlock()
			return &upstreamHTTPError{StatusCode: http.StatusTooManyRequests}
		}
		delay := l.next.Sub(now)
		if delay <= 0 {
			l.next = now.Add(l.interval)
			l.mu.Unlock()
			return nil
		}
		l.mu.Unlock()
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}
}

func (l *intervalLimiter) removeWaiter(finished *limiterWaiter) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, waiter := range l.waiters {
		if waiter != finished {
			continue
		}
		l.waiters = append(l.waiters[:i], l.waiters[i+1:]...)
		if i == 0 && len(l.waiters) > 0 {
			// Alternate search and other traffic when both are queued. A browsing
			// request must not wait behind a full batch of score enrichment, and
			// searches must not starve profiles, scores or background indexing.
			for j, next := range l.waiters {
				if next.search != finished.search {
					copy(l.waiters[1:j+1], l.waiters[:j])
					l.waiters[0] = next
					break
				}
			}
			close(l.waiters[0].ready)
		}
		return
	}
}

func (l *intervalLimiter) backoff(retryAfter string) {
	delay := time.Minute
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds > 0 && seconds <= 86400 {
		delay = time.Duration(seconds) * time.Second
	} else if date, err := http.ParseTime(retryAfter); err == nil && time.Until(date) > 0 {
		delay = min(time.Until(date), 24*time.Hour)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if until := time.Now().Add(delay); until.After(l.blockedUntil) {
		l.blockedUntil = until
	}
}
