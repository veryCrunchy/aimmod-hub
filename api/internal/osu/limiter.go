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
}

func newIntervalLimiter(requestsPerSecond float64) *intervalLimiter {
	if requestsPerSecond <= 0 {
		requestsPerSecond = 1
	}
	return &intervalLimiter{interval: time.Duration(float64(time.Second) / requestsPerSecond)}
}

func (l *intervalLimiter) wait(ctx context.Context) error {
	// Reserve only actual starts. Cancelled requests must not leave future slots
	// that accumulate and eventually block every caller, including token renewal.
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
