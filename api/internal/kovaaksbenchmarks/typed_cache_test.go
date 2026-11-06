package kovaaksbenchmarks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestTypedCacheEvicts(t *testing.T) {
	entries := map[string]time.Time{"expired": time.Now().Add(-time.Second)}
	expiry := func(v time.Time) time.Time { return v }
	for i := 0; i < 1000; i++ {
		pruneTypedCache(entries, expiry)
		entries[fmt.Sprint(i)] = time.Now().Add(time.Hour)
	}
	if len(entries) != typedCacheEntries {
		t.Fatal(len(entries))
	}
	if _, ok := entries["expired"]; ok {
		t.Fatal("expired retained")
	}
	if _, ok := entries["0"]; ok {
		t.Fatal("oldest retained")
	}
}

func TestFreshRefreshDoesNotHideProviderFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "unavailable", 500) }))
	defer server.Close()
	c := NewClient(nil)
	c.responses.put(server.URL, []byte(`{"ok":true}`), time.Now())
	var out json.RawMessage
	if err := c.fetchJSON(context.Background(), server.URL, url.Values{}, 0, &out); err != nil {
		t.Fatal("normal stale fallback failed", err)
	}
	if err := c.fetchJSON(FreshResults(context.Background()), server.URL, url.Values{}, 0, &out); err == nil {
		t.Fatal("failed refresh hidden")
	}
}
