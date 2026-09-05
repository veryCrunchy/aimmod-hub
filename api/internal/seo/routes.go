package seo

import "strings"

// IsPrivateRoute is independent of the published catalog so adding metadata
// cannot accidentally make an account or authentication page indexable.
func IsPrivateRoute(route string) bool {
	for _, prefix := range []string{"/admin", "/account", "/link-device", "/auth", "/search"} {
		if route == prefix || strings.HasPrefix(route, prefix+"/") {
			return true
		}
	}
	return false
}
