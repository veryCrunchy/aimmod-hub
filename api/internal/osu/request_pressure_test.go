package osu

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCancelledRequestsDoNotAccumulateLimiterDebt(t *testing.T) {
	limiter := newIntervalLimiter(1)
	if err := limiter.wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	initial := limiter.next
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for i := 0; i < 10000; i++ {
		if !errors.Is(limiter.wait(ctx), context.Canceled) {
			t.Fatal("cancel ignored")
		}
	}
	if !limiter.next.Equal(initial) {
		t.Fatal("cancelled requests reserved future slots")
	}
}

func TestCancelledWaitingRequestsDoNotReserveSlots(t *testing.T) {
	limiter := newIntervalLimiter(1)
	_ = limiter.wait(context.Background())
	initial := limiter.next
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = limiter.wait(ctx) }()
	}
	wg.Wait()
	if !limiter.next.Equal(initial) {
		t.Fatal("abandoned queue delayed future traffic")
	}
}

func TestUpstreamConcurrentMissesShareRequestAndCancellation(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()
	client, _ := newUpstreamClient(server.URL, server.Client(), newResponseCache(time.Minute, 32), newIntervalLimiter(100000), "test")
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { _, e := client.get(ctx, "/test", nil, ""); first <- e }()
	<-entered
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, e := client.get(context.Background(), "/test", nil, "")
			if e != nil || string(b) != `{"ok":true}` {
				t.Errorf("shared response: %q %v", b, e)
			}
		}()
	}
	cancel()
	if !errors.Is(<-first, context.Canceled) {
		t.Fatal("caller cancellation ignored")
	}
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("100 followers made %d upstream requests", calls.Load())
	}
}

func TestUpstreamQueueDeadlineAndCapacity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("request should not reach upstream") }))
	defer server.Close()
	limiter := newIntervalLimiter(1)
	_ = limiter.wait(context.Background())
	client, _ := newUpstreamClient(server.URL, &http.Client{Timeout: 25 * time.Millisecond}, newResponseCache(time.Minute, 32), limiter, "test")
	start := time.Now()
	_, err := client.get(context.Background(), "/waiting", nil, "")
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatalf("unbounded queue: %v", err)
	}
	for i := 0; i < cap(client.capacity); i++ {
		client.capacity <- struct{}{}
	}
	_, err = client.get(context.Background(), "/overflow", nil, "")
	if !isUpstreamHTTPStatus(err, 503) {
		t.Fatalf("capacity must fail fast: %v", err)
	}
}

func TestUpstreamRateLimitCooldownStillServesCachedResponses(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/cached" {
			fmt.Fprint(w, `{}`)
			return
		}
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(429)
	}))
	defer server.Close()
	limiter := newIntervalLimiter(100000)
	client, _ := newUpstreamClient(server.URL, server.Client(), newResponseCache(time.Minute, 32), limiter, "test")
	_, _ = client.get(context.Background(), "/cached", nil, "")
	_, err := client.get(context.Background(), "/limited", nil, "")
	if !isUpstreamHTTPStatus(err, 429) {
		t.Fatal(err)
	}
	if time.Until(limiter.blockedUntil) < 119*time.Second {
		t.Fatal("Retry-After ignored")
	}
	_, err = client.get(context.Background(), "/other", nil, "")
	if !isUpstreamHTTPStatus(err, 429) {
		t.Fatal(err)
	}
	if _, err = client.get(context.Background(), "/cached", nil, ""); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("cooldown leaked %d requests", calls.Load())
	}
}

func TestResponseCacheBoundsBytesAndAccountsForReplacementAndExpiry(t *testing.T) {
	cache := newResponseCache(time.Minute, 256)
	cache.maxBytes = 10
	cache.set("a", []byte("123456"))
	cache.set("b", []byte("123456"))
	if len(cache.entries) != 1 || cache.bytes != 6 {
		t.Fatal("byte cap ignored")
	}
	cache.set("b", []byte("12"))
	if cache.bytes != 2 {
		t.Fatal("replacement double counted")
	}
	cache.set("large", make([]byte, 11))
	if cache.bytes != 2 {
		t.Fatal("oversized entry admitted")
	}
	entry := cache.entries["b"]
	entry.expiresAt = time.Now().Add(-time.Second)
	cache.entries["b"] = entry
	cache.get("b")
	if cache.bytes != 0 {
		t.Fatal("expired bytes retained")
	}
}

func TestTokenRefreshDoesNotLockCancelledCallers(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		fmt.Fprint(w, `{"access_token":"synthetic-token","expires_in":3600}`)
	}))
	defer upstream.Close()
	server, err := NewServer(Config{OfficialBaseURL: upstream.URL, OfficialClientID: "123", OfficialClientSecret: "synthetic-secret", ProviderRequestsPerSecond: 10000})
	if err != nil {
		t.Fatal(err)
	}
	first := make(chan error, 1)
	go func() { _, err := server.official.accessToken(context.Background()); first <- err }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	second := make(chan error, 1)
	go func() { _, err := server.official.accessToken(ctx); second <- err }()
	select {
	case err := <-second:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Error(err)
		}
	case <-time.After(time.Second):
		t.Error("token mutex blocked a cancelled waiter")
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if _, err := server.official.accessToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("duplicated token refresh")
	}
}
