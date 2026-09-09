package osu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

const maxUpstreamResponseBytes = 8 << 20

type upstreamHTTPError struct {
	StatusCode int
}

func (e *upstreamHTTPError) Error() string {
	return fmt.Sprintf("upstream returned HTTP %d", e.StatusCode)
}

func isUpstreamHTTPStatus(err error, statusCode int) bool {
	var httpErr *upstreamHTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode == statusCode
}

type upstreamClient struct {
	baseURL   *url.URL
	http      *http.Client
	cache     *responseCache
	limiter   *intervalLimiter
	userAgent string
	requests  singleflight.Group
	capacity  chan struct{}
	timeout   time.Duration
}

func newUpstreamClient(rawBaseURL string, httpClient *http.Client, cache *responseCache, limiter *intervalLimiter, userAgent string) (*upstreamClient, error) {
	baseURL, err := url.Parse(strings.TrimRight(rawBaseURL, "/"))
	if err != nil {
		return nil, err
	}
	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported upstream URL scheme %q", baseURL.Scheme)
	}
	return &upstreamClient{
		baseURL:   baseURL,
		http:      httpClient,
		cache:     cache,
		limiter:   limiter,
		userAgent: userAgent,
		capacity:  make(chan struct{}, 16),
		timeout:   upstreamTimeout(httpClient),
	}, nil
}

func (c *upstreamClient) resolve(path string, query url.Values) string {
	resolved := *c.baseURL
	resolved.Path = strings.TrimRight(c.baseURL.Path, "/") + "/" + strings.TrimLeft(path, "/")
	resolved.RawQuery = query.Encode()
	return resolved.String()
}

func upstreamTimeout(client *http.Client) time.Duration {
	if client.Timeout > 0 {
		return client.Timeout
	}
	return 10 * time.Second
}

func (c *upstreamClient) get(ctx context.Context, path string, query url.Values, authorization string) ([]byte, error) {
	return c.getResponse(ctx, path, query, authorization, false)
}

// All requests use application public scope. Keep the score format in the key;
// legacy and modern score identities must never share a cache entry.
func (c *upstreamClient) getResponse(ctx context.Context, path string, query url.Values, authorization string, modernScore bool) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	requestURL := c.resolve(path, query)
	key := "GET " + requestURL
	if modernScore {
		key = "GET score-v20220705 " + requestURL
	}
	if cached, ok := c.cache.get(key); ok {
		return cached, nil
	}
	result := c.requests.DoChan(key, func() (any, error) {
		if cached, ok := c.cache.get(key); ok {
			return cached, nil
		}
		// Fail fast for many distinct misses; never build an unbounded upstream queue.
		select {
		case c.capacity <- struct{}{}:
			defer func() { <-c.capacity }()
		default:
			return nil, &upstreamHTTPError{StatusCode: http.StatusServiceUnavailable}
		}
		// Queueing is included in this deadline. A cancelled browser must not cancel
		// another browser's shared fetch, and orphaned work still has a strict bound.
		fetchCtx, cancel := context.WithTimeout(context.Background(), c.timeout)
		defer cancel()
		if err := c.limiter.wait(fetchCtx); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", c.userAgent)
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		if modernScore {
			req.Header.Set("x-api-version", "20220705")
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.backoff(resp.Header.Get("Retry-After"))
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, &upstreamHTTPError{StatusCode: resp.StatusCode}
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamResponseBytes+1))
		if err != nil {
			return nil, err
		}
		if len(body) > maxUpstreamResponseBytes {
			return nil, fmt.Errorf("upstream response exceeded %d bytes", maxUpstreamResponseBytes)
		}
		if modernScore && !json.Valid(body) {
			return nil, fmt.Errorf("invalid score JSON")
		}
		c.cache.set(key, body)
		return body, nil
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case response := <-result:
		if response.Err != nil {
			return nil, response.Err
		}
		return append([]byte(nil), response.Val.([]byte)...), nil
	}
}
