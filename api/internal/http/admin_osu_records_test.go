package httpserver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

func TestAdminOsuRecordFilters(t *testing.T) {
	for _, path := range []string{"players", "beatmaps"} {
		kind := "synced"
		if path == "beatmaps" {
			kind = "online"
		}
		for _, tc := range []struct {
			query string
			want  int
		}{
			{"?q=Map&offset=25&kind=" + kind, 200}, {"?kind=invalid", 400}, {"?offset=-5", 400}, {"?offset=1000001", 400}, {"?offset=bad", 400}, {"?q=" + strings.Repeat("a", 201), 400},
		} {
			s := &fakeAdminOsuStore{}
			h := &adminOsuHandler{store: s, isAdmin: func(store.AuthUser) bool { return true }}
			r := httptest.NewRequest("GET", "/admin/osu/"+path+tc.query, nil)
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("%s%s: %d", path, tc.query, w.Code)
			}
			if tc.want == 400 && s.calls != 0 {
				t.Fatal("invalid request queried store")
			}
			if tc.want == 200 && (s.recordFilter.Kind != kind || s.recordFilter.Search != "Map" || s.recordFilter.Limit != 25 || s.recordFilter.Offset != 25) {
				t.Fatalf("incorrect filter %+v", s.recordFilter)
			}
		}
		for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
			s := &fakeAdminOsuStore{}
			h := &adminOsuHandler{store: s, isAdmin: func(store.AuthUser) bool { return true }}
			r := httptest.NewRequest(method, "/admin/osu/"+path, nil)
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 405 || s.calls != 0 {
				t.Fatal("read-only records accepted mutation")
			}
		}
		s := &fakeAdminOsuStore{dataErr: errors.New("SECRET query details")}
		h := &adminOsuHandler{store: s, isAdmin: func(store.AuthUser) bool { return true }}
		r := httptest.NewRequest("GET", "/admin/osu/"+path, nil)
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 500 || strings.Contains(w.Body.String(), "SECRET") {
			t.Fatal("unsafe error")
		}
	}
}
