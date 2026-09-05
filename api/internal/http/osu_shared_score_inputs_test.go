package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	osuservice "github.com/veryCrunchy/aimmod-hub/api/internal/osu"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

func sharedInputFixture() (store.OsuPublicReplay, *replayEnrichmentProvider) {
	local := store.OsuPublicReplay{ShareID: "osu_" + strings.Repeat("a", 32), Visibility: "public", OnlineScoreID: 42, OsuUserID: 7, BeatmapID: 9, Ruleset: "osu", BeatmapChecksum: strings.Repeat("a", 32), HasReplayFile: true, Analysis: json.RawMessage(`{"retained":true}`)}
	lazer := false
	input := &osuservice.ScorePPCalculationInput{Version: 1, BeatmapID: 9, BeatmapChecksum: local.BeatmapChecksum, RulesetID: 0, Lazer: &lazer, Mods: []osuservice.OfficialScoreMod{}, Statistics: map[string]int{"great": 100, "miss": 1}, MaxCombo: 90}
	return local, &replayEnrichmentProvider{detail: osuservice.OfficialScoreDetail{Status: "available", Item: &osuservice.PublicScoreItem{OsuPublicReplay: local, PPCalculation: input}}}
}

func TestSharedEndpointsExposeExactInputsWhenOfficialPPNull(t *testing.T) {
	for _, route := range []string{"replay", "community", "profile"} {
		t.Run(route, func(t *testing.T) {
			local, provider := sharedInputFixture()
			h := newOsuSyncHandler(&replayEnrichmentStore{replay: local}, nil, provider)
			w := httptest.NewRecorder()
			switch route {
			case "replay":
				h.handleReplay(w, httptest.NewRequest("GET", "/api/osu/v1/replays/"+local.ShareID, nil))
			case "community":
				h.handleCommunity(w, httptest.NewRequest("GET", "/api/osu/v1/community", nil))
			case "profile":
				h.handleProfile(w, httptest.NewRequest("GET", "/api/osu/v1/profiles/test-player", nil))
			}
			var result struct {
				osuservice.PublicScoreItem
				Items         []osuservice.PublicScoreItem `json:"items"`
				RecentReplays []osuservice.PublicScoreItem `json:"recentReplays"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			item := result.PublicScoreItem
			if route == "community" {
				if len(result.Items) != 1 {
					t.Fatal("missing item")
				}
				item = result.Items[0]
			}
			if route == "profile" {
				if len(result.RecentReplays) != 1 {
					t.Fatal("missing replay")
				}
				item = result.RecentReplays[0]
			}
			if w.Code != 200 || item.PPCalculation == nil || item.PPCalculation.Lazer == nil || *item.PPCalculation.Lazer || item.PPCalculation.Statistics["miss"] != 1 || item.PerformancePoints != nil || !item.HasReplayFile || string(item.Analysis) != string(local.Analysis) {
				t.Fatalf("lost exact input or fabricated PP: %s", w.Body.String())
			}
		})
	}
}

func TestSharedInputsRejectIdentityMismatchAndUnknownLegacy(t *testing.T) {
	for _, field := range []string{"score", "owner", "map", "mode", "checksum", "legacy"} {
		t.Run(field, func(t *testing.T) {
			local, provider := sharedInputFixture()
			switch field {
			case "score":
				local.OnlineScoreID++
			case "owner":
				local.OsuUserID++
			case "map":
				local.BeatmapID++
			case "mode":
				local.Ruleset = "mania"
			case "checksum":
				local.BeatmapChecksum = strings.Repeat("b", 32)
			case "legacy":
				local.OnlineScoreID = 0
			}
			item := sharedScoreItems(context.Background(), provider, []store.OsuPublicReplay{local})[0]
			if item.PPCalculation != nil || item.PerformancePoints != nil {
				t.Fatal("invented input/PP")
			}
		})
	}
}

func TestSharedInputLookupsBoundedDeduplicatedAndRetryFailures(t *testing.T) {
	local, provider := sharedInputFixture()
	replays := []store.OsuPublicReplay{local, local}
	sharedScoreItems(context.Background(), provider, replays)
	if provider.calls != 1 {
		t.Fatal("duplicate lookup")
	}
	provider.calls = 0
	for i := int64(1); i <= 100; i++ {
		next := local
		next.OnlineScoreID = i
		replays = append(replays, next)
	}
	sharedScoreItems(context.Background(), provider, replays)
	if provider.calls != 100 {
		t.Fatalf("unbounded lookup: %d", provider.calls)
	}
	provider.calls = 0
	provider.detail.Status = "rate_limited"
	sharedScoreItems(context.Background(), provider, replays)
	if provider.calls < 1 || provider.calls > 2 {
		t.Fatal("continued after rate limit")
	}
	provider.detail.Status = "available"
	if sharedScoreItems(context.Background(), provider, []store.OsuPublicReplay{local})[0].PPCalculation == nil {
		t.Fatal("failure pinned across requests")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	provider.calls = 0
	sharedScoreItems(ctx, provider, replays)
	if provider.calls != 0 {
		t.Fatal("lookup after cancellation")
	}
}

type legacySharedProvider struct {
	*replayEnrichmentProvider
	legacy osuservice.OfficialScoreDetail
}

func (p *legacySharedProvider) GetPublicLegacyScore(context.Context, int64, string) (osuservice.OfficialScoreDetail, error) {
	return p.legacy, nil
}

func TestSharedLegacyFallbackStillRequiresOwnerMapMode(t *testing.T) {
	for _, field := range []string{"exact", "owner", "map", "mode"} {
		t.Run(field, func(t *testing.T) {
			local, modern := sharedInputFixture()
			legacy := modern.detail
			legacyItem := *legacy.Item
			legacy.Item = &legacyItem
			switch field {
			case "owner":
				legacy.Item.OsuUserID++
			case "map":
				legacy.Item.BeatmapID++
			case "mode":
				legacy.Item.Ruleset = "mania"
			}
			modern.detail = osuservice.OfficialScoreDetail{Status: "not_found"}
			provider := &legacySharedProvider{replayEnrichmentProvider: modern, legacy: legacy}
			item := sharedScoreItems(context.Background(), provider, []store.OsuPublicReplay{local})[0]
			if (item.PPCalculation != nil) != (field == "exact") {
				t.Fatal("unsafe legacy fallback")
			}
		})
	}
}

type isolatedFailureProvider struct {
	detail osuservice.OfficialScoreDetail
}

func (p isolatedFailureProvider) GetPublicScore(_ context.Context, id int64) (osuservice.OfficialScoreDetail, error) {
	if id != 42 {
		return osuservice.OfficialScoreDetail{}, errors.New("temporary upstream failure")
	}
	return p.detail, nil
}

func TestSharedTransientFailureDoesNotBlockOtherRows(t *testing.T) {
	local, provider := sharedInputFixture()
	other := local
	other.OnlineScoreID = 43
	items := sharedScoreItems(context.Background(), isolatedFailureProvider{provider.detail}, []store.OsuPublicReplay{other, local})
	if items[0].PPCalculationStatus != "pending" || items[1].PPCalculation == nil {
		t.Fatal("failure blocked neighbor or lost retry state")
	}
}
