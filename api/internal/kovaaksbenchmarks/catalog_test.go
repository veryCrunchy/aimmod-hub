package kovaaksbenchmarks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
)

func TestCatalogPaginationBeyondTenPagesAndUnrankedDefinitions(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		payload := benchmarkListEnvelope{Max: 2, Total: 25, Data: []benchmarkListSummary{}}
		for i := page * 2; i < (page+1)*2 && i < 25; i++ {
			rank := "Gold"
			if i%2 == 0 {
				rank = "No Rank"
			}
			payload.Data = append(payload.Data, benchmarkListSummary{BenchmarkID: uint32(i + 1), BenchmarkName: "Example benchmark", RankName: rank})
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()
	client := NewClient()
	client.baseURL = server.URL
	items, err := client.ListPlayerBenchmarkCatalog(context.Background(), "ExamplePlayer")
	if err != nil || len(items) != 25 || requests.Load() != 13 {
		t.Fatalf("incomplete pagination: %d items, %d requests, %v", len(items), requests.Load(), err)
	}
	ranked, err := client.ListPlayerBenchmarks(context.Background(), "ExamplePlayer")
	if err != nil || len(ranked) != 12 || requests.Load() != 13 {
		t.Fatal("ranked view changed or did not reuse catalog")
	}
}

func TestMalformedCatalogResponseCannotBecomeEmptySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":"temporarily unavailable"}`))
	}))
	defer server.Close()
	client := NewClient()
	client.baseURL = server.URL
	if _, err := client.ListPlayerBenchmarkCatalog(context.Background(), "ExamplePlayer"); err == nil {
		t.Fatal("missing data cached as empty catalog")
	}
	if len(client.listCache) != 0 {
		t.Fatal("malformed response populated cache")
	}
}
