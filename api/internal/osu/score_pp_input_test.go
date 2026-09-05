package osu

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

const scorePPFixture = `{"id":123,"user_id":7,"ruleset_id":0,"beatmap_id":9,"build_id":456,"pp":null,"passed":false,"accuracy":0.7,"max_combo":12,"total_score":12345,"mods":[{"acronym":"DT","settings":{"speed_change":1.2}},{"acronym":"CL","settings":{"no_slider_head_accuracy":true}}],"statistics":{"great":10,"ok":2,"miss":3,"large_tick_hit":4,"large_tick_miss":1,"small_tick_hit":2,"slider_tail_hit":1},"maximum_statistics":{"great":100,"large_tick_hit":20,"small_tick_hit":10,"slider_tail_hit":10},"beatmap":{"id":9,"checksum":"0123456789abcdef0123456789abcdef"}}`

func scorePPInputFixture(t *testing.T) officialScore {
	t.Helper()
	var score officialScore
	if err := json.Unmarshal([]byte(scorePPFixture), &score); err != nil {
		t.Fatal(err)
	}
	return score
}

func TestScorePPInputPreservesExactPublicEvidence(t *testing.T) {
	score := scorePPInputFixture(t)
	normalized := normalizePublicScore(score, "osu")
	input := normalized.PPCalculation
	if input == nil || input.Version != 1 || input.BeatmapID != 9 || input.BeatmapChecksum != score.Beatmap.Checksum || input.RulesetID != 0 || input.Lazer == nil || !*input.Lazer {
		t.Fatalf("missing calculation identity: %+v", input)
	}
	if !reflect.DeepEqual(input.Mods, score.Mods) || !reflect.DeepEqual(input.Statistics, score.Statistics) || !reflect.DeepEqual(input.MaximumStatistics, score.MaximumStatistics) {
		t.Fatal("mod settings or nested judgement evidence lost")
	}
	if input.MaxCombo != 12 || input.Accuracy != .7 || input.Passed || input.TotalScore != 12345 || input.LegacyTotalScore != nil {
		t.Fatalf("actual failed score replaced by a hypothetical: %+v", input)
	}
	if normalized.Replay.PerformancePoints != nil {
		t.Fatal("null official PP fabricated")
	}
	if _, exists := input.Statistics["meh"]; exists {
		t.Fatal("sparse statistics were changed")
	}
}

func TestScorePPInputScoringIdentityRequiresEvidence(t *testing.T) {
	positive, zero := int64(1), int64(0)
	for _, tc := range []struct {
		name          string
		legacy, build *int64
		want          *bool
	}{
		{"unknown", nil, nil, nil},
		{"zero_legacy_id_is_stable", &zero, &zero, boolPointer(false)},
		{"stable", &positive, nil, boolPointer(false)},
		{"lazer", nil, &positive, boolPointer(true)},
		{"legacy_identity_is_authoritative", &positive, &positive, boolPointer(false)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			score := scorePPInputFixture(t)
			score.LegacyScoreID, score.BuildID = tc.legacy, tc.build
			input := normalizePublicScore(score, "osu").PPCalculation
			if !reflect.DeepEqual(input.Lazer, tc.want) {
				t.Fatalf("scoring identity guessed: got %v want %v", input.Lazer, tc.want)
			}
		})
	}
}

func boolPointer(value bool) *bool { return &value }

func TestScorePPInputModernAPIIdentityNullIsNotMissing(t *testing.T) {
	for _, tc := range []struct {
		json string
		want *bool
	}{
		{`{"ruleset_id":0,"legacy_score_id":null,"build_id":null,"mods":[],"statistics":{"great":1}}`, boolPointer(true)},
		{`{"ruleset_id":0,"legacy_score_id":0,"build_id":null,"mods":[],"statistics":{"great":1}}`, boolPointer(false)},
		{`{"ruleset_id":0,"legacy_score_id":123,"build_id":null,"mods":[],"statistics":{"great":1}}`, boolPointer(false)},
		{`{"ruleset_id":0,"build_id":null,"mods":[],"statistics":{"great":1}}`, nil},
	} {
		var score officialScore
		if err := json.Unmarshal([]byte(tc.json), &score); err != nil {
			t.Fatal(err)
		}
		input := normalizePublicScore(score, "osu").PPCalculation
		if !reflect.DeepEqual(input.Lazer, tc.want) {
			t.Fatalf("identity mismatch for %s: %+v", tc.json, input)
		}
	}
}

func TestScorePPInputMissingFieldsStayUnknown(t *testing.T) {
	score := scorePPInputFixture(t)
	score.Mods, score.Statistics, score.MaximumStatistics = nil, nil, nil
	score.BuildID = nil
	input := normalizePublicScore(score, "osu").PPCalculation
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"lazer", "mods", "statistics", "maximumStatistics", "legacyTotalScore"} {
		value, exists := fields[key]
		if !exists || value != nil {
			t.Errorf("%s must be explicitly unknown: %v", key, fields)
		}
	}
	score.RulesetID = nil
	if normalizePublicScore(score, "osu").PPCalculation != nil {
		t.Fatal("unknown ruleset was fabricated")
	}
}

func TestScorePPInputSurvivesLocalMergeWithoutUsingLocalStats(t *testing.T) {
	official := normalizePublicScore(scorePPInputFixture(t), "osu")
	local := official.Replay
	local.ShareID = "synthetic-share"
	local.MaxCombo, local.Count300 = 999, 999
	items := MergePublicScores([]store.OsuPublicReplay{local}, []OfficialPublicScore{official})
	if len(items) != 1 || items[0].Source != "merged" || items[0].PPCalculation.MaxCombo != 12 || items[0].PPCalculation.Statistics["great"] != 10 || len(items[0].Uploads) != 1 {
		t.Fatalf("merge replaced official calculation evidence: %+v", items)
	}
	if items[0].PerformancePoints != nil || items[0].PPSource != "unavailable" {
		t.Fatal("merge fabricated PP")
	}
	localOnly := MergePublicScores([]store.OsuPublicReplay{local}, nil)
	if localOnly[0].PPCalculation != nil {
		t.Fatal("local-only score acquired official calculation input")
	}
}

func TestScorePPInputFlowsThroughCachedListAndDetail(t *testing.T) {
	calls := 0
	s := scoreTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/api/v2/scores/123" {
			_, _ = w.Write([]byte(scorePPFixture))
		} else {
			_, _ = w.Write([]byte("[" + scorePPFixture + "]"))
		}
	})
	for range 2 {
		list, err := s.GetPublicUserScores(context.Background(), 7, "osu")
		if err != nil || len(list.Scores) != 1 {
			t.Fatalf("list: %+v %v", list, err)
		}
		detail, err := s.GetPublicScore(context.Background(), 123)
		if err != nil || detail.Item == nil {
			t.Fatalf("detail: %+v %v", detail, err)
		}
		if !reflect.DeepEqual(list.Scores[0].PPCalculation, detail.Item.PPCalculation) {
			t.Fatal("list/detail input differs")
		}
		body, err := json.Marshal(detail)
		if err != nil {
			t.Fatal(err)
		}
		var dto struct {
			Item struct {
				Input *ScorePPCalculationInput `json:"ppCalculation"`
			} `json:"item"`
		}
		if err := json.Unmarshal(body, &dto); err != nil || dto.Item.Input == nil || dto.Item.Input.Mods[0].Settings["speed_change"] != 1.2 {
			t.Fatalf("input lost in public JSON: %s", body)
		}
	}
	if calls != 3 {
		t.Fatalf("cached score input redownloaded: %d calls", calls)
	}
}
