package osu

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"connectrpc.com/connect"
	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
)

func TestSkinDedupRequiresExactArtifactAndPreservesSources(t *testing.T) {
	direct := func(uri string) *osuv1.SkinDownloadHandoff {
		return &osuv1.SkinDownloadHandoff{Kind: osuv1.SkinDownloadHandoffKind_SKIN_DOWNLOAD_HANDOFF_KIND_DIRECT_URL, Available: true, Uri: uri}
	}
	items := []*osuv1.SkinItem{
		{Provider: 1, SourceId: "Abc1234", Name: "Same name", DownloadHandoff: direct("https://osuskins.net/file.osk?v=1")},
		{Provider: 2, SourceId: "23", Name: "Different name", DownloadHandoff: direct("https://osuskins.net/file.osk?v=1")},
		{Provider: 2, SourceId: "24", Name: "Same name", DownloadHandoff: direct("https://osuskins.net/file.osk?v=2")},
		{Provider: 2, SourceId: "25", Name: "Same name"},
		{Provider: 2, SourceId: "25", Name: "Same name"},
	}
	got := deduplicateSkins(items)
	if len(got) != 3 || len(got[0].Sources) != 2 {
		t.Fatalf("incorrect merge: %v", got)
	}
	if items[0].NormalizedId != "" || len(items[0].Sources) != 0 {
		t.Fatal("mutated input")
	}
	if got[0].NormalizedId == got[1].NormalizedId {
		t.Fatal("variants share identity")
	}
	items[0].Sources = []*osuv1.SkinSource{{Variant: "NM"}, {Variant: "DT"}}
	if len(deduplicateSkins(items[:2])) != 2 {
		t.Fatal("multi-variant skin merged through one archive")
	}
}

func TestSkinExternalHostsAreBrowserOnly(t *testing.T) {
	for _, raw := range []string{"https://drive.google.com/file/d/abc/view", "https://mega.nz/file/abc#secret", "https://mega.nz/#!abc!secret"} {
		if externalSkinBrowserURL(raw) != raw {
			t.Fatalf("lost browser source %q", raw)
		}
	}
	for _, raw := range []string{"http://drive.google.com/file/d/a", "https://drive.google.com.evil.test/file/d/a", "https://user@mega.nz/file/a", "https://mega.nz:444/file/a", "http://127.0.0.1/a.osk", "https://drive.google.com/folders/a"} {
		if externalSkinBrowserURL(raw) != "" {
			t.Fatalf("accepted unsafe/unsupported source %q", raw)
		}
	}
}

func TestSkinArchiveVerificationAndRedirectBoundary(t *testing.T) {
	hits := 0
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++; w.Write([]byte("PK\x03\x04")) }))
	defer foreign.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/good.osk":
			w.Header().Set("Content-Range", "bytes 0-3/1234")
			w.WriteHeader(206)
			w.Write([]byte("PK\x03\x04"))
		case "/redirect.osk":
			http.Redirect(w, r, foreign.URL+"/a.osk", 302)
		case "/large.osk":
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-3/%d", maxSkinArchiveBytes+1))
			w.WriteHeader(206)
			w.Write([]byte("PK\x03\x04"))
		default:
			w.Write([]byte("<html>challenge</html>"))
		}
	}))
	defer upstream.Close()
	client, _ := newUpstreamClient(upstream.URL, upstream.Client(), newResponseCache(0, 0), newIntervalLimiter(10000), "test")
	for _, name := range []string{"good", "redirect", "large", "challenge"} {
		u, _ := url.Parse(upstream.URL + "/" + name + ".osk")
		handoff, err := verifySkinArchive(context.Background(), client, u)
		if name == "good" {
			if err != nil || handoff.ExpectedSizeBytes != 1234 || handoff.Kind != 1 {
				t.Fatalf("good archive: %v %v", handoff, err)
			}
		} else if err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	if hits != 0 {
		t.Fatal("followed cross-origin redirect")
	}
	u, _ := url.Parse(foreign.URL + "/a.osk")
	if _, err := verifySkinArchive(context.Background(), client, u); err == nil {
		t.Fatal("accepted arbitrary origin")
	}
	if _, err := skinUpstreamClient(client).get(context.Background(), "/redirect.osk", nil, ""); err == nil || hits != 0 {
		t.Fatal("catalog followed foreign redirect")
	}
}

func TestSkinOsuckSearchReportsActualFailureAndEscapedBrowserQuery(t *testing.T) {
	queries := []string{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query().Get("query"))
		http.Error(w, "private upstream body", 503)
	}))
	defer upstream.Close()
	server, err := NewServer(Config{OsuckBaseURL: upstream.URL, HTTPClient: upstream.Client(), ProviderRequestsPerSecond: 10000})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		result, err := server.SearchSkins(context.Background(), connect.NewRequest(&osuv1.SearchSkinsRequest{Providers: []osuv1.SkinProvider{2}, Query: "a & b"}))
		if err != nil {
			t.Fatal(err)
		}
		status := result.Msg.Providers[0]
		if !status.Retryable || status.Available || !strings.Contains(status.Message, "503") || strings.Contains(status.Message, "Cloudflare") || strings.Contains(status.Message, "private upstream body") {
			t.Fatalf("wrong status: %v", status)
		}
		u, _ := url.Parse(status.BrowserUrl)
		if u.Query().Get("query") != "a & b" {
			t.Fatal("lost browser query")
		}
	}
	if len(queries) != 2 || queries[0] != "a & b" {
		t.Fatal("search did not retry upstream")
	}
}

func TestSkinSourceIDsCannotInjectPaths(t *testing.T) {
	for _, id := range []string{"../1", "01", "-1", "0", "1?tab=a", "1/2", "https://evil.test"} {
		if skinPageURL(2, id) != "" {
			t.Fatal(id)
		}
	}
	if skinPageURL(2, "23") != "https://skins.osuck.net/skins/23" {
		t.Fatal("valid ID")
	}
}

func TestSkinPublishedSourcesPreserveVariantsAndBoundProbes(t *testing.T) {
	probes := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { probes++; http.Error(w, "missing", 404) }))
	defer upstream.Close()
	client, _ := newUpstreamClient(upstream.URL, upstream.Client(), newResponseCache(0, 0), newIntervalLimiter(10000), "test")
	adapter := newOsuSkinsAdapter(client)
	item := &osuv1.SkinItem{Provider: 1, SourceId: "Abc1234", DownloadHandoff: browserSkinHandoff("https://osuskins.net/skin/Abc1234", "verification")}
	body := `<a href="https://drive.google.com/file/d/one/view">NM</a><a href="https://mega.nz/file/two#key">DT</a><a href="https://evil.test/skin.osk">bad</a>`
	for i := 0; i < 20; i++ {
		body += fmt.Sprintf(`<a href="/%d.osk">missing</a>`, i)
	}
	adapter.resolvePublishedDownloads(context.Background(), []byte(body), item)
	if len(item.Sources) != 3 || item.Sources[1].Variant != "NM" || item.Sources[2].Variant != "DT" || item.Sources[2].DownloadHandoff.Uri != "https://mega.nz/file/two#key" {
		t.Fatalf("lost sources: %v", item.Sources)
	}
	if probes != 6 {
		t.Fatalf("unbounded probes: %d", probes)
	}
	if item.DownloadHandoff.Kind != 3 {
		t.Fatal("silently selected a variant")
	}
}
