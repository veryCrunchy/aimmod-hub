package httpserver

import (
	"context"
	"encoding/xml"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/veryCrunchy/aimmod-hub/api/internal/seo"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

type seoStoreStub struct {
	replay  store.OsuPublicReplay
	profile store.OsuPublicProfile
	err     error
}

func (s seoStoreStub) GetOsuPublicReplay(context.Context, string) (store.OsuPublicReplay, error) {
	return s.replay, s.err
}
func (s seoStoreStub) GetOsuPublicProfile(context.Context, string, int) (store.OsuPublicProfile, error) {
	return s.profile, s.err
}

func TestOsuDetailMetadataVisibility(t *testing.T) {
	for _, visibility := range []string{"public", "unlisted", "private", ""} {
		t.Run(visibility, func(t *testing.T) {
			m := resolveOsuDetailMeta(context.Background(), "/osu/replays/example", "https://aimmod.app/osu/replays/example", seoStoreStub{replay: store.OsuPublicReplay{Visibility: visibility, Title: "Sensitive title", Artist: "Artist", Difficulty: "Hard", OsuUsername: "Player", Accuracy: .975}})
			if visibility == "public" {
				if m.NoIndex || !strings.Contains(m.Title, "Sensitive title") || !strings.Contains(m.Description, "97.50%") {
					t.Fatalf("public metadata: %+v", m)
				}
			} else if !m.NoIndex || strings.Contains(m.Title+m.Description, "Sensitive title") || strings.Contains(m.Description, "Player") {
				t.Fatalf("share leaked metadata: %+v", m)
			}
		})
	}
	missing := resolveOsuDetailMeta(context.Background(), "/osu/replays/missing", "https://aimmod.app/osu/replays/missing", seoStoreStub{err: errors.New("not found")})
	if !missing.NoIndex {
		t.Fatal("missing replay indexed")
	}
	profile := resolveOsuDetailMeta(context.Background(), "/osu/profiles/player", "https://aimmod.app/osu/profiles/player", seoStoreStub{profile: store.OsuPublicProfile{OsuUsername: "Player", SharedReplayCount: 3}})
	if profile.NoIndex || profile.OGType != "profile" || !strings.Contains(profile.Description, "3 public") {
		t.Fatalf("profile metadata: %+v", profile)
	}
}

func TestSEOPrivateRoutesAndCanonical(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(`<html><head><meta name="robots" content="index"></head><body>App</body></html>`), 0600); err != nil {
		t.Fatal(err)
	}
	handler := NewSPAHandler(dir, nil, "https://aimmod.app/")
	for _, route := range []string{"/admin", "/admin/coaching", "/account", "/auth/callback", "/link-device", "/search", "/osu/replays/private", "/osu/learn/not-a-guide", "/not-a-page"} {
		r := httptest.NewRecorder()
		handler.ServeHTTP(r, httptest.NewRequest("GET", route+"/?secret=excluded", nil))
		m := readBrandHead(t, r.Body.String())
		if route == "/osu/learn/not-a-guide" && r.Code != 404 {
			t.Fatalf("missing guide returned %d instead of 404", r.Code)
		}
		if len(m["robots"]) != 1 || m["robots"][0] != "noindex, nofollow" || r.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
			t.Fatalf("%s missing noindex: %v", route, m)
		}
		if len(m["canonical"]) != 1 || m["canonical"][0] != "https://aimmod.app"+route {
			t.Fatalf("%s canonical: %v", route, m["canonical"])
		}
	}
}

func TestSEOSitemapIncludesPublishedGuidesAndExcludesPrivateRoutes(t *testing.T) {
	r := httptest.NewRecorder()
	newStaticSitemapHandler("https://aimmod.app/").ServeHTTP(r, httptest.NewRequest("GET", "/sitemap.xml", nil))
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	var sitemap sitemapURLSet
	if err := xml.Unmarshal(r.Body.Bytes(), &sitemap); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, item := range sitemap.URLs {
		if seen[item.Loc] {
			t.Fatalf("duplicate %s", item.Loc)
		}
		seen[item.Loc] = true
		if strings.Contains(item.Loc, "/admin") || strings.Contains(item.Loc, "/account") || strings.Contains(item.Loc, "/search") || strings.Contains(item.Loc, "/osu/replays/") {
			t.Fatalf("private or search route listed: %s", item.Loc)
		}
	}
	for _, route := range []string{"/osu", "/osu/beatmaps", "/osu/skins", "/osu/learn", "/osu/pp-targets"} {
		if !seen["https://aimmod.app"+route] {
			t.Fatal("missing", route)
		}
	}
	for _, guide := range seo.Published.Guides {
		if !seen["https://aimmod.app/osu/learn/"+guide.Slug] {
			t.Fatal("missing guide", guide.Slug)
		}
		m := resolvePageMeta(context.Background(), "/osu/learn/"+guide.Slug, "https://aimmod.app/osu/learn/"+guide.Slug, nil)
		if m.NoIndex || m.OGType != "article" || m.Description != guide.Description {
			t.Fatalf("guide metadata %+v", m)
		}
	}
}

func TestPpTargetsServerMetadata(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(`<html><head><title>Default</title><meta name="robots" content="noindex"></head><body>App</body></html>`), 0600); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	NewSPAHandler(dir, nil, "https://aimmod.app").ServeHTTP(r, httptest.NewRequest("GET", "/osu/pp-targets/?accuracy=99&mods=HD", nil))
	head := readBrandHead(t, r.Body.String())
	for key, want := range map[string]string{
		"canonical":           "https://aimmod.app/osu/pp-targets",
		"og:url":              "https://aimmod.app/osu/pp-targets",
		"og:title":            seo.Published.Routes["/osu/pp-targets"].Title,
		"twitter:description": seo.Published.Routes["/osu/pp-targets"].Description,
		"og:image":            "https://aimmod.app/social-preview.png?path=%2Fosu%2Fpp-targets&v=1",
		"robots":              "index, follow",
	} {
		if len(head[key]) != 1 || head[key][0] != want {
			t.Errorf("%s = %v, want %q", key, head[key], want)
		}
	}
	if r.Header().Get("X-Robots-Tag") != "" {
		t.Fatal("public PP targets blocked")
	}
}

func TestClientPublicDetailRoutesRemainIndexable(t *testing.T) {
	for _, route := range []string{"/benchmarks/example", "/u/123", "/u/123/benchmarks/example", "/u/kovaaks/player"} {
		meta := resolvePageMeta(context.Background(), route, "https://aimmod.app"+route, nil)
		if meta.NoIndex {
			t.Errorf("public client detail blocked: %s", route)
		}
	}
}

func TestPrerenderedPrivateRouteCannotBypassNoindex(t *testing.T) {
	dir := t.TempDir()
	document := []byte(`<html><head><title>Account</title><meta name="robots" content="index, follow"></head><body>App</body></html>`)
	if err := os.WriteFile(filepath.Join(dir, "index.html"), document, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "account"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "account", "index.html"), document, 0600); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	NewSPAHandler(dir, nil, "https://aimmod.app").ServeHTTP(r, httptest.NewRequest("GET", "/account/", nil))
	head := readBrandHead(t, r.Body.String())
	if len(head["robots"]) != 1 || head["robots"][0] != "noindex, nofollow" || r.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
		t.Fatalf("private prerender indexed: %v", head)
	}
}
