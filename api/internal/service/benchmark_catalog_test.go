package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/kovaaksbenchmarks"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	hubv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1"
)

type catalogFixture struct{ failList, failDetail bool }

func (f catalogFixture) ListPlayerBenchmarkCatalog(_ context.Context, username string) ([]kovaaksbenchmarks.ProfileBenchmarkSummary, error) {
	if f.failList && username == "ExampleTwo" {
		return nil, errors.New("provider unavailable")
	}
	items := make([]kovaaksbenchmarks.ProfileBenchmarkSummary, 35)
	for i := range items {
		items[i] = kovaaksbenchmarks.ProfileBenchmarkSummary{BenchmarkID: uint32(i + 1), BenchmarkName: fmt.Sprintf("Example benchmark %02d", i+1), OverallRankName: "Gold"}
	}
	items[34].OverallRankName = "No Rank"
	return items, nil
}
func (f catalogFixture) GetBenchmarkDetail(_ context.Context, id uint32, _ string) (*kovaaksbenchmarks.BenchmarkDetail, error) {
	if f.failDetail && id == 3 {
		return nil, errors.New("provider unavailable")
	}
	rank := uint32(1)
	if id == 2 {
		rank = 0
	}
	return &kovaaksbenchmarks.BenchmarkDetail{OverallRank: rank, Ranks: []kovaaksbenchmarks.BenchmarkRankVisual{{}, {RankIndex: 1, RankName: "Gold"}}}, nil
}
func catalogUsers() []store.BenchmarkUserIdentity {
	return []store.BenchmarkUserIdentity{{SteamID: "synthetic-one", KovaaksUsername: "ExampleOne"}, {SteamID: "synthetic-two", KovaaksUsername: "ExampleTwo"}}
}

func TestCatalogRetainsEveryDefinitionAndCountsRanksSeparately(t *testing.T) {
	items, err := loadBenchmarkCatalog(context.Background(), catalogUsers(), catalogFixture{})
	if err != nil || len(items) != 35 {
		t.Fatalf("catalog lost definitions: count=%d err=%v", len(items), err)
	}
	for _, item := range items {
		want := uint32(2)
		if item.BenchmarkId == 2 || item.BenchmarkId == 35 {
			want = 0
		}
		if item.PlayerCount != want {
			t.Fatalf("benchmark %d: count=%d want=%d", item.BenchmarkId, item.PlayerCount, want)
		}
	}
}

func TestCatalogNeverCachesPartialUpstreamResults(t *testing.T) {
	for _, fixture := range []catalogFixture{{failList: true}, {failDetail: true}} {
		var cache publicResultCache[[]*hubv1.BenchmarkListItem]
		cache.entries = map[string]publicCacheEntry[[]*hubv1.BenchmarkListItem]{"catalog": {value: []*hubv1.BenchmarkListItem{{BenchmarkId: 999}}, expires: time.Now().Add(-time.Second)}}
		_, err := cache.get(context.Background(), "catalog", time.Minute, func(ctx context.Context) ([]*hubv1.BenchmarkListItem, error) {
			return loadBenchmarkCatalog(ctx, catalogUsers(), fixture)
		})
		if err == nil {
			t.Fatal("partial result was reported as success")
		}
		if cached := cache.entries["catalog"].value; len(cached) != 1 || cached[0].BenchmarkId != 999 {
			t.Fatal("partial refresh replaced good catalog")
		}
		items, err := cache.get(context.Background(), "catalog", time.Minute, func(ctx context.Context) ([]*hubv1.BenchmarkListItem, error) {
			return loadBenchmarkCatalog(ctx, catalogUsers(), catalogFixture{})
		})
		if err != nil || len(items) != 35 {
			t.Fatal("retry did not recover complete catalog")
		}
	}
}
