package store

import (
	"strconv"
	"testing"
	"time"
)

func TestKnownScenarioType(t *testing.T) {
	for value, want := range map[string]bool{"": false, " ": false, "Unknown": false, "unknown": false, "Tracking": true, "StaticClicking": true} {
		if got := KnownScenarioType(value); got != want {
			t.Fatalf("KnownScenarioType(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestScenarioTypeCacheExpiresAndStaysBounded(t *testing.T) {
	var cache scenarioTypeCache
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cache.put("synthetic scenario", "Tracking", now)
	if value, ok := cache.get("synthetic scenario", now.Add(time.Minute)); !ok || value != "Tracking" {
		t.Fatalf("cached value = %q, %v", value, ok)
	}
	if _, ok := cache.get("synthetic scenario", now.Add(scenarioTypeCacheTTL+time.Second)); ok {
		t.Fatal("expired entry was returned")
	}
	// A miss is cached too, so unclassified scenarios are not queried on every heartbeat.
	cache.put("unclassified", "", now)
	if value, ok := cache.get("unclassified", now); !ok || value != "" {
		t.Fatalf("cached miss = %q, %v", value, ok)
	}
	for i := 0; i < scenarioTypeCacheLimit+10; i++ {
		cache.put("scenario "+strconv.Itoa(i), "Tracking", now)
	}
	if len(cache.entries) > scenarioTypeCacheLimit {
		t.Fatalf("cache grew to %d entries", len(cache.entries))
	}
}
