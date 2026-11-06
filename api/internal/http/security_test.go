package httpserver

import (
	"github.com/veryCrunchy/aimmod-hub/api/internal/service"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestReturnToLocalOnly(t *testing.T) {
	for _, value := range []string{"https://example.com", "//example.com", "/%2fexample.com", "/\\example.com", "/%0d%0aLocation:bad", "relative"} {
		if validReturnTo(value) {
			t.Errorf("accepted %q", value)
		}
	}
	if !validReturnTo("/account?tab=links") {
		t.Fatal("local path rejected")
	}
}
func TestCallbackRequiresBrowserNonce(t *testing.T) {
	h := &authHandler{}
	w := httptest.NewRecorder()
	h.handleDiscordCallback(w, httptest.NewRequest("GET", "/auth/discord/callback?code=synthetic&state=synthetic", nil))
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}
func TestOversizedBody(t *testing.T) {
	w := httptest.NewRecorder()
	_, err := io.Copy(io.Discard, http.MaxBytesReader(w, io.NopCloser(strings.NewReader("12345")), 4))
	bodyError(w, err)
	if w.Code != 413 {
		t.Fatal(w.Code)
	}
}

func TestBatchEndpointLimits(t *testing.T) {
	mux := NewMux(Config{MediaDir: t.TempDir()}, service.NewHubServer("test", nil))
	for _, body := range []string{`{"sessions":[` + strings.Repeat(`{},`, 100) + `{}]}`, `{"sessions":[],"padding":"` + strings.Repeat("x", int(maxIngestBytes)) + `"}`, `{"sessions":[]}` + strings.Repeat(" ", int(maxIngestBytes))} {
		w := httptest.NewRecorder()
		request := httptest.NewRequest("POST", "/ingest/batch", strings.NewReader(body))
		request.ContentLength = -1
		mux.ServeHTTP(w, request)
		if w.Code != 413 {
			t.Fatalf("got %d", w.Code)
		}
	}
}
func TestSteamAssertionBindings(t *testing.T) {
	target := "https://example.com/auth/steam/callback?state=synthetic"
	q := url.Values{"openid.ns": {openIDNS}, "openid.mode": {"id_res"}, "openid.op_endpoint": {steamEndpoint}, "openid.return_to": {target}, "openid.claimed_id": {"https://steamcommunity.com/openid/id/00000000000000001"}, "openid.identity": {"https://steamcommunity.com/openid/id/00000000000000001"}, "openid.response_nonce": {time.Now().UTC().Format(time.RFC3339) + "synthetic"}, "openid.signed": {"op_endpoint,claimed_id,identity,return_to,response_nonce,assoc_handle"}}
	if _, ok := validateSteamAssertion(q, target); !ok {
		t.Fatal("valid assertion rejected")
	}
	q.Set("openid.return_to", "https://example.org")
	if _, ok := validateSteamAssertion(q, target); ok {
		t.Fatal("foreign callback accepted")
	}
	q.Set("openid.return_to", target)
	q.Set("openid.signed", "identity")
	if _, ok := validateSteamAssertion(q, target); ok {
		t.Fatal("unsigned claims accepted")
	}
}

func TestVideoEndpointRejectsOversizedLength(t *testing.T) {
	r := httptest.NewRequest("POST", "/media/replays/upload", strings.NewReader("video"))
	r.ContentLength = maxVideoBytes + 1
	w := httptest.NewRecorder()
	(&authHandler{}).handleReplayMediaUpload(w, r)
	if w.Code != 413 {
		t.Fatal(w.Code)
	}
}
