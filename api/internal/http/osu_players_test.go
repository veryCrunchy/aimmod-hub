package httpserver

import (
	"context"
	"encoding/json"
	osuservice "github.com/veryCrunchy/aimmod-hub/api/internal/osu"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	"net/http/httptest"
	"testing"
)

type playerDirectoryStub struct{ calls int }

func (p *playerDirectoryStub) ListPublicPlayers(context.Context, string, string, int) (osuservice.PublicPlayersPage, error) {
	p.calls++
	return osuservice.PublicPlayersPage{Items: []store.OsuPublicProfile{{OsuUserID: 42, OsuUsername: "ExamplePlayer"}}}, nil
}
func TestPlayerDirectoryPublicAndValidatesBeforeUpstream(t *testing.T) {
	p := &playerDirectoryStub{}
	h := newOsuPlayersHandler(p)
	for _, path := range []string{"/api/osu/v1/players?page=0", "/api/osu/v1/players?mode=invalid", "/api/osu/v1/players?page=201"} {
		w := httptest.NewRecorder()
		h.list(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 400 {
			t.Fatal(w.Code)
		}
	}
	if p.calls != 0 {
		t.Fatal("invalid input reached provider")
	}
	w := httptest.NewRecorder()
	h.list(w, httptest.NewRequest("GET", "/api/osu/v1/players", nil))
	if w.Code != 200 || p.calls != 1 {
		t.Fatal("public directory requires no AimMod account")
	}
}

type indexedSEOStub struct{ seoStoreStub }

func (s indexedSEOStub) GetIndexedOsuProfile(context.Context, string, string) (store.OsuPublicProfile, error) {
	return store.OsuPublicProfile{OsuUserID: 42, OsuUsername: "ExamplePlayer"}, nil
}
func TestIndexedPlayerSEOWithoutSharedUploads(t *testing.T) {
	meta := resolveOsuDetailMeta(context.Background(), "/osu/profiles/42", "https://example.com/osu/profiles/42", indexedSEOStub{})
	if meta.NoIndex || meta.OGType != "profile" {
		t.Fatal("official player without AimMod uploads excluded from SEO")
	}
}

type indexedReplayProvider struct {
	scoreProviderStub
	replaysOnly bool
}

func (p *indexedReplayProvider) ListPublicIndexedScores(_ context.Context, _ int, only bool) ([]osuservice.OfficialPublicScore, error) {
	p.replaysOnly = only
	return []osuservice.OfficialPublicScore{{Replay: store.OsuPublicReplay{OnlineScoreID: 77, OsuUserID: 42, BeatmapID: 9, Ruleset: "osu", OsuUsername: "ExamplePlayer", Visibility: "public"}, OfficialReplayExists: true}}, nil
}
func TestReplayLibraryIncludesOfficialPlayersWithoutUploads(t *testing.T) {
	provider := &indexedReplayProvider{}
	h := newOsuSyncHandler(&fakeOsuSyncStore{}, nil, provider)
	w := httptest.NewRecorder()
	h.handleCommunity(w, httptest.NewRequest("GET", "/api/osu/v1/community?replays=true", nil))
	var body struct {
		Items []osuservice.PublicScoreItem `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || !provider.replaysOnly || len(body.Items) != 1 || body.Items[0].Source != "official" || body.Items[0].OfficialScoreID != "77" || body.Items[0].HasReplayFile {
		t.Fatal("official replay missing or misrepresented as AimMod upload")
	}
}
