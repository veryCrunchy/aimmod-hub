package httpserver

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Matches Obiente's /join flow: resolve the widget invite, retaining the last success.
func newDiscordJoinHandler(endpoint string) http.Handler {
	client := &http.Client{Timeout: 8 * time.Second}
	var mu sync.Mutex
	var lastSavedInvite string
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
		invite := ""
		if err == nil {
			resp, fetchErr := client.Do(req)
			if fetchErr == nil {
				defer resp.Body.Close()
				var widget struct {
					Invite string `json:"instant_invite"`
				}
				if resp.StatusCode == http.StatusOK && json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&widget) == nil && validDiscordInvite(widget.Invite) {
					invite = widget.Invite
				}
			}
		}
		mu.Lock()
		if invite != "" {
			lastSavedInvite = invite
		} else {
			invite = lastSavedInvite
		}
		mu.Unlock()
		if invite == "" {
			w.Header().Set("Retry-After", "30")
			http.Error(w, "Discord couldn't fetch an invite, please try again later.", http.StatusServiceUnavailable)
			return
		}
		http.Redirect(w, r, invite, http.StatusFound)
	})
}

func validDiscordInvite(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	return (u.Host == "discord.gg" && len(strings.Trim(u.Path, "/")) > 0) ||
		(u.Host == "discord.com" && strings.HasPrefix(u.Path, "/invite/") && len(strings.TrimPrefix(u.Path, "/invite/")) > 0)
}
