package service

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/kovaaksbenchmarks"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

const (
	benchmarkCatalogRefresh   = 12 * time.Hour
	benchmarkDefinitionMaxAge = 7 * 24 * time.Hour
	benchmarkSyncBatch        = 20
	benchmarkSyncPause        = 1500 * time.Millisecond
	benchmarkSyncIdle         = 10 * time.Minute
)

// benchmarkSyncer keeps public benchmark definitions in Postgres. It runs
// slowly in the background so page views never wait on hundreds of
// KovaaK's requests and the provider is never hit in bursts.
type benchmarkSyncer struct {
	store   *store.Store
	client  *kovaaksbenchmarks.Client
	pause   time.Duration
	idle    time.Duration
	lastCat time.Time
}

// StartBackgroundSync begins syncing KovaaK's benchmark definitions until ctx
// ends. Set AIMMOD_KOVAAKS_SYNC=off to disable it.
func (s *HubServer) StartBackgroundSync(ctx context.Context) {
	if s.store == nil || strings.EqualFold(strings.TrimSpace(os.Getenv("AIMMOD_KOVAAKS_SYNC")), "off") {
		return
	}
	syncer := &benchmarkSyncer{store: s.store, client: s.benchmarks, pause: benchmarkSyncPause, idle: benchmarkSyncIdle}
	go syncer.run(ctx)
}

func (b *benchmarkSyncer) run(ctx context.Context) {
	for ctx.Err() == nil {
		worked := b.step(ctx)
		wait := b.idle
		if worked {
			wait = b.pause
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// step refreshes the catalog when due and syncs one batch of definitions.
// It reports whether any definition was processed.
func (b *benchmarkSyncer) step(ctx context.Context) bool {
	if time.Since(b.lastCat) > benchmarkCatalogRefresh {
		if err := b.refreshCatalog(ctx); err != nil {
			log.Printf("benchmark sync: catalog refresh failed: %v", err)
		} else {
			b.lastCat = time.Now()
		}
	}
	ids, err := b.store.ListKovaaksBenchmarksNeedingSync(ctx, benchmarkDefinitionMaxAge, benchmarkSyncBatch)
	if err != nil {
		log.Printf("benchmark sync: %v", err)
		return false
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			return true
		}
		b.syncDefinition(ctx, id)
		select {
		case <-ctx.Done():
		case <-time.After(b.pause):
		}
	}
	return len(ids) > 0
}

func (b *benchmarkSyncer) refreshCatalog(ctx context.Context) error {
	items, err := b.client.ListCatalog(ctx)
	if err != nil {
		return err
	}
	meta := make([]store.KovaaksBenchmarkMeta, 0, len(items))
	for _, item := range items {
		meta = append(meta, store.KovaaksBenchmarkMeta{
			BenchmarkID: item.BenchmarkID, Name: item.BenchmarkName, IconURL: item.BenchmarkIconURL,
			Author: item.BenchmarkAuthor, BenchmarkType: item.BenchmarkType,
		})
	}
	return b.store.UpsertKovaaksBenchmarkCatalog(ctx, meta)
}

func (b *benchmarkSyncer) syncDefinition(ctx context.Context, id uint32) {
	detail, err := b.client.GetBenchmarkDefinition(ctx, id)
	if err != nil || detail == nil {
		reason := "definition unavailable"
		if err != nil {
			reason = err.Error()
		}
		_ = b.store.MarkKovaaksBenchmarkSyncFailed(ctx, id, reason)
		return
	}
	ranks, scenarios := definitionRows(detail)
	var estimate *int64
	if len(scenarios) > 0 && scenarios[0].LeaderboardID > 0 {
		if board, err := b.client.GetScenarioLeaderboard(ctx, scenarios[0].LeaderboardID, 0, 1); err == nil {
			total := int64(board.Total)
			estimate = &total
		}
	}
	if err := b.store.SaveKovaaksBenchmarkDefinition(ctx, id, ranks, scenarios, estimate); err != nil {
		log.Printf("benchmark sync: save %d: %v", id, err)
	}
}

// definitionRows flattens a provider detail into stored rows, keeping order.
func definitionRows(detail *kovaaksbenchmarks.BenchmarkDetail) ([]store.KovaaksBenchmarkRank, []store.KovaaksBenchmarkScenario) {
	ranks := make([]store.KovaaksBenchmarkRank, 0, len(detail.Ranks))
	for _, rank := range detail.Ranks {
		ranks = append(ranks, store.KovaaksBenchmarkRank{
			RankIndex: rank.RankIndex, RankName: rank.RankName, IconURL: rank.IconURL, Color: rank.Color, FrameURL: rank.FrameURL,
		})
	}
	var scenarios []store.KovaaksBenchmarkScenario
	position := 0
	for _, categoryName := range detail.CategoryOrder {
		category := detail.Categories[categoryName]
		for _, scenarioName := range category.ScenarioOrder {
			scenario := category.Scenarios[scenarioName]
			scenarios = append(scenarios, store.KovaaksBenchmarkScenario{
				Position: position, CategoryName: categoryName, ScenarioName: scenarioName,
				LeaderboardID: scenario.LeaderboardID, RankMaxes: scenario.RankMaxes,
			})
			position++
		}
	}
	return ranks, scenarios
}
