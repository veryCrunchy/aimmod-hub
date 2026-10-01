package httpserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func inviteRequest(h http.Handler, query, method, etag string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, "/og/invite.png?"+query, nil)
	if etag != "" {
		request.Header.Set("If-None-Match", etag)
	}
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	return response
}

func syntheticArtwork() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 640, 360))
	for y := 0; y < 360; y++ {
		for x := 0; x < 640; x++ {
			img.Set(x, y, color.RGBA{uint8(x / 3), uint8(y / 2), 180, 255})
		}
	}
	return img
}

const inviteQuery = "v=1&mode=cs&map=Synthetic+Arena&n=2&max=6&state=lobby&host=synthetic-host"

func TestInviteCardRendersBothLayoutsAndCaches(t *testing.T) {
	h := newInviteCardHandler(nil)
	for layout, size := range map[string]image.Point{"banner": {1280, 720}, "square": {1024, 1024}} {
		first := inviteRequest(h, inviteQuery+"&layout="+layout, "GET", "")
		if first.Code != 200 || first.Header().Get("Content-Type") != "image/png" || !strings.HasPrefix(first.Header().Get("Cache-Control"), "public") {
			t.Fatalf("%s: %d %v", layout, first.Code, first.Header())
		}
		decoded, err := png.Decode(bytes.NewReader(first.Body.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Bounds().Size() != size {
			t.Fatalf("%s: %v", layout, decoded.Bounds())
		}
		if decoded.At(5, 4) == decoded.At(size.X/2, size.Y/2) {
			t.Fatalf("%s: blank card", layout)
		}
		second := inviteRequest(h, inviteQuery+"&layout="+layout, "GET", "")
		if !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
			t.Fatal("non-deterministic card")
		}
		if inviteRequest(h, inviteQuery+"&layout="+layout, "GET", "W/"+first.Header().Get("ETag")).Code != 304 {
			t.Fatal("conditional response")
		}
		head := inviteRequest(h, inviteQuery+"&layout="+layout, "HEAD", "")
		if head.Code != 200 || head.Body.Len() != 0 || head.Header().Get("Content-Length") == "" {
			t.Fatal("HEAD response")
		}
	}
	if h.lru.Len() != 2 {
		t.Fatal("cache entries", h.lru.Len())
	}
	// Another player count is another card.
	other := inviteRequest(h, strings.Replace(inviteQuery, "n=2", "n=3", 1), "GET", "")
	if other.Code != 200 || other.Header().Get("ETag") == inviteRequest(h, inviteQuery, "GET", "").Header().Get("ETag") {
		t.Fatal("player count not reflected")
	}
}

func TestInviteCardRejectsUnknownAndOutOfRangeParameters(t *testing.T) {
	h := newInviteCardHandler(nil)
	bad := []string{
		"",
		"mode=cs&n=2&max=6",                        // no version
		"v=2&mode=cs&n=2&max=6",                    // unknown version
		"v=1&mode=custom-text&n=2&max=6",           // modes are keys, not free text
		"v=1&mode=cs&n=7&max=6",                    // more players than slots
		"v=1&mode=cs&n=0&max=6",                    // nobody
		"v=1&mode=cs&n=2&max=11",                   // over the limit
		"v=1&mode=cs&n=2&max=1",                    // under the limit
		"v=1&mode=cs&n=-1&max=6",                   // signs
		"v=1&mode=cs&n=2&max=6&state=secret",       // unknown state
		"v=1&mode=cs&n=2&max=6&layout=poster",      // unknown layout
		"v=1&mode=cs&n=2&max=6&title=x",            // unknown key
		"v=1&mode=cs&n=2&max=6&url=https://x.test", // no caller URLs
		"v=1&mode=cs&mode=duel&n=2&max=6",          // duplicates
		"v=1&mode=cs&n=2&max=6&host=Upper",         // not a hub handle
		"v=1&mode=cs&n=2&max=6&host=a--b",          // not a hub handle
		"v=1&mode=cs&n=2&max=6&host=" + strings.Repeat("a", 33),
		"v=1&mode=cs&n=2&max=6&ws=12ab",                          // workshop ids are digits
		"v=1&mode=cs&n=2&max=6&ws=0123",                          // no leading zero
		"v=1&mode=cs&n=2&max=6&map=" + url.QueryEscape("a\x00b"), // controls
		"v=1&mode=cs&n=2&max=6&map=" + strings.Repeat("m", 97),
		"v=1&mode=cs&n=2&max=6&map=" + strings.Repeat("m", 1100),
	}
	for _, query := range bad {
		if result := inviteRequest(h, query, "GET", ""); result.Code != 400 || result.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%q: %d", query, result.Code)
		}
	}
	if inviteRequest(h, inviteQuery, "POST", "").Code != 405 {
		t.Fatal("mutation method allowed")
	}
	for _, query := range []string{"v=1&mode=practice&n=1&max=2", "v=1&mode=score-race&n=10&max=10&state=results&map=" + url.QueryEscape("初音ミク Arena")} {
		if result := inviteRequest(h, query, "GET", ""); result.Code != 200 {
			t.Errorf("%q: %d", query, result.Code)
		}
	}
}

