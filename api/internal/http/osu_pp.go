package httpserver

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// The headless worker uses the same pinned official ruleset as AimMod native.
func newOsuPpHandler() http.Handler {
	target, _ := url.Parse("http://127.0.0.1:5192")
	return newOsuPpProxy(target)
}

func newOsuPpProxy(target *url.URL) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{ResponseHeaderTimeout: 25 * time.Second}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "PP calculation is temporarily unavailable. Please retry.", http.StatusServiceUnavailable)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
		r.URL.Path = "/calculate"
		r.URL.RawQuery = ""
		w.Header().Set("Cache-Control", "no-store")
		proxy.ServeHTTP(w, r)
	})
}
