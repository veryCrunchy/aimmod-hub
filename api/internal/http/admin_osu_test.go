package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
)

type fakeAdminOsuStore struct {
	user       store.AuthUser
	sessionErr error
	dataErr    error
	calls      int
	filter     store.AdminOsuFilter
}

func (s *fakeAdminOsuStore) GetUserBySession(context.Context, string) (store.AuthUser, error) {
	return s.user, s.sessionErr
}
func (s *fakeAdminOsuStore) GetAdminOsuOverview(_ context.Context, f store.AdminOsuFilter) (store.AdminOsuOverview, error) {
	s.calls++
	s.filter = f
	return store.AdminOsuOverview{Items: []store.AdminOsuShare{{ID: 1, Visibility: "private", Status: "pending"}}, Total: 1}, s.dataErr
}

type fakeAdminProviders struct{ calls int }

func (p *fakeAdminProviders) GetProviderStatus(context.Context, *connect.Request[osuv1.GetProviderStatusRequest]) (*connect.Response[osuv1.GetProviderStatusResponse], error) {
	p.calls++
	return connect.NewResponse(&osuv1.GetProviderStatusResponse{Providers: []*osuv1.ProviderStatus{{Provider: osuv1.Provider_PROVIDER_OSU_OFFICIAL, Message: "SECRET upstream response", Configured: true}}}), nil
}
func (p *fakeAdminProviders) GetSkinProviderStatus(context.Context, *connect.Request[osuv1.GetSkinProviderStatusRequest]) (*connect.Response[osuv1.GetSkinProviderStatusResponse], error) {
	p.calls++
	return connect.NewResponse(&osuv1.GetSkinProviderStatusResponse{Providers: []*osuv1.SkinProviderStatus{{Provider: osuv1.SkinProvider_SKIN_PROVIDER_OSUCK, Message: "SECRET upstream response"}}}), nil
}

func TestAdminOsuAccess(t *testing.T) {
	for _, path := range []string{"/admin/osu/overview", "/admin/osu/providers"} {
		for _, tc := range []struct {
			name, cookie, discord, configured string
			sessionErr                        error
			want                              int
		}{
			{"anonymous", "", "", "admin", nil, 401},
			{"expired", "session", "admin", "admin", errors.New("expired"), 401},
			{"non-admin", "session", "member", "admin", nil, 403},
			{"unconfigured", "session", "admin", "", nil, 403},
			{"admin", "session", "admin", "admin", nil, 200},
		} {
			t.Run(path+tc.name, func(t *testing.T) {
				s := &fakeAdminOsuStore{user: store.AuthUser{DiscordUserID: tc.discord}, sessionErr: tc.sessionErr}
				p := &fakeAdminProviders{}
				auth := &authHandler{cfg: Config{AdminDiscordUserID: tc.configured}}
				h := &adminOsuHandler{store: s, isAdmin: auth.isAdminUser, providers: p}
				r := httptest.NewRequest(http.MethodGet, path, nil)
				if tc.cookie != "" {
					r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: tc.cookie})
				}
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if w.Code != tc.want {
					t.Fatalf("got %d: %s", w.Code, w.Body)
				}
				if tc.want != 200 && (s.calls != 0 || p.calls != 0) {
					t.Fatal("unauthorized data access")
				}
				if w.Header().Get("Cache-Control") != "private, no-store" {
					t.Fatal("admin response must not be cached")
				}
				if strings.Contains(w.Body.String(), "SECRET") {
					t.Fatal("raw provider message leaked")
				}
				if tc.want == 200 && path == "/admin/osu/providers" && !strings.Contains(w.Body.String(), `"browserOnly":true`) {
					t.Fatal("osuck must be browser-only")
				}
			})
		}
	}
}

func TestAdminOsuFiltersAndErrors(t *testing.T) {
	for _, tc := range []struct {
		query string
		want  int
	}{
		{"?q=Player&visibility=private&status=pending&offset=25", 200},
		{"?visibility=secret", 400}, {"?status=failed", 400}, {"?offset=-1", 400}, {"?offset=1000001", 400}, {"?offset=abc", 400}, {"?q=" + strings.Repeat("a", 201), 400},
	} {
		s := &fakeAdminOsuStore{}
		h := &adminOsuHandler{store: s, isAdmin: func(store.AuthUser) bool { return true }}
		r := httptest.NewRequest("GET", "/admin/osu/overview"+tc.query, nil)
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s: %d", tc.query, w.Code)
		}
		if tc.want == 200 && (s.filter.Limit != 25 || s.filter.Offset != 25 || s.filter.Search != "Player" || s.filter.Visibility != "private" || s.filter.Status != "pending") {
			t.Fatalf("wrong filter: %+v", s.filter)
		}
		if tc.want == 400 && s.calls != 0 {
			t.Fatal("invalid filter reached database")
		}
	}
	s := &fakeAdminOsuStore{dataErr: errors.New("SECRET SQL details")}
	h := &adminOsuHandler{store: s, isAdmin: func(store.AuthUser) bool { return true }}
	r := httptest.NewRequest("GET", "/admin/osu/overview", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 500 || strings.Contains(w.Body.String(), "SECRET") {
		t.Fatal("unsafe database error response")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/admin/osu/overview", nil))
	if w.Code != 405 {
		t.Fatal("mutation accepted")
	}
}
