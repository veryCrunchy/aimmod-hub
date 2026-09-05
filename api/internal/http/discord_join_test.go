package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscordJoinResolvesAndRetainsInvite(t *testing.T) {
	body := `{"instant_invite":"https://discord.gg/example"}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
	defer upstream.Close()
	handler := newDiscordJoinHandler(upstream.URL)
	for _, payload := range []string{body, `{"instant_invite":null}`, `invalid`, `{"instant_invite":"https://example.com/"}`} {
		body = payload
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/join", nil))
		if w.Code != 302 || w.Header().Get("Location") != "https://discord.gg/example" || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("unexpected redirect: %d %v", w.Code, w.Header())
		}
	}
}

func TestDiscordJoinUnavailableAndMethods(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer upstream.Close()
	handler := newDiscordJoinHandler(upstream.URL)
	for method, status := range map[string]int{"GET": 503, "POST": 405} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(method, "/join", nil))
		if w.Code != status {
			t.Fatalf("%s: %d", method, w.Code)
		}
	}
}
