package httpserver

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// Proxy the authoritative pointer, not the accumulated packages on a channel
// release. GitHub download assets do not expose browser CORS headers.
func newOsuReleaseChannelHandler() http.Handler {
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 || req.URL.Scheme != "https" || (req.URL.Hostname() != "github.com" && req.URL.Hostname() != "release-assets.githubusercontent.com") {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	return osuReleaseChannelHandler(client)
}

func osuReleaseChannelHandler(client *http.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		channel := strings.TrimPrefix(r.URL.Path, "/api/osu/v1/releases/")
		if channel != "stable" && channel != "preview" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", 405)
			return
		}
		req, err := http.NewRequestWithContext(r.Context(), "GET", "https://github.com/veryCrunchy/aimmod/releases/download/aimmod-osu-"+channel+"/aimmod-osu-"+channel+".json", nil)
		if err != nil {
			http.Error(w, "Release information is unavailable.", 502)
			return
		}
		req.Header.Set("Accept", "application/json")
		response, err := client.Do(req)
		if err != nil {
			http.Error(w, "Release information is unavailable.", 502)
			return
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			status := 502
			if response.StatusCode == 404 {
				status = 404
			}
			http.Error(w, "Release information is unavailable.", status)
			return
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, 128*1024+1))
		var manifest struct {
			Schema  int    `json:"schemaVersion"`
			Product string `json:"product"`
			Channel string `json:"channel"`
			Version string `json:"version"`
			Tag     string `json:"tag"`
		}
		if err != nil || len(data) > 128*1024 || json.Unmarshal(data, &manifest) != nil || manifest.Schema != 1 || manifest.Product != "aimmod-osu" || manifest.Channel != channel || manifest.Version == "" || manifest.Tag != "aimmod-osu-v"+manifest.Version {
			http.Error(w, "Release information is unavailable.", 502)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=30, must-revalidate")
		_, _ = w.Write(data)
	})
}
