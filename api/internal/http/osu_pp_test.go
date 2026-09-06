package httpserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestOsuPpProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/calculate" || r.URL.RawQuery != "" {
			t.Errorf("unexpected upstream route")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != `{"lazer":false}` {
			t.Errorf("calculation inputs were changed")
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"pp":123,"engine":"example"}`)
	}))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)
	handler := newOsuPpProxy(target)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/osu/v1/pp/calculate?ignored=true", strings.NewReader(`{"lazer":false}`)))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"pp":123`) || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("calculation response not forwarded")
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/osu/v1/pp/calculate", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatal("GET accepted")
	}
	upstream.Close()
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/osu/v1/pp/calculate", strings.NewReader(`{}`)))
	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), upstream.URL) {
		t.Fatal("unavailable worker did not fail safely")
	}
}
