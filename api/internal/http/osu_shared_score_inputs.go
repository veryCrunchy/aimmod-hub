package httpserver

import (
	"context"
	"strings"
	"sync"
	"time"

	osuservice "github.com/veryCrunchy/aimmod-hub/api/internal/osu"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

func sharedScoreItems(ctx context.Context, provider replayScoreProvider, replays []store.OsuPublicReplay) []osuservice.PublicScoreItem {
	items := make([]osuservice.PublicScoreItem, len(replays))
	for i, replay := range replays {
		ppSource := "unavailable"
		if replay.PerformancePoints != nil {
			ppSource = "local"
		}
		items[i] = osuservice.PublicScoreItem{OsuPublicReplay: replay, Source: "local", PPSource: ppSource}
	}
	enrichSharedScoreItems(ctx, provider, items)
	return items
}

type legacyReplayScoreProvider interface {
	GetPublicLegacyScore(context.Context, int64, string) (osuservice.OfficialScoreDetail, error)
}

func sameSharedScore(a, b store.OsuPublicReplay) bool {
	return a.OnlineScoreID == b.OnlineScoreID && a.OsuUserID == b.OsuUserID && a.BeatmapID == b.BeatmapID && a.Ruleset == b.Ruleset &&
		(a.BeatmapChecksum == "" || b.BeatmapChecksum == "" || strings.EqualFold(a.BeatmapChecksum, b.BeatmapChecksum))
}

// All rows are eligible; two workers and a deadline bound cold upstream work.
// Pending rows explicitly allow clients to retry the individual replay endpoint.
func enrichSharedScoreItems(ctx context.Context, provider replayScoreProvider, items []osuservice.PublicScoreItem) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	type identity struct {
		id, owner, beatmap int64
		mode, checksum     string
	}
	groups := map[identity][]int{}
	for i := range items {
		item := &items[i]
		if item.ShareID == "" || item.PerformancePoints != nil || item.PPCalculation != nil {
			continue
		}
		item.PPCalculationStatus = "unavailable"
		if provider == nil || item.OnlineScoreID <= 0 || item.OsuUserID <= 0 || item.BeatmapID <= 0 {
			continue
		}
		item.PPCalculationStatus = "pending"
		key := identity{item.OnlineScoreID, item.OsuUserID, item.BeatmapID, item.Ruleset, item.BeatmapChecksum}
		groups[key] = append(groups[key], i)
	}
	jobs := make(chan []int, len(groups))
	for _, indices := range groups {
		jobs <- indices
	}
	close(jobs)
	var workers sync.WaitGroup
	for worker := 0; worker < 2; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for indices := range jobs {
				if ctx.Err() != nil {
					return
				}
				local := items[indices[0]].OsuPublicReplay
				detail, err := provider.GetPublicScore(ctx, local.OnlineScoreID)
				if err != nil {
					continue
				}
				if detail.Status == "not_found" || (detail.Status == "available" && detail.Item != nil && !sameSharedScore(local, detail.Item.OsuPublicReplay)) {
					if legacy, ok := provider.(legacyReplayScoreProvider); ok {
						detail, err = legacy.GetPublicLegacyScore(ctx, local.OnlineScoreID, local.Ruleset)
						if err != nil {
							continue
						}
					}
				}
				if detail.Status == "rate_limited" {
					cancel()
					return
				}
				for _, index := range indices {
					item := &items[index]
					if detail.Status == "not_found" || detail.Status == "not_configured" || detail.Status == "invalid_response" {
						item.PPCalculationStatus = "unavailable"
						continue
					}
					if detail.Status != "available" || detail.Item == nil {
						continue
					}
					item.PPCalculationStatus = "unavailable"
					score := detail.Item
					if !sameSharedScore(local, score.OsuPublicReplay) {
						continue
					}
					item.PPCalculation = score.PPCalculation
					if item.PPCalculation != nil {
						item.PPCalculationStatus = "available"
					}
					if score.PerformancePoints != nil && finiteBetween(*score.PerformancePoints, 0, 100_000) {
						pp := *score.PerformancePoints
						item.PerformancePoints = &pp
						item.PPSource = "official"
					}
				}
			}
		}()
	}
	workers.Wait()
}
