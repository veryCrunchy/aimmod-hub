package httpserver

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	osuservice "github.com/veryCrunchy/aimmod-hub/api/internal/osu"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

type scoreMetadataFunc func(context.Context, int64) (osuservice.OfficialScoreDetail, error)

func (f scoreMetadataFunc) GetPublicScore(ctx context.Context, id int64) (osuservice.OfficialScoreDetail, error) {
	return f(ctx, id)
}

func publicMetadataFixture() osuservice.OfficialScoreDetail {
	pp := 123.45
	return osuservice.OfficialScoreDetail{Status: "available", Item: &osuservice.PublicScoreItem{
		Source: "official", OfficialScoreID: "123", PPSource: "official",
		OsuPublicReplay: store.OsuPublicReplay{OnlineScoreID: 123, OsuUserID: 456, BeatmapID: 789,
			Visibility: "public", OsuUsername: "Player", Title: "Map <script>", Difficulty: "Hard", Accuracy: .9876, PerformancePoints: &pp},
	}}
}

func TestOfficialScoreSSRPublicMetadata(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(`<html><head><title>Default</title></head><body>App</body></html>`), 0600); err != nil {
		t.Fatal(err)
	}
	provider := scoreMetadataFunc(func(ctx context.Context, id int64) (osuservice.OfficialScoreDetail, error) {
		deadline, ok := ctx.Deadline()
		if id != 123 || !ok || time.Until(deadline) > 2*time.Second {
			t.Fatalf("unbounded or wrong lookup: %d, %v", id, deadline)
		}
		return publicMetadataFixture(), nil
	})
	r := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/osu/scores/123/?token=secret", nil)
	req.Header.Set("Authorization", "Bearer must-not-appear")
	NewSPAHandler(dir, nil, "https://aimmod.app", provider).ServeHTTP(r, req)
	head := readBrandHead(t, r.Body.String())
	for key, want := range map[string]string{
		"canonical": "https://aimmod.app/osu/scores/123", "og:url": "https://aimmod.app/osu/scores/123",
		"robots": "index, follow", "og:title": "Player on Map <script> [Hard] · osu! score 123 · AimMod Hub",
	} {
		if len(head[key]) != 1 || head[key][0] != want {
			t.Errorf("%s = %v, want %s", key, head[key], want)
		}
	}
	if !strings.Contains(head["description"][0], "98.76%") || !strings.Contains(head["twitter:description"][0], "123.45 PP") {
		t.Fatalf("score statistics missing: %v", head)
	}
	for _, forbidden := range []string{"<script>", "secret", "must-not-appear", "Download"} {
		if strings.Contains(r.Body.String(), forbidden) {
			t.Errorf("unexpected metadata content: %s", forbidden)
		}
	}
}

func TestOfficialScoreMetadataRejectsUnavailableOrNonpublicData(t *testing.T) {
	for _, scenario := range []string{"missing", "private", "unlisted", "wrong_id", "local", "permission_denied", "error", "nil_item"} {
		t.Run(scenario, func(t *testing.T) {
			provider := scoreMetadataFunc(func(context.Context, int64) (osuservice.OfficialScoreDetail, error) {
				detail := publicMetadataFixture()
				switch scenario {
				case "missing", "permission_denied":
					detail.Status = scenario
				case "private", "unlisted":
					detail.Item.Visibility = scenario
				case "wrong_id":
					detail.Item.OnlineScoreID++
				case "local":
					detail.Item.Source = "local"
				case "nil_item":
					detail.Item = nil
				case "error":
					return detail, errors.New("secret upstream detail")
				}
				return detail, nil
			})
			meta := resolvePageMeta(context.Background(), "/osu/scores/123", "https://aimmod.app/osu/scores/123", nil, provider)
			if !meta.NoIndex || strings.Contains(meta.Title+meta.Description, "Player") || strings.Contains(meta.Description, "secret") {
				t.Fatalf("nonpublic metadata leaked: %+v", meta)
			}
		})
	}
}

func TestOfficialScoreMetadataValidatesPathBeforeLookup(t *testing.T) {
	provider := scoreMetadataFunc(func(context.Context, int64) (osuservice.OfficialScoreDetail, error) {
		t.Fatal("invalid ID reached provider")
		return osuservice.OfficialScoreDetail{}, nil
	})
	for _, id := range []string{"0", "-1", "+123", "0123", "123/more", "https://evil.test", "9223372036854775808"} {
		meta := resolvePageMeta(context.Background(), "/osu/scores/"+id, "https://aimmod.app/osu/scores/"+id, nil, provider)
		if !meta.NoIndex {
			t.Errorf("invalid score indexed: %s", id)
		}
	}
	if meta := resolvePageMeta(context.Background(), "/osu/scores/123", "https://aimmod.app/osu/scores/123", nil); !meta.NoIndex {
		t.Fatal("missing provider indexed")
	}
}

func TestOfficialScoreMetadataTimeout(t *testing.T) {
	provider := scoreMetadataFunc(func(ctx context.Context, _ int64) (osuservice.OfficialScoreDetail, error) {
		<-ctx.Done()
		return publicMetadataFixture(), nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	meta := resolvePageMeta(ctx, "/osu/scores/123", "https://aimmod.app/osu/scores/123", nil, provider)
	if !meta.NoIndex {
		t.Fatal("expired lookup indexed")
	}
}

func TestOfficialScoreMetadataDoesNotInventPP(t *testing.T) {
	provider := scoreMetadataFunc(func(context.Context, int64) (osuservice.OfficialScoreDetail, error) {
		detail := publicMetadataFixture()
		detail.Item.PerformancePoints = nil
		return detail, nil
	})
	meta := resolvePageMeta(context.Background(), "/osu/scores/123", "https://aimmod.app/osu/scores/123", nil, provider)
	if meta.NoIndex || strings.Contains(meta.Description, "PP") {
		t.Fatalf("missing PP fabricated or public score hidden: %+v", meta)
	}
}

func TestPrivateRouteNeverUsesOfficialScoreProvider(t *testing.T) {
	provider := scoreMetadataFunc(func(context.Context, int64) (osuservice.OfficialScoreDetail, error) {
		t.Fatal("private page requested official metadata")
		return publicMetadataFixture(), nil
	})
	for _, route := range []string{"/account", "/admin", "/osu/replays/private"} {
		meta := resolvePageMeta(context.Background(), route, "https://aimmod.app"+route, nil, provider)
		if !meta.NoIndex {
			t.Errorf("private route indexed: %s", route)
		}
	}
}

func TestOfficialScoreSSRTypedNilServerRemainsUnavailable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(`<html><head><title>Default</title></head><body>App</body></html>`), 0600); err != nil {
		t.Fatal(err)
	}
	// Match server.go's optional concrete server passed through the interface.
	var server *osuservice.Server
	r := httptest.NewRecorder()
	NewSPAHandler(dir, nil, "https://aimmod.app", server).ServeHTTP(r, httptest.NewRequest("GET", "/osu/scores/123", nil))
	head := readBrandHead(t, r.Body.String())
	if r.Code != 200 || r.Header().Get("X-Robots-Tag") != "noindex, nofollow" || len(head["robots"]) != 1 || head["robots"][0] != "noindex, nofollow" {
		t.Fatalf("typed-nil provider did not fail closed: status=%d head=%v", r.Code, head)
	}
	if len(head["og:title"]) != 1 || head["og:title"][0] != "osu! score unavailable · AimMod Hub" {
		t.Fatalf("typed-nil provider fabricated score metadata: %v", head)
	}
}
