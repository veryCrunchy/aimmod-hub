package httpserver

import (
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func newLLMAssetHandler(root string) http.Handler {
	root = filepath.Clean(root)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		rel := strings.TrimPrefix(r.URL.Path, "/llm")
		rel = strings.TrimPrefix(rel, "/")
		rel = path.Clean("/" + rel)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" || rel == "." {
			http.NotFound(w, r)
			return
		}

		fullPath := filepath.Join(root, filepath.FromSlash(rel))
		if !isWithinRoot(root, fullPath) {
			http.NotFound(w, r)
			return
		}

		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}

		setLLMAssetHeaders(w, fullPath)
		http.ServeFile(w, r, fullPath)
	})
}

func isWithinRoot(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func setLLMAssetHeaders(w http.ResponseWriter, filename string) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".json":
		w.Header().Set("Cache-Control", "no-cache")
	case ".zip", ".gguf", ".exe", ".dll":
		w.Header().Set("Cache-Control", "public, max-age=3600, immutable")
	default:
		w.Header().Set("Cache-Control", "public, max-age=300")
	}

	if contentType := mime.TypeByExtension(ext); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
}
