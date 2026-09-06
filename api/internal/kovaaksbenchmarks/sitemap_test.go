package kovaaksbenchmarks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSitemapDiscoveryObservesSuccessfulPublicListsOnly(t *testing.T) {
	failed := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failed {
			http.Error(w, "unavailable", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"benchmarkId":123,"benchmarkName":"Example benchmark","rankName":"Gold"}]}`))
	}))
	defer server.Close()
	observed := 0
	client := NewClient(func(_ context.Context, username string, ids []uint32) {
		observed++
		if username != "ExamplePlayer" || len(ids) != 1 || ids[0] != 123 {
			t.Fatalf("unexpected discovery: %q %v", username, ids)
		}
	})
	client.baseURL = server.URL
	for i := 0; i < 2; i++ {
		items, err := client.ListPlayerBenchmarks(context.Background(), "ExamplePlayer")
		if err != nil || len(items) != 1 {
			t.Fatalf("public list failed: %v", err)
		}
	}
	if observed != 1 {
		t.Fatal("cache hits should not rewrite the database")
	}
	failed = true
	if _, err := client.ListPlayerBenchmarks(context.Background(), "AnotherExample"); err == nil {
		t.Fatal("expected provider error")
	}
	if observed != 1 {
		t.Fatal("failed request replaced public catalog")
	}
}
