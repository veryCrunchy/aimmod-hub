package seo

import "testing"

func TestPrivateRouteBoundaries(t *testing.T) {
	for _, route := range []string{"/account", "/account/", "/admin/coaching", "/auth/callback", "/link-device", "/search/"} {
		if !IsPrivateRoute(route) {
			t.Errorf("private route not protected: %s", route)
		}
	}
	for _, route := range []string{"/osu/pp-targets", "/replays", "/osu/replays", "/osu/replays/public", "/administrator"} {
		if IsPrivateRoute(route) {
			t.Errorf("public route treated as private: %s", route)
		}
	}
}

func TestPublishedRouteMetadataIsDistinctAndPublic(t *testing.T) {
	titles, descriptions := map[string]string{}, map[string]string{}
	for route, page := range Published.Routes {
		if IsPrivateRoute(route) {
			// Search has descriptive metadata but is explicitly excluded from indexing.
			if route != "/search" {
				t.Errorf("private route in published catalog: %s", route)
			}
			continue
		}
		if previous, ok := titles[page.Title]; ok {
			t.Errorf("duplicate title on %s and %s", previous, route)
		}
		if previous, ok := descriptions[page.Description]; ok {
			t.Errorf("duplicate description on %s and %s", previous, route)
		}
		titles[page.Title], descriptions[page.Description] = route, route
	}
}
