package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// Clients that cannot classify a scenario themselves (the in-game mod) send
// an empty scenario type. The Hub then uses the type other runs of the same
// scenario were classified as, so those runs still reach type leaderboards.

const (
	scenarioTypeCacheTTL   = 10 * time.Minute
	scenarioTypeCacheLimit = 4096
	// Only recent runs vote, so the lookup stays an index range scan.
	scenarioTypeSampleRuns = 500
)

// KnownScenarioType reports whether a type is an actual classification.
func KnownScenarioType(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !strings.EqualFold(value, "unknown")
}

type scenarioTypeEntry struct {
	value     string
	expiresAt time.Time
}

type scenarioTypeCache struct {
	mu      sync.Mutex
	entries map[string]scenarioTypeEntry
}

func (c *scenarioTypeCache) get(key string, now time.Time) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || now.After(entry.expiresAt) {
		return "", false
	}
	return entry.value, true
}

func (c *scenarioTypeCache) put(key, value string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil || len(c.entries) >= scenarioTypeCacheLimit {
		c.entries = make(map[string]scenarioTypeEntry)
	}
	c.entries[key] = scenarioTypeEntry{value: value, expiresAt: now.Add(scenarioTypeCacheTTL)}
}

// InferScenarioType returns the most common known type among the latest runs
// of the scenario, or "" when none is known. Results are cached briefly.
func (s *Store) InferScenarioType(ctx context.Context, scenarioName string) (string, error) {
	name := strings.TrimSpace(scenarioName)
	if name == "" {
		return "", nil
	}
	key := strings.ToLower(name)
	now := time.Now()
	if value, ok := s.scenarioTypes.get(key, now); ok {
		return value, nil
	}
	var value string
	err := s.pool.QueryRow(ctx, `
		SELECT scenario_type
		FROM (
			SELECT scenario_type
			FROM scenario_runs
			WHERE scenario_name = $1
			ORDER BY played_at DESC
			LIMIT $2
		) recent
		WHERE scenario_type <> '' AND LOWER(scenario_type) <> 'unknown'
		GROUP BY scenario_type
		ORDER BY COUNT(*) DESC, scenario_type ASC
		LIMIT 1
	`, name, scenarioTypeSampleRuns).Scan(&value)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("infer scenario type: %w", err)
	}
	s.scenarioTypes.put(key, value, now)
	return value, nil
}
