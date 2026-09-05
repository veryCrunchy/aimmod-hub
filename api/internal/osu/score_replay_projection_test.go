package osu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

func TestPublicScoreListProjectsOfficialReplayWithoutDetailLookups(t *testing.T) {
	calls := 0
	s := scoreTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/v2/users/7/scores/best" && r.URL.Path != "/api/v2/users/7/scores/recent" {
			t.Errorf("unexpected per-score lookup: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[{"id":42,"user_id":7,"ruleset_id":0,"beatmap_id":9,"has_replay":true},{"id":43,"user_id":7,"ruleset_id":0,"beatmap_id":9,"has_replay":false},{"id":44,"user_id":7,"ruleset_id":0,"beatmap_id":9}]`))
	})
	for i := 0; i < 2; i++ {
		result, err := s.GetPublicUserScores(context.Background(), 7, "osu")
		if err != nil {
			t.Fatal(err)
		}
		items := MergePublicScores(nil, result.Scores)
		if len(items) != 3 {
			t.Fatalf("unexpected items: %+v", items)
		}
		for _, item := range items {
			want := item.OnlineScoreID == 42
			if item.OfficialReplayExists != want || item.HasReplayFile {
				t.Fatalf("flags confused: %+v", item)
			}
			checkReplayFlagJSON(t, item, want, false)
		}
	}
	if calls != 2 {
		t.Fatalf("wanted two cached category calls, no per-score lookups: %d", calls)
	}
}

func TestMergeOfficialReplayFlagRemainsIndependentOfHubAttachments(t *testing.T) {
	for _, exists := range []bool{true, false} {
		for _, attached := range []bool{true, false} {
			for _, fallback := range []bool{true, false} {
				t.Run(fmt.Sprintf("exists%t_attached%t_fallback%t", exists, attached, fallback), func(t *testing.T) {
					local, official := mergeFixture(), mergeFixture()
					local.ShareID = "osu_shared"
					local.HasReplayFile = attached
					local.Analysis = json.RawMessage(`{"retained":true}`)
					if fallback {
						local.OnlineScoreID = 0
					}
					items := MergePublicScores([]store.OsuPublicReplay{local}, []OfficialPublicScore{{Replay: official, FallbackEligible: true, OfficialReplayExists: exists}})
					if len(items) != 1 || items[0].Source != "merged" || items[0].OfficialReplayExists != exists || items[0].HasReplayFile != attached || string(items[0].Analysis) != string(local.Analysis) || len(items[0].Uploads) != 1 {
						t.Fatalf("merge lost provenance: %+v", items)
					}
					checkReplayFlagJSON(t, items[0], exists, attached)
				})
			}
		}
	}
	local := mergeFixture()
	local.HasReplayFile = true
	items := MergePublicScores([]store.OsuPublicReplay{local}, nil)
	if len(items) != 1 || items[0].OfficialReplayExists || !items[0].HasReplayFile {
		t.Fatalf("local upload invented official availability: %+v", items)
	}
	checkReplayFlagJSON(t, items[0], false, true)
}

func TestOfficialScoreDetailItemProjectsReplayFlagWithoutPermissionClaim(t *testing.T) {
	s := scoreTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/scores/42" {
			t.Errorf("unexpected lookup: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":42,"user_id":7,"ruleset_id":0,"beatmap_id":9,"has_replay":true}`))
	})
	result, err := s.GetPublicScore(context.Background(), 42)
	if err != nil || result.Item == nil || !result.Item.OfficialReplayExists || result.Item.HasReplayFile || !result.Replay.Exists || result.Replay.DownloadAvailable {
		t.Fatalf("unexpected projection: %+v %v", result, err)
	}
}

func checkReplayFlagJSON(t *testing.T, item PublicScoreItem, official, attachment bool) {
	t.Helper()
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["officialReplayExists"] != official || fields["hasReplayFile"] != attachment {
		t.Fatalf("flags missing/wrong in profile item JSON: %s", data)
	}
}
