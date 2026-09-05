package httpserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type releaseTransport func(*http.Request) (*http.Response, error)

func (f releaseTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOsuChannelUsesPublishedPointer(t *testing.T) {
	for _, body := range []string{`{"schemaVersion":1,"product":"aimmod-osu","channel":"stable","version":"0.1.4","tag":"aimmod-osu-v0.1.4"}`, `<html>Not a manifest</html>`, `{"schemaVersion":1,"product":"aimmod-osu","channel":"preview","version":"0.1.4","tag":"aimmod-osu-v0.1.4"}`} {
		client := &http.Client{Transport: releaseTransport(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != "https://github.com/veryCrunchy/aimmod/releases/download/aimmod-osu-stable/aimmod-osu-stable.json" {
				t.Fatal(r.URL)
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})}
		w := httptest.NewRecorder()
		osuReleaseChannelHandler(client).ServeHTTP(w, httptest.NewRequest("GET", "/api/osu/v1/releases/stable", nil))
		if strings.Contains(body, `"channel":"stable"`) {
			if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
				t.Fatal(w.Code, w.Body.String())
			}
		} else if w.Code != 502 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(w.Code)
		}
	}
}

func TestOsuChannelLiveManifest(t *testing.T) {
	if os.Getenv("AIMMOD_RELEASE_LIVE") != "1" {
		t.Skip("explicit live check")
	}
	w := httptest.NewRecorder()
	newOsuReleaseChannelHandler().ServeHTTP(w, httptest.NewRequest("GET", "/api/osu/v1/releases/stable", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"installers"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	t.Log("Published channel manifest retrieved with platform installers")
}