func TestInviteCardUsesWorkshopArtworkAndShortCacheOnMiss(t *testing.T) {
	var calls atomic.Int32
	art := func(_ context.Context, item string) (image.Image, error) {
		calls.Add(1)
		if item == "1001" {
			return syntheticArtwork(), nil
		}
		return nil, errors.New("unavailable")
	}
	h := newInviteCardHandler(art)
	plain := inviteRequest(h, inviteQuery, "GET", "")
	with := inviteRequest(h, inviteQuery+"&ws=1001", "GET", "")
	missing := inviteRequest(h, inviteQuery+"&ws=2002", "GET", "")
	if plain.Code != 200 || with.Code != 200 || missing.Code != 200 {
		t.Fatal(plain.Code, with.Code, missing.Code)
	}
	if bytes.Equal(plain.Body.Bytes(), with.Body.Bytes()) {
		t.Fatal("artwork not drawn")
	}
	if !bytes.Equal(plain.Body.Bytes(), missing.Body.Bytes()) {
		t.Fatal("a missing preview should fall back to the plain card")
	}
	if with.Header().Get("Cache-Control") != "public, max-age=86400" || missing.Header().Get("Cache-Control") != "public, max-age=600" {
		t.Fatal(with.Header().Get("Cache-Control"), missing.Header().Get("Cache-Control"))
	}
	if calls.Load() != 2 {
		t.Fatal("artwork lookups", calls.Load())
	}
}

