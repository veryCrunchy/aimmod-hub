package httpserver

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

var (
	reProfile  = regexp.MustCompile(`^/profiles/([^/]+)`)
	reScenario = regexp.MustCompile(`^/scenarios/([^/]+)$`)
	reRun      = regexp.MustCompile(`^/runs/([^/]+)$`)
)

type pageMeta struct {
	Title       string
	Description string
	OGType      string
	Canonical   string
}

func (m pageMeta) inject(indexHTML string) string {
	t := html.EscapeString(m.Title)
	d := html.EscapeString(m.Description)
	c := html.EscapeString(m.Canonical)
	block := fmt.Sprintf(
		`<title>%s</title>
    <meta name="description" content="%s" />
    <link rel="canonical" href="%s" />
    <meta property="og:title" content="%s" />
    <meta property="og:description" content="%s" />
    <meta property="og:type" content="%s" />
    <meta property="og:url" content="%s" />
    <meta property="og:site_name" content="AimMod Hub" />
    <meta name="twitter:card" content="summary" />
    <meta name="twitter:title" content="%s" />
    <meta name="twitter:description" content="%s" />`,
		t, d, c, t, d, m.OGType, c, t, d,
	)
	return strings.Replace(indexHTML, "<title>AimMod Hub</title>", block, 1)
}

func resolvePageMeta(ctx context.Context, path, canonical string, st *store.Store) pageMeta {
	fallback := pageMeta{
		Title:       "AimMod Hub",
		Description: "Shared KovaaK's practice data. View scenario pages, player profiles, and run history.",
		OGType:      "website",
		Canonical:   canonical,
	}

	if m := reProfile.FindStringSubmatch(path); m != nil {
		meta, err := st.GetProfileMeta(ctx, m[1])
		if err != nil {
			return pageMeta{Title: m[1] + " · AimMod Hub", Description: fallback.Description, OGType: "profile", Canonical: canonical}
		}
		name := meta.DisplayName
		if name == "" {
			name = meta.Handle
		}
		return pageMeta{
			Title:       fmt.Sprintf("%s (@%s) · AimMod Hub", name, meta.Handle),
			Description: fmt.Sprintf("%d runs across %d scenarios on AimMod Hub.", meta.RunCount, meta.ScenarioCount),
			OGType:      "profile",
			Canonical:   canonical,
		}
	}

	if m := reScenario.FindStringSubmatch(path); m != nil {
		meta, err := st.GetScenarioMeta(ctx, m[1])
		if err != nil {
			return pageMeta{Title: m[1] + " · AimMod Hub", Description: fallback.Description, OGType: "website", Canonical: canonical}
		}
		return pageMeta{
			Title:       fmt.Sprintf("%s · AimMod Hub", meta.Name),
			Description: fmt.Sprintf("%d runs · Best score %.0f · Avg accuracy %.1f%%", meta.RunCount, meta.BestScore, meta.AvgAcc),
			OGType:      "website",
			Canonical:   canonical,
		}
	}

	if m := reRun.FindStringSubmatch(path); m != nil {
		meta, err := st.GetRunMeta(ctx, m[1])
		if err != nil {
			return fallback
		}
		name := meta.UserDisplayName
		if name == "" {
			name = meta.UserHandle
		}
		return pageMeta{
			Title:       fmt.Sprintf("%s by %s · AimMod Hub", meta.ScenarioName, name),
			Description: fmt.Sprintf("Score: %.0f · Accuracy: %.1f%%", meta.Score, meta.Accuracy),
			OGType:      "website",
			Canonical:   canonical,
		}
	}

	switch path {
	case "/app", "/app/":
		return pageMeta{
			Title:       "Download AimMod · AimMod Hub",
			Description: "KovaaK's overlay, replay, and coaching suite. Download the latest release for Windows.",
			OGType:      "website",
			Canonical:   canonical,
		}
	case "/community", "/community/":
		return pageMeta{
			Title:       "Community · AimMod Hub",
			Description: "Explore the AimMod Hub community — top scenarios, active players, and recent runs.",
			OGType:      "website",
			Canonical:   canonical,
		}
	case "/leaderboard", "/leaderboard/":
		return pageMeta{
			Title:       "Leaderboard · AimMod Hub",
			Description: "The top scores across all scenarios on AimMod Hub.",
			OGType:      "website",
			Canonical:   canonical,
		}
	case "/search", "/search/":
		return pageMeta{
			Title:       "Search · AimMod Hub",
			Description: "Search for players, scenarios, and runs across AimMod Hub.",
			OGType:      "website",
			Canonical:   canonical,
		}
	}

	return fallback
}

func NewSPAHandler(dir string, st *store.Store, origin string) http.Handler {
	raw, err := os.ReadFile(dir + "/index.html")
	if err != nil {
		log.Fatalf("spa: read index.html from %q: %v", dir, err)
	}
	indexHTML := string(raw)

	fsys := os.DirFS(dir)
	fileServer := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")

		// Serve static assets that exist on disk directly.
		if p != "" {
			_, err := fs.Stat(fsys, p)
			if err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
			if !errors.Is(err, fs.ErrNotExist) {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}

		// SPA fallback: serve index.html with injected meta.
		canonical := origin + r.URL.Path
		meta := resolvePageMeta(r.Context(), r.URL.Path, canonical, st)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(meta.inject(indexHTML)))
	})
}
