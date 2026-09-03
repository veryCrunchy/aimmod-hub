package osu

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
	}, nil
}

func (c *upstreamClient) resolve(path string, query url.Values) string {
	resolved := *c.baseURL
	resolved.Path = strings.TrimRight(c.baseURL.Path, "/") + "/" + strings.TrimLeft(path, "/")
	resolved.RawQuery = query.Encode()
	return resolved.String()
}

func (c *upstreamClient) get(ctx context.Context, path string, query url.Values, authorization string) ([]byte, error) {
	requestURL := c.resolve(path, query)
	cacheKey := "GET " + requestURL
	if cached, ok := c.cache.get(cacheKey); ok {
		return cached, nil
	}
	if err := c.limiter.wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxUpstreamResponseBytes {
		return nil, fmt.Errorf("upstream response exceeded %d bytes", maxUpstreamResponseBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &upstreamHTTPError{StatusCode: resp.StatusCode}
	}
	c.cache.set(cacheKey, body)
	return body, nil
}