// A Steam stand-in: GetPublishedFileDetails plus an image host. The client
// rewrites the allowed Steam image host to the test server.
func steamStub(t *testing.T, details string, image []byte) (*workshopArtworkCache, *atomic.Int32) {
	var hits atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/details":
			if r.Method != http.MethodPost || r.FormValue("publishedfileids[0]") == "" || r.FormValue("itemcount") != "1" {
				http.Error(w, "bad", 400)
				return
			}
			fmt.Fprint(w, details)
		case "/ugc/preview":
			_, _ = w.Write(image)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	target, _ := url.Parse(server.URL)
	client := server.Client()
	base := client.Transport
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if workshopImageHosts[r.URL.Hostname()] {
			r = r.Clone(r.Context())
			r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
		}
		return base.RoundTrip(r)
	})
	return newWorkshopArtwork(server.URL+"/details", client), &hits
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func syntheticPNG(t *testing.T, w, h int) []byte {
	var out bytes.Buffer
	if err := png.Encode(&out, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func details(id string, app int, preview string, visibility int) string {
	return fmt.Sprintf(`{"response":{"result":1,"resultcount":1,"publishedfiledetails":[{"publishedfileid":%q,"result":1,"consumer_app_id":%d,"preview_url":%q,"visibility":%d,"banned":0}]}}`, id, app, preview, visibility)
}

func TestWorkshopArtworkFetchesOnlyPublicKovaaksPreviews(t *testing.T) {
	good := syntheticPNG(t, 64, 36)
	cache, hits := steamStub(t, details("1001", kovaaksSteamApp, "https://images.steamusercontent.com/ugc/preview", 0), good)
	img, err := cache.Get(context.Background(), "1001")
	if err != nil || img.Bounds().Dx() != 64 {
		t.Fatal(err)
	}
	if _, err := cache.Get(context.Background(), "1001"); err != nil || hits.Load() != 2 {
		t.Fatal("preview not cached", hits.Load())
	}
	for name, body := range map[string]string{
		"other game": details("1001", 730, "https://images.steamusercontent.com/ugc/preview", 0),
		"not public": details("1001", kovaaksSteamApp, "https://images.steamusercontent.com/ugc/preview", 1),
		"other host": details("1001", kovaaksSteamApp, "https://example.test/ugc/preview", 0),
		"plain http": details("1001", kovaaksSteamApp, "http://images.steamusercontent.com/ugc/preview", 0),
		"other item": details("9999", kovaaksSteamApp, "https://images.steamusercontent.com/ugc/preview", 0),
		"not json":   "<html>",
		"no files":   `{"response":{"publishedfiledetails":[]}}`,
		"banned":     strings.Replace(details("1001", kovaaksSteamApp, "https://images.steamusercontent.com/ugc/preview", 0), `"banned":0`, `"banned":1`, 1),
	} {
		cache, _ := steamStub(t, body, good)
		if _, err := cache.Get(context.Background(), "1001"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	huge, _ := steamStub(t, details("1001", kovaaksSteamApp, "https://images.steamusercontent.com/ugc/preview", 0), syntheticPNG(t, workshopPreviewPixels+1, 2))
	if _, err := huge.Get(context.Background(), "1001"); err == nil {
		t.Error("oversized image accepted")
	}
	notImage, _ := steamStub(t, details("1001", kovaaksSteamApp, "https://images.steamusercontent.com/ugc/preview", 0), []byte("GIF89a not really"))
	if _, err := notImage.Get(context.Background(), "1001"); err == nil {
		t.Error("non-image accepted")
	}
}

func TestWorkshopArtworkRetriesMissesLater(t *testing.T) {
	cache, hits := steamStub(t, `{"response":{"publishedfiledetails":[]}}`, nil)
	now := time.Unix(1_700_000_000, 0)
	cache.now = func() time.Time { return now }
	_, _ = cache.Get(context.Background(), "1001")
	_, _ = cache.Get(context.Background(), "1001")
	if hits.Load() != 1 {
		t.Fatal("miss not cached", hits.Load())
	}
	now = now.Add(11 * time.Minute)
	_, _ = cache.Get(context.Background(), "1001")
	if hits.Load() != 2 {
		t.Fatal("miss not retried", hits.Load())
	}
	for i := 0; i < 200; i++ {
		_, _ = cache.Get(context.Background(), fmt.Sprint(5000+i))
	}
	if len(cache.entries) > 128 || len(cache.order) > 128 {
		t.Fatal("artwork cache unbounded")
	}
}

func TestInviteCardArtifacts(t *testing.T) {
	dir := os.Getenv("INVITE_CARD_OUTPUT_DIR")
	if dir == "" {
		t.Skip("set INVITE_CARD_OUTPUT_DIR to export review artifacts")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, layout := range []string{"banner", "square"} {
		for name, art := range map[string]image.Image{"plain": nil, "art": syntheticArtwork()} {
			card, err := parseInviteCard(inviteQuery + "&layout=" + layout)
			if err != nil {
				t.Fatal(err)
			}
			data, err := renderInviteCard(card, art)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(dir, layout+"-"+name+".png"), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
