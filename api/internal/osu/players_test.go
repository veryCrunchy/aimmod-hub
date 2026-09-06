package osu

import (
	"context"
	"encoding/json"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type memoryPlayerIndex struct {
	players []store.OsuPublicProfile
	scores  []store.IndexedOsuScore
	mode    string
}

func (m *memoryPlayerIndex) SaveIndexedOsuPlayers(_ context.Context, mode string, p []store.OsuPublicProfile) error {
	m.players = append(m.players, p...)
	m.mode = mode
	return nil
}
func (m *memoryPlayerIndex) ListIndexedOsuPlayers(context.Context, string, string, int) ([]store.OsuPublicProfile, error) {
	return m.players, nil
}
func (m *memoryPlayerIndex) ClaimOsuPlayerIndexPage(context.Context) (string, int, error) {
	return "osu", 1, nil
}
func (m *memoryPlayerIndex) FinishOsuPlayerIndexPage(context.Context, string, int, int) error {
	return nil
}
func (m *memoryPlayerIndex) SaveIndexedOsuScores(_ context.Context, s []store.IndexedOsuScore) error {
	m.scores = append(m.scores, s...)
	return nil
}
func (m *memoryPlayerIndex) ListIndexedOsuScores(context.Context, int, bool) ([]json.RawMessage, error) {
	items := []json.RawMessage{}
	for _, s := range m.scores {
		items = append(items, s.Item)
	}
	return items, nil
}
func (m *memoryPlayerIndex) ClaimOsuScoreIndexPlayer(context.Context) (int64, string, error) {
	return 42, "osu", nil
}
func (m *memoryPlayerIndex) FinishOsuScoreIndexPlayer(context.Context, int64, string) error {
	return nil
}

func TestPublicPlayersRankingsSearchAndUsernameWithoutHub(t *testing.T) {
	index := &memoryPlayerIndex{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			w.Write([]byte(`{"access_token":"example","expires_in":3600}`))
		case "/api/v2/rankings/mania/performance":
			if r.URL.Query().Get("cursor[page]") != "2" {
				t.Error("lost ranking cursor")
			}
			w.Write([]byte(`{"ranking":[{"user":{"id":42,"username":"ExamplePlayer","country_code":"ZZ"},"pp":123.4,"global_rank":51,"play_count":500}],"cursor":{"page":3}}`))
		case "/api/v2/search":
			if r.URL.Query().Get("mode") != "user" || r.URL.Query().Get("query") != "Example" {
				t.Error("invalid public user search")
			}
			w.Write([]byte(`{"user":{"data":[{"id":42,"username":"ExamplePlayer"}],"total":21}}`))
		case "/api/v2/users/ExamplePlayer/taiko":
			if r.URL.Query().Get("key") != "username" {
				t.Error("missing explicit username lookup")
			}
			w.Write([]byte(`{"id":42,"username":"ExamplePlayer","statistics":{"pp":123.4}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	s, err := NewServer(Config{OfficialBaseURL: upstream.URL, OfficialClientID: "1", OfficialClientSecret: "example", PlayerIndex: index, ProviderRequestsPerSecond: 10000})
	if err != nil {
		t.Fatal(err)
	}
	page, err := s.ListPublicPlayers(context.Background(), "mania", "", 2)
	if err != nil || len(page.Items) != 1 || page.NextPage != 3 || *page.Items[0].GlobalRank != 51 || page.Items[0].HubHandle != "" {
		t.Fatalf("ranking failed: %+v %v", page, err)
	}
	page, err = s.ListPublicPlayers(context.Background(), "osu", "Example", 1)
	if err != nil || page.NextPage != 2 || page.Items[0].OsuUserID != 42 {
		t.Fatalf("search failed: %+v %v", page, err)
	}
	profile, err := s.ResolvePublicPlayer(context.Background(), "ExamplePlayer", "taiko")
	if err != nil || profile.OsuUserID != 42 || profile.HubHandle != "" || index.mode != "taiko" || len(index.players) != 3 {
		t.Fatalf("profile not indexed: %+v %v", profile, err)
	}
	// An upstream failure still exposes the previously indexed public players.
	page, err = s.ListPublicPlayers(context.Background(), "osu", "", 1)
	if err != nil || !page.Cached || len(page.Items) == 0 {
		t.Fatal("public index fallback failed")
	}
}

func TestNonAimModReplayIndexKeepsScoreIdentityAndAvailability(t *testing.T) {
	index := &memoryPlayerIndex{}
	s := &Server{official: &officialAdapter{playerIndex: index}}
	mode := 0
	score := officialScore{ID: 42, UserID: 7, User: officialUser{ID: 7, Username: "ExamplePlayer"}, RulesetID: &mode, HasReplay: true, Passed: true, EndedAt: time.Now(), BeatmapID: 9, Statistics: map[string]int{"great": 10}}
	normalized := normalizePublicScore(score, "osu")
	if err := s.retainPublicScores(context.Background(), []OfficialPublicScore{normalized}); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListPublicIndexedScores(context.Background(), 100, true)
	if err != nil || len(items) != 1 || !items[0].OfficialReplayExists || items[0].Replay.OsuUsername != "ExamplePlayer" || items[0].Replay.HubHandle != "" || items[0].Replay.HasReplayFile {
		t.Fatal("external replay lost identity or fabricated upload")
	}
	merged := MergePublicScores(nil, items)
	if merged[0].OfficialScoreID != "42" || merged[0].ShareID != "" {
		t.Fatal("external replay must use its official score route")
	}
	if len(index.players) != 1 {
		t.Fatal("score owner was not discovered")
	}
}
