package osu

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
)

func TestLiveOsuSkinsBrowseContract(t *testing.T) {
	if os.Getenv("AIMMOD_LIVE_PROVIDER_TESTS") != "1" {
		t.Skip("set AIMMOD_LIVE_PROVIDER_TESTS=1 to check live provider contracts")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	upstream, err := newUpstreamClient(
		"https://osuskins.net",
		client,
		newResponseCache(time.Minute, 16),
		newIntervalLimiter(2),
		"AimMod-Hub/provider-contract-test (https://aimmod.app)",
	)
	if err != nil {
		t.Fatal(err)
	}
	adapter := newOsuSkinsAdapter(upstream)
	items, _, err := adapter.search(context.Background(), &osuv1.SearchSkinsRequest{
		Query:   "Komori",
		Filters: &osuv1.SkinSearchFilters{Rulesets: []osuv1.Ruleset{osuv1.Ruleset_RULESET_OSU}},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 || !strings.HasPrefix(items[0].GetThumbnailUrl(), "https://cdn.osuskins.net/") {
		t.Fatalf("live search did not return a real osuskins.net asset: %+v", items)
	}
	detail, err := adapter.detail(context.Background(), items[0].GetSourceId())
	if err != nil {
		t.Fatal(err)
	}
	if detail.GetName() == "" || len(detail.GetScreenshots()) == 0 || detail.GetDownloadHandoff().GetAvailable() || !detail.GetDownloadHandoff().GetRequiresInteractiveVerification() {
		t.Fatalf("live detail contract mismatch: %+v", detail)
	}
}
