package httpserver

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type playbackRoundTripper func(*http.Request) (*http.Response, error)

func (f playbackRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPlaybackMapFetchIsExactCachedAndConditional(t *testing.T) {
	h := newOsuPlaybackHandler()
	calls := 0
	data := "osu file format v14\r\n[Metadata]\r\nBeatmapID:42\r\n"
	h.client.Transport = playbackRoundTripper(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://osu.ppy.sh/osu/42" {
			t.Errorf("unbounded source %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(data))}, nil
	})
	first := httptest.NewRecorder()
	h.ServeHTTP(first, httptest.NewRequest("GET", osuPlaybackMapPrefix+"42/file", nil))
	if first.Code != 200 || first.Body.String() != data {
		t.Fatalf("map changed: %d %q", first.Code, first.Body.String())
	}
	second := httptest.NewRecorder()
	request := httptest.NewRequest("GET", osuPlaybackMapPrefix+"42/file", nil)
	request.Header.Set("If-None-Match", first.Header().Get("ETag"))
	h.ServeHTTP(second, request)
	if second.Code != 304 || second.Body.Len() != 0 || calls != 1 {
		t.Fatalf("cache not reused: %d calls%d", second.Code, calls)
	}
	head := httptest.NewRecorder()
	h.ServeHTTP(head, httptest.NewRequest("HEAD", osuPlaybackMapPrefix+"42/file", nil))
	if head.Code != 200 || head.Body.Len() != 0 || head.Header().Get("Content-Length") == "" {
		t.Fatal("invalid HEAD")
	}
}

func TestPlaybackMapRejectsURLsInvalidIDsAndMethodsWithoutNetwork(t *testing.T) {
	h := newOsuPlaybackHandler()
	h.client.Transport = playbackRoundTripper(func(*http.Request) (*http.Response, error) {
		t.Error("unexpected upstream request")
		return nil, io.EOF
	})
	for _, path := range []string{"0/file", "-1/file", "0042/file", "9999999999999/file", "42/file?url=https://evil.test", "https://evil.test/file", "42/audio"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", osuPlaybackMapPrefix+path, nil))
		if w.Code < 400 {
			t.Errorf("accepted %s", path)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", osuPlaybackMapPrefix+"42/file", nil))
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
}

func TestPlaybackMapChecksumSeparatesRevisionsAndLegacyCache(t *testing.T) {
	h := newOsuPlaybackHandler()
	data := "osu file format v14\r\n[Metadata]\r\nVersion:Old\r\n"
	calls := 0
	h.client.Transport = playbackRoundTripper(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://osu.ppy.sh/osu/42" {
			t.Errorf("unexpected upstream URL %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(data))}, nil
	})
	fetch := func(query string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", osuPlaybackMapPrefix+"42/file"+query, nil))
		return w
	}
	old := data
	oldChecksum := fmt.Sprintf("%x", md5.Sum([]byte(old)))
	if w := fetch(""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := fetch("?checksum=" + oldChecksum); w.Code != 200 || calls != 2 {
		t.Fatal("checksum request reused legacy cache", w.Code, calls)
	}
	data = strings.ReplaceAll(old, "Old", "Updated")
	newChecksum := fmt.Sprintf("%x", md5.Sum([]byte(data)))
	w := fetch("?checksum=" + strings.ToUpper(newChecksum))
	if w.Code != 200 || w.Body.String() != data || calls != 3 {
		t.Fatal("new revision reused stale file", w.Code, calls)
	}
	if w = fetch("?checksum=" + newChecksum); w.Code != 200 || calls != 3 {
		t.Fatal("validated revision not cached", w.Code, calls)
	}
	if w = fetch(""); w.Body.String() != old || calls != 3 {
		t.Fatal("legacy cache behavior changed")
	}
	if w = fetch("?checksum=" + oldChecksum); w.Body.String() != old || calls != 3 {
		t.Fatal("old verified revision lost")
	}
}

func TestPlaybackMapChecksumMismatchIsNotCachedAndCanRetry(t *testing.T) {
	h := newOsuPlaybackHandler()
	wanted := "osu file format v14\n[Metadata]\nVersion:Expected\n"
	data := strings.ReplaceAll(wanted, "Expected", "Wrong")
	calls := 0
	h.client.Transport = playbackRoundTripper(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(data))}, nil
	})
	path := osuPlaybackMapPrefix + "42/file?checksum=" + fmt.Sprintf("%x", md5.Sum([]byte(wanted)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	if w.Code != http.StatusConflict || w.Header().Get("Cache-Control") != "no-store" || len(h.cache) != 0 {
		t.Fatal("mismatched bytes served or cached", w.Code)
	}
	data = wanted
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	if w.Code != 200 || w.Body.String() != wanted || calls != 2 {
		t.Fatal("mismatch prevented fresh retry", w.Code, calls)
	}
}

func TestPlaybackMapRejectsMalformedChecksumQueries(t *testing.T) {
	h := newOsuPlaybackHandler()
	h.client.Transport = playbackRoundTripper(func(*http.Request) (*http.Response, error) {
		t.Error("invalid query reached upstream")
		return nil, io.EOF
	})
	valid := strings.Repeat("a", 32)
	for _, query := range []string{
		"checksum=", "checksum=abc", "checksum=" + strings.Repeat("g", 32),
		"checksum=" + valid + "&checksum=" + valid,
		"checksum=" + valid + "&extra=1", "extra=" + valid,
		"checksum=%zz", "checksum=" + valid + ";extra=1",
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", osuPlaybackMapPrefix+"42/file?"+query, nil))
		if w.Code != http.StatusBadRequest {
			t.Errorf("accepted query %q: %d", query, w.Code)
		}
	}
}

func TestPlaybackMapRejectsHTMLRedirectAndOversizedResponses(t *testing.T) {
	for _, response := range []struct {
		status int
		body   string
	}{
		{200, "<html>login</html>"}, {302, "redirect"}, {200, "osu file format v14" + strings.Repeat("x", osuPlaybackMapLimit)},
	} {
		h := newOsuPlaybackHandler()
		h.client.Transport = playbackRoundTripper(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: response.status, Body: io.NopCloser(strings.NewReader(response.body))}, nil
		})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", osuPlaybackMapPrefix+"42/file", nil))
		if w.Code != 502 || len(h.cache) != 0 {
			t.Fatalf("accepted invalid map: %d", w.Code)
		}
	}
}

func TestPlaybackMapCacheEvictsOldestAndExpiredEntries(t *testing.T) {
	h := newOsuPlaybackHandler()
	now := time.Now()
	h.remember("expired", osuPlaybackMap{data: []byte("old"), expires: now.Add(-time.Second), accessed: now})
	if _, ok := h.cached("expired"); ok || h.cacheBytes != 0 {
		t.Fatal("expired cache retained")
	}
	for i := 0; i < osuPlaybackCacheEntries+1; i++ {
		key := strings.Repeat("x", i+1)
		h.remember(key, osuPlaybackMap{data: []byte("map"), expires: now.Add(time.Hour), accessed: now.Add(time.Duration(i) * time.Second)})
	}
	if len(h.cache) != osuPlaybackCacheEntries || h.cacheBytes != 3*osuPlaybackCacheEntries {
		t.Fatal("entry bound exceeded")
	}
	if _, ok := h.cache["x"]; ok {
		t.Fatal("oldest entry was not evicted")
	}
}
