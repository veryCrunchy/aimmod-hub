package httpserver

import (
	"context"
	"encoding/xml"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeSitemapStore struct {
	counts        map[string]int64
	paths         []string
	err           error
	kind          string
	limit, offset int
}

func (s *fakeSitemapStore) SitemapCounts(context.Context) (map[string]int64, error) {
	return s.counts, s.err
}
func (s *fakeSitemapStore) SitemapPaths(_ context.Context, kind string, limit, offset int) ([]string, error) {
	s.kind, s.limit, s.offset = kind, limit, offset
	return s.paths, s.err
}

func TestSitemapIndexPaginatesAllPublicFamilies(t *testing.T) {
	s := &fakeSitemapStore{counts: map[string]int64{"profiles": 5001, "runs": 10000, "osu-replays": 1, "osu-profiles": 0}}
	r := httptest.NewRecorder()
	newSitemapHandler("https://example.com/", s).ServeHTTP(r, httptest.NewRequest("GET", "/sitemap.xml", nil))
	var index sitemapIndex
	if err := xml.Unmarshal(r.Body.Bytes(), &index); err != nil {
		t.Fatal(err)
	}
	if r.Code != 200 || index.Xmlns != sitemapNamespace || len(index.Sitemaps) != 6 {
		t.Fatalf("invalid index: %d %+v", r.Code, index)
	}
	for i, want := range []string{"pages.xml", "osu-replays-1.xml", "profiles-1.xml", "profiles-2.xml", "runs-1.xml", "runs-2.xml"} {
		if index.Sitemaps[i].Loc != "https://example.com/sitemaps/"+want {
			t.Fatal(index.Sitemaps[i])
		}
	}
}

func TestSitemapShardEncodingPaginationAndDeduplication(t *testing.T) {
	s := &fakeSitemapStore{paths: []string{"/profiles/Example%20Player%26Co", "/profiles/Example%20Player%26Co", "//other.example/path", "/search?q=test"}}
	r := httptest.NewRecorder()
	newSitemapHandler("https://example.com", s).ServeHTTP(r, httptest.NewRequest("GET", "/sitemaps/profiles-2.xml", nil))
	var set sitemapURLSet
	if err := xml.Unmarshal(r.Body.Bytes(), &set); err != nil {
		t.Fatal(err)
	}
	if s.kind != "profiles" || s.limit != 5000 || s.offset != 5000 || len(set.URLs) != 1 || set.URLs[0].Loc != "https://example.com/profiles/Example%20Player%26Co" {
		t.Fatalf("bad shard: %+v %+v", s, set)
	}
	if set.URLs[0].LastMod != "" {
		t.Fatal("invented modification date")
	}
}

func TestSitemapErrorsAreNotCachedAsEmptySuccess(t *testing.T) {
	for _, path := range []string{"/sitemap.xml", "/sitemaps/profiles-1.xml"} {
		r := httptest.NewRecorder()
		newSitemapHandler("", &fakeSitemapStore{err: errors.New("database offline")}).ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		if r.Code != 503 || r.Header().Get("Cache-Control") != "no-store" || r.Header().Get("Retry-After") == "" {
			t.Fatal(r)
		}
	}
}

func TestSitemapRejectsInvalidAndEmptyShards(t *testing.T) {
	for _, path := range []string{"/sitemaps/private-1.xml", "/sitemaps/profiles-0.xml", "/sitemaps/profiles-01.xml", "/sitemaps/profiles-50001.xml", "/sitemaps/profiles-1.xml/extra", "/sitemaps/profiles-1.xml"} {
		r := httptest.NewRecorder()
		newSitemapHandler("", &fakeSitemapStore{}).ServeHTTP(r, httptest.NewRequest("GET", path, nil))
		if r.Code != 404 {
			t.Fatalf("%s: %d", path, r.Code)
		}
	}
}

func TestSitemapMethodsAndStaticFallback(t *testing.T) {
	h := newSitemapHandler("https://example.com", &fakeSitemapStore{})
	for _, path := range []string{"/sitemap.xml", "/sitemaps/pages.xml"} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest("POST", path, nil))
		if r.Code != 405 {
			t.Fatal(r.Code)
		}
	}
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest("HEAD", "/sitemap.xml", nil))
	if r.Code != 200 || r.Body.Len() != 0 {
		t.Fatal(r)
	}
	r = httptest.NewRecorder()
	newSitemapHandler("https://example.com", nil).ServeHTTP(r, httptest.NewRequest("GET", "/sitemaps/pages.xml", nil))
	if r.Code != 200 || !strings.Contains(r.Body.String(), "https://example.com/osu/learn/") {
		t.Fatal(r)
	}
	r = httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest("HEAD", "/sitemaps/pages.xml", nil))
	if r.Code != 200 || r.Body.Len() != 0 {
		t.Fatal(r)
	}
}

func TestPlayerHandlesWithDotsArePages(t *testing.T) {
	for _, route := range []string{"profiles/example.player", "profiles/example.player/benchmarks", "osu/profiles/example.player", "u/kovaaks/example.player"} {
		if isStaticAssetPath(route) {
			t.Fatalf("profile served as missing asset: %s", route)
		}
	}
	h := staticTestHandler(t)
	r := httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest("GET", "/u/kovaaks/Example%20Player?filter=x", nil))
	if !strings.Contains(r.Body.String(), `href="https://aimmod.app/u/kovaaks/Example%20Player"`) {
		t.Fatal("canonical lost path escaping or retained query")
	}
	r = httptest.NewRecorder()
	h.ServeHTTP(r, httptest.NewRequest("GET", "/not-a-real-page", nil))
	if r.Code != 404 || r.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
		t.Fatal("unknown route is a soft 404")
	}
}

func TestProfileRouteBoundariesAndUnavailableDetails(t *testing.T) {
	for _, path := range []string{"/profiles/example", "/profiles/example/benchmarks", "/profiles/example/benchmarks/123", "/profiles/example/scenarios/example-map", "/scenarios/example-map/", "/runs/example-run"} {
		meta := resolvePageMeta(context.Background(), path, "https://example.com"+path, nil)
		if !meta.NoIndex {
			t.Fatalf("unavailable data indexed: %s", path)
		}
	}
	for _, path := range []string{"/profiles/example/invalid", "/profiles/example/benchmarks/0", "/profiles/example/scenarios/map/extra"} {
		if reProfile.MatchString(path) {
			t.Fatalf("unknown profile page matched: %s", path)
		}
	}
}
