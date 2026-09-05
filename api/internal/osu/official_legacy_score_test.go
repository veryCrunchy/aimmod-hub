package osu

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestPublicLegacyScoreRequiresAuthoritativeAliasAndMode(t *testing.T) {
	for _, tc := range []struct {
		name, legacy string
		mode         int
		available    bool
	}{
		{"exact", "42", 0, true}, {"different", "43", 0, false}, {"null", "null", 0, false}, {"zero", "0", 0, false}, {"mode", "42", 3, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			s := scoreTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/api/v2/scores/osu/42" {
					t.Errorf("wrong endpoint: %s", r.URL.Path)
				}
				fmt.Fprintf(w, `{"id":99,"legacy_score_id":%s,"user_id":7,"ruleset_id":%d,"beatmap_id":9,"has_replay":true,"mods":[],"statistics":{"great":100}}`, tc.legacy, tc.mode)
			})
			for i := 0; i < 2; i++ {
				result, err := s.GetPublicLegacyScore(context.Background(), 42, "osu")
				if err != nil {
					t.Fatal(err)
				}
				if (result.Status == "available") != tc.available {
					t.Fatalf("wrong status: %s", result.Status)
				}
				if tc.available && (result.Item.OnlineScoreID != 42 || result.Item.OfficialScoreID != "99" || result.Item.PPCalculation == nil || result.Item.PPCalculation.Lazer == nil || *result.Item.PPCalculation.Lazer || result.Replay.DownloadURL != "/api/osu/v1/official-scores/99/replay") {
					t.Fatal("lost canonical/legacy identity")
				}
			}
			if calls != 1 {
				t.Fatal("successful upstream response not cached")
			}
		})
	}
}
