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
	"path"
	"regexp"
	"strings"

	htmlparser "golang.org/x/net/html"

	"github.com/veryCrunchy/aimmod-hub/api/internal/coaching"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

var (
	reProfile    = regexp.MustCompile(`^/profiles/([^/]+)`)
	reScenario   = regexp.MustCompile(`^/scenarios/([^/]+)$`)
	reRun        = regexp.MustCompile(`^/runs/([^/]+)$`)
	reLearn      = regexp.MustCompile(`^/learn/([^/]+)$`)
	reLearnTopic = regexp.MustCompile(`^/learn/topics/([^/]+)$`)
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
    <meta property="og:image" content="https://aimmod.app/brand/aimmod-v9/share-card-1200x630.png" />
    <meta property="og:image:width" content="1200" />
    <meta property="og:image:height" content="630" />
    <meta property="og:image:type" content="image/png" />
    <meta property="og:image:alt" content="AimMod wordmark and forward-leaning monogram" />
    <meta name="twitter:card" content="summary_large_image" />
    <meta name="twitter:image" content="https://aimmod.app/brand/aimmod-v9/share-card-1200x630.png" />
    <meta name="twitter:image:alt" content="AimMod wordmark and forward-leaning monogram" />
    <meta name="twitter:title" content="%s" />
    <meta name="twitter:description" content="%s" />`,
		t, d, c, t, d, html.EscapeString(m.OGType), c, t, d,
	)
	// Replace owned head metadata rather than appending conflicting defaults.
	// Tokenization preserves unrelated markup, scripts and favicon links verbatim.
	z := htmlparser.NewTokenizer(strings.NewReader(indexHTML))
	var output strings.Builder
	inHead, inTitle := false, false
	for {
		kind := z.Next()
		if kind == htmlparser.ErrorToken {
			return output.String()
		}
		raw := string(z.Raw())
		token := z.Token()
		if kind == htmlparser.StartTagToken && token.Data == "head" {
			inHead = true
		}
		if inHead {
			if kind == htmlparser.EndTagToken && token.Data == "head" {
				output.WriteString(block)
				inHead = false
			}
			if token.Data == "title" && kind == htmlparser.StartTagToken {
				inTitle = true
				continue
			}
			if inTitle {
				if kind == htmlparser.EndTagToken && token.Data == "title" {
					inTitle = false
				}
				continue
			}
			if kind == htmlparser.StartTagToken || kind == htmlparser.SelfClosingTagToken {
				owned := false
				for _, attr := range token.Attr {
					value := strings.ToLower(strings.TrimSpace(attr.Val))
					if token.Data == "meta" && (attr.Key == "name" || attr.Key == "property") {
						owned = owned || value == "description" || strings.HasPrefix(value, "og:") || strings.HasPrefix(value, "twitter:")
					}
					if token.Data == "link" && attr.Key == "rel" {
						for _, rel := range strings.Fields(value) {
							owned = owned || rel == "canonical"
						}
					}
				}
				if owned {
					continue
				}
			}
		}
		output.WriteString(raw)
	}
}

func isStaticAssetPath(p string) bool {
	if p == "" {
		return false
	}
	if strings.HasPrefix(p, "assets/") {
		return true
	}
	if path.Ext(p) != "" {
		return true
	}
	switch p {
	case "runtime-config.js", "favicon.ico", "robots.txt", "manifest.json":
		return true
	default:
		return false
	}
}

func resolvePageMeta(ctx context.Context, path, canonical string, st *store.Store) pageMeta {
	fallback := pageMeta{
		Title:       "AimMod Hub",
		Description: "AimMod analysis and coaching for osu! and KovaaK's, plus shared practice data.",
		OGType:      "website",
		Canonical:   canonical,
	}

	cleanPath := strings.TrimSuffix(path, "/")
	if cleanPath == "/osu" || strings.HasPrefix(cleanPath, "/osu/") || cleanPath == "/app/osu" {
		fallback.Title = "osu! Analysis and Community · AimMod Hub"
		fallback.Description = "Explore osu! beatmaps, skins, player profiles and shared replays with AimMod analysis and coaching."
		switch cleanPath {
		case "/osu/beatmaps":
			fallback.Title = "osu! Beatmaps · AimMod Hub"
			fallback.Description = "Browse osu! beatmaps and difficulties, filter map statistics, and open maps in AimMod."
		case "/osu/skins":
			fallback.Title = "osu! Skins · AimMod Hub"
			fallback.Description = "Browse osu! skins, inspect previews, and find a skin for your next play."
		case "/osu/players":
			fallback.Title = "osu! Players · AimMod Hub"
		case "/osu/replays":
			fallback.Title = "osu! Replay Library · AimMod Hub"
		case "/osu/community":
			fallback.Title = "osu! Community · AimMod Hub"
		case "/app/osu":
			fallback.Title = "Download AimMod for osu! · AimMod Hub"
			fallback.Description = "Download AimMod for osu! on Windows and Linux for replay analysis, coaching and practice."
		}
		return fallback
	}

	if m := reProfile.FindStringSubmatch(path); m != nil {
		meta, err := st.GetProfileMeta(ctx, m[1])
		if err != nil || meta == nil {
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

	if m := reLearn.FindStringSubmatch(path); m != nil {
		entry, err := coaching.GetLearnEntry(m[1])
		if err != nil {
			return pageMeta{
				Title:       "Aim Training Guide · AimMod Hub",
				Description: fallback.Description,
				OGType:      "article",
				Canonical:   canonical,
			}
		}
		return pageMeta{
			Title:       fmt.Sprintf("%s · AimMod Learn", entry.Entry.Title),
			Description: entry.Entry.Summary,
			OGType:      "article",
			Canonical:   canonical,
		}
	}

	if m := reLearnTopic.FindStringSubmatch(path); m != nil {
		topic, err := coaching.GetLearnTopic(m[1])
		if err != nil {
			return pageMeta{
				Title:       "Aim Training Topic · AimMod Hub",
				Description: fallback.Description,
				OGType:      "website",
				Canonical:   canonical,
			}
		}
		return pageMeta{
			Title:       fmt.Sprintf("%s Aim Training Guides · AimMod Learn", topic.Title),
			Description: topic.Description,
			OGType:      "website",
			Canonical:   canonical,
		}
	}

	switch path {
	case "/app", "/app/":
		return pageMeta{
			Title:       "Download AimMod · AimMod Hub",
			Description: "Download AimMod for osu! or KovaaK's. Replay analysis, coaching and practice tools for your game.",
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
	case "/learn", "/learn/":
		return pageMeta{
			Title:       "Aim Training Learning Pages · AimMod Hub",
			Description: "Browse AimMod's coaching knowledge as learning pages covering aim flaws, mechanics, scenarios, sensitivity, and improvement methods.",
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

		// Serve hashed assets and other real files directly without preflight
		// fs.Stat() checks so asset requests can't fail closed with a 500.
		if isStaticAssetPath(p) {
			fileServer.ServeHTTP(w, r)
			return
		}

		if p != "" {
			indexPath := path.Join(p, "index.html")
			rawRouteHTML, readErr := fs.ReadFile(fsys, indexPath)
			if readErr == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Header().Set("Cache-Control", "no-store")
				_, _ = w.Write(rawRouteHTML)
				return
			}
			if !errors.Is(readErr, fs.ErrNotExist) {
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
