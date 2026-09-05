package httpserver

import (
	"net/http"
	"path"
	"regexp"
	"strings"
)

// Vite's default build emits name-[hash] under assets/, with 8-character hashes.
// Public files outside this naming convention may change at the same URL.
var spaImmutableAsset = regexp.MustCompile(`^assets/(?:[^/]+/)*[^/]+-[A-Za-z0-9_-]{8}\.(?:js|mjs|css|wasm|woff2?|ttf|otf|png|jpe?g|gif|webp|avif|svg|ico)$`)

type spaStaticResponse struct {
	http.ResponseWriter
	assetPath   string
	wroteHeader bool
}

func (w *spaStaticResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *spaStaticResponse) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	// Apply policy at commitment, after FileServer knows whether the file exists.
	// Never give a missing chunk JS MIME or a long-lived negative cache entry.
	h.Set("Cache-Control", "no-store")
	if status >= 200 && status < 300 || status == http.StatusNotModified {
		ext := strings.ToLower(path.Ext(w.assetPath))
		if ext != ".html" && ext != ".htm" {
			h.Set("Cache-Control", "no-cache")
			if spaImmutableAsset.MatchString(w.assetPath) {
				h.Set("Cache-Control", "public, max-age=31536000, immutable")
			}
		}
		// Do not depend on OS MIME registries for module/worker/WASM loading.
		if status != http.StatusNotModified && !strings.HasPrefix(h.Get("Content-Type"), "multipart/byteranges") {
			switch ext {
			case ".js", ".mjs":
				h.Set("Content-Type", "text/javascript; charset=utf-8")
			case ".css":
				h.Set("Content-Type", "text/css; charset=utf-8")
			case ".wasm":
				h.Set("Content-Type", "application/wasm")
			case ".html", ".htm":
				h.Set("Content-Type", "text/html; charset=utf-8")
			}
		}
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *spaStaticResponse) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}
