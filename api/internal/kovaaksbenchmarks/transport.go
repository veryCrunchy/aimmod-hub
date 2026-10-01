package kovaaksbenchmarks

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultAPIBaseURL   = "https://kovaaks.com/webapp-backend"
	defaultSteamBaseURL = "https://steamcommunity.com"

	// staleTTL is how long an expired response may still be served when the
	// provider is failing or rate limiting us. Public pages stay useful during
	// an outage instead of turning into errors.
	staleTTL = 6 * time.Hour

	// Outbound request budget. KovaaK's does not publish a limit, so stay
	// well below anything a browser session would produce.
	defaultRequestsPerSecond = 8
	defaultRequestBurst      = 16
)

// ErrRateLimited is returned when the provider asked us to back off and no
// cached response is available.
var ErrRateLimited = errors.New("kovaak's is rate limiting requests; try again shortly")

// envOr returns the trimmed environment value or the fallback.
func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return strings.TrimRight(value, "/")
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

// tokenBucket is a small, dependency-free limiter for outbound requests.
type tokenBucket struct {
	mu       sync.Mutex
	rate     float64
	burst    float64
	tokens   float64
	last     time.Time
	now      func() time.Time
	blocked  time.Time
	sleepFor func(context.Context, time.Duration) error
}

func newTokenBucket(perSecond, burst int) *tokenBucket {
	if perSecond <= 0 {
		perSecond = defaultRequestsPerSecond
	}
	if burst <= 0 {
		burst = perSecond
	}
	return &tokenBucket{
		rate:     float64(perSecond),
		burst:    float64(burst),
		tokens:   float64(burst),
		now:      time.Now,
		sleepFor: sleepContext,
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// wait blocks until a token is available, the context ends, or the provider
// has asked us to back off (in which case it fails fast).
func (b *tokenBucket) wait(ctx context.Context) error {
	for {
		b.mu.Lock()
		now := b.now()
		if now.Before(b.blocked) {
			b.mu.Unlock()
			return ErrRateLimited
		}
		if b.last.IsZero() {
			b.last = now
		}
		b.tokens += now.Sub(b.last).Seconds() * b.rate
		if b.tokens > b.burst {
			b.tokens = b.burst
		}
		b.last = now
		if b.tokens >= 1 {
			b.tokens--
			b.mu.Unlock()
			return nil
		}
		need := time.Duration((1 - b.tokens) / b.rate * float64(time.Second))
		b.mu.Unlock()
		if err := b.sleepFor(ctx, need); err != nil {
			return err
		}
	}
}

// backOff stops outbound requests until the given time.
func (b *tokenBucket) backOff(until time.Time) {
	b.mu.Lock()
	if until.After(b.blocked) {
		b.blocked = until
	}
	b.mu.Unlock()
}

// retryAfter reads a Retry-After header in seconds, defaulting to 30s and
// capping at 10 minutes.
func retryAfter(resp *http.Response) time.Duration {
	const fallback = 30 * time.Second
	if resp == nil {
		return fallback
	}
	value := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if value == "" {
		return fallback
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		d := time.Duration(seconds) * time.Second
		if d > 10*time.Minute {
			d = 10 * time.Minute
		}
		return d
	}
	if at, err := http.ParseTime(value); err == nil {
		if d := time.Until(at); d > 0 && d <= 10*time.Minute {
			return d
		}
	}
	return fallback
}

type cachedResponse struct {
	raw       []byte
	fetchedAt time.Time
}

// responseCache keeps raw provider responses so that a fresh entry skips the
// network and an expired one can still be served while the provider fails.
type responseCache struct {
	mu      sync.Mutex
	entries map[string]cachedResponse
	max     int
}

func newResponseCache(max int) *responseCache {
	return &responseCache{entries: map[string]cachedResponse{}, max: max}
}

func (c *responseCache) get(key string) (cachedResponse, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	return entry, ok
}

func (c *responseCache) put(key string, raw []byte, at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.max > 0 && len(c.entries) >= c.max {
		// Drop entries past the stale window first, then the oldest.
		var oldestKey string
		var oldest time.Time
		for k, v := range c.entries {
			if at.Sub(v.fetchedAt) > staleTTL {
				delete(c.entries, k)
				continue
			}
			if oldestKey == "" || v.fetchedAt.Before(oldest) {
				oldestKey, oldest = k, v.fetchedAt
			}
		}
		if len(c.entries) >= c.max && oldestKey != "" {
			delete(c.entries, oldestKey)
		}
	}
	c.entries[key] = cachedResponse{raw: raw, fetchedAt: at}
}
