package httpserver

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

func newProfileSubdomainRedirectHandler(cfg Config, st *store.Store, next http.Handler) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	if st == nil {
		return next
	}

	targetOrigin := strings.TrimRight(strings.TrimSpace(cfg.WebAppOrigin), "/")
	if targetOrigin == "" {
		return next
	}
	targetURL, err := url.Parse(targetOrigin)
	if err != nil || targetURL.Scheme == "" || targetURL.Host == "" {
		log.Printf("profile subdomain redirects disabled: invalid web origin %q", cfg.WebAppOrigin)
		return next
	}

	subdomainHost := strings.ToLower(strings.TrimSpace(cfg.ProfileSubdomainHost))
	if subdomainHost == "" {
		subdomainHost = strings.ToLower(targetURL.Hostname())
	}
	if subdomainHost == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}

		handle, ok := profileHandleFromRequestHost(r.Host, subdomainHost)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		meta, err := st.GetProfileMeta(r.Context(), handle)
		if err != nil {
			log.Printf("profile subdomain lookup failed for %q on host %q: %v", handle, r.Host, err)
			next.ServeHTTP(w, r)
			return
		}
		if meta == nil || strings.TrimSpace(meta.Handle) == "" {
			next.ServeHTTP(w, r)
			return
		}

		destination := targetOrigin + "/profiles/" + url.PathEscape(meta.Handle)
		if rawQuery := strings.TrimSpace(r.URL.RawQuery); rawQuery != "" {
			destination += "?" + rawQuery
		}
		http.Redirect(w, r, destination, http.StatusFound)
	})
}

func profileHandleFromRequestHost(requestHost, subdomainHost string) (string, bool) {
	host := strings.ToLower(strings.TrimSpace(hostnameOnly(requestHost)))
	subdomainHost = strings.ToLower(strings.TrimSpace(subdomainHost))
	if host == "" || subdomainHost == "" || host == subdomainHost {
		return "", false
	}

	suffix := "." + subdomainHost
	if !strings.HasSuffix(host, suffix) {
		return "", false
	}

	handle := strings.TrimSuffix(host, suffix)
	if handle == "" || strings.Contains(handle, ".") {
		return "", false
	}
	for _, r := range handle {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return "", false
		}
	}
	return handle, true
}

func hostnameOnly(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		return parsedHost
	}
	return strings.Trim(host, "[]")
}
