package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func staticTestHandler(t *testing.T) http.Handler {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"index.html":                  "<!doctype html><html><head></head><body>SPA_SENTINEL</body></html>",
		"assets/page-Ab12_cdE.js":     "export const loaded = true;",
		"assets/worker-1234abcd.mjs":  "export {};",
		"assets/style-Ab123456.css":   "body { color: red; }",
		"assets/engine-12345678.wasm": "\x00asm\x01\x00\x00\x00",
		"assets/unhashed.js":          "export {};",
		"assets/not-a-hash.js":        "export {};",
		"assets/page-Ab12_cdE.html":   "<html>ASSET_HTML</html>",
		"runtime-config.js":           "window.config = {};",
		"brand/mark-12345678.svg":     "<svg/>",
		"osu/beatmaps/index.html":     "<html>PRERENDERED</html>",
	} {
		file := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return NewSPAHandler(dir, nil, "https://aimmod.app")
}

func TestSPAStaticSuccessfulAssetMIMEAndCaching(t *testing.T) {
	h := staticTestHandler(t)
	for _, tc := range []struct{ path, mime, cache string }{
		{"/assets/page-Ab12_cdE.js", "text/javascript; charset=utf-8", "public, max-age=31536000, immutable"},
		{"/assets/worker-1234abcd.mjs", "text/javascript; charset=utf-8", "public, max-age=31536000, immutable"},
		{"/assets/style-Ab123456.css", "text/css; charset=utf-8", "public, max-age=31536000, immutable"},
		{"/assets/engine-12345678.wasm", "application/wasm", "public, max-age=31536000, immutable"},
		{"/assets/unhashed.js", "text/javascript; charset=utf-8", "no-cache"},
		{"/assets/not-a-hash.js", "text/javascript; charset=utf-8", "no-cache"},
		{"/runtime-config.js", "text/javascript; charset=utf-8", "no-cache"},
		{"/brand/mark-12345678.svg", "image/svg+xml", "no-cache"},
		{"/assets/page-Ab12_cdE.html", "text/html; charset=utf-8", "no-store"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			for _, method := range []string{"GET", "HEAD"} {
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest(method, tc.path, nil))
				if w.Code != 200 || w.Header().Get("Content-Type") != tc.mime || w.Header().Get("Cache-Control") != tc.cache || w.Header().Get("X-Content-Type-Options") != "nosniff" {
					t.Fatalf("%s: status=%d headers=%v", method, w.Code, w.Header())
				}
				if method == "HEAD" && w.Body.Len() != 0 {
					t.Fatal("HEAD body")
				}
			}
		})
	}
}

func TestSPAStaticMissingChunksAreUncached404NotHTMLOrJS(t *testing.T) {
	h := staticTestHandler(t)
	for _, url := range []string{"/assets/oldpage-12345678.js", "/assets/oldpage-12345678.js?retry=1", "/assets/gone-12345678.css", "/assets/gone-12345678.wasm", "/assets/missing-worker", "/missing.js", "/missing.css", "/missing.wasm"} {
		for _, method := range []string{"GET", "HEAD"} {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(method, url, nil)
			r.Header.Set("Accept", "text/html")
			h.ServeHTTP(w, r)
			if w.Code != 404 || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
				t.Fatalf("%s %s: status=%d headers=%v", method, url, w.Code, w.Header())
			}
			if strings.Contains(w.Body.String(), "SPA_SENTINEL") {
				t.Fatal("missing asset fell back to SPA")
			}
		}
	}
}

func TestSPAStaticConditionalAndRangePolicies(t *testing.T) {
	h := staticTestHandler(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/assets/page-Ab12_cdE.js", nil))
	modified := w.Header().Get("Last-Modified")
	if modified == "" {
		t.Fatal("missing Last-Modified")
	}
	r := httptest.NewRequest("GET", "/assets/page-Ab12_cdE.js", nil)
	r.Header.Set("If-Modified-Since", modified)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 304 || w.Body.Len() != 0 || !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
		t.Fatal("conditional policy", w.Code, w.Header())
	}
	for _, tc := range []struct {
		value  string
		status int
		cache  string
	}{
		{"bytes=0-5", 206, "public, max-age=31536000, immutable"},
		{"bytes=999999-", 416, "no-store"},
	} {
		r = httptest.NewRequest("GET", "/assets/page-Ab12_cdE.js", nil)
		r.Header.Set("Range", tc.value)
		w = httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status || w.Header().Get("Cache-Control") != tc.cache {
			t.Fatal("range policy", w.Code, w.Header())
		}
		if tc.status == 416 && w.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
			t.Fatal("error disguised as module")
		}
	}
}

func TestSPAHTMLAndRedirectsAreNotStored(t *testing.T) {
	h := staticTestHandler(t)
	for _, url := range []string{"/", "/osu/beatmaps", "/osu/players", "/index.html", "/osu/beatmaps/index.html"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(url, w.Code, w.Header())
		}
		if w.Code == 200 && !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
			t.Fatal("HTML MIME", url)
		}
	}
}
