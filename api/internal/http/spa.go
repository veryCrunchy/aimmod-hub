package httpserver

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	htmlparser "golang.org/x/net/html"

	"github.com/veryCrunchy/aimmod-hub/api/internal/coaching"
	osuservice "github.com/veryCrunchy/aimmod-hub/api/internal/osu"
	"github.com/veryCrunchy/aimmod-hub/api/internal/seo"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

var (
	reProfile      = regexp.MustCompile(`^/profiles/([^/]+)(?:/benchmarks(?:/([1-9][0-9]*))?|/scenarios/([^/]+))?$`)
	reScenario     = regexp.MustCompile(`^/scenarios/([^/]+)$`)
	reRun          = regexp.MustCompile(`^/runs/([^/]+)$`)
	reLearn        = regexp.MustCompile(`^/learn/([^/]+)$`)
	reLearnTopic   = regexp.MustCompile(`^/learn/topics/([^/]+)$`)
	reSocialDetail = regexp.MustCompile(`^/(?:osu/(?:learn|replays|profiles)|learn(?:/topics)?|profiles|runs|scenarios)/[^/]+$`)
	reClientDetail = regexp.MustCompile(`^/(?:benchmarks/[^/]+|u/[^/]+(?:/benchmarks/[^/]+)?|u/kovaaks/[^/]+)$`)
)

type pageMeta struct {
	Title       string
	Description string
	OGType      string
	Canonical   string
	NoIndex     bool
	NotFound    bool
}

type publicScoreMetadataProvider interface {
	GetPublicScore(context.Context, int64) (osuservice.OfficialScoreDetail, error)
}

func resolveOfficialScoreMeta(ctx context.Context, route, canonical string, provider publicScoreMetadataProvider) pageMeta {
	meta := pageMeta{Title: "osu! score unavailable · AimMod Hub", Description: "This official osu! score is unavailable.", OGType: "website", Canonical: canonical, NoIndex: true}
	rawID := strings.TrimPrefix(route, "/osu/scores/")
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != rawID || provider == nil {
		return meta
	}
	// Reuse the public application-credential provider and its cache. Do not fetch
	// replay bytes or user-authorized/private score data to construct metadata.
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	detail, err := provider.GetPublicScore(ctx, id)
	if err != nil || ctx.Err() != nil || detail.Status != "available" || detail.Item == nil {
		return meta
	}
	item := detail.Item
	if item.Source != "official" || item.Visibility != store.OsuVisibilityPublic || item.OnlineScoreID != id || item.OfficialScoreID != rawID || item.OsuUserID <= 0 || item.BeatmapID <= 0 {
		return meta
	}
	player := strings.TrimSpace(item.OsuUsername)
	if player == "" {
		player = fmt.Sprintf("osu! player %d", item.OsuUserID)
	}
	beatmap := strings.TrimSpace(item.Title)
	if beatmap == "" {
		beatmap = fmt.Sprintf("Beatmap %d", item.BeatmapID)
	}
	if item.Difficulty != "" {
		beatmap += " [" + item.Difficulty + "]"
	}
	meta.Title = fmt.Sprintf("%s on %s · osu! score %d · AimMod Hub", player, beatmap, id)
	meta.Description = fmt.Sprintf("Official osu! score by %s on %s.", player, beatmap)
	if !math.IsNaN(item.Accuracy) && !math.IsInf(item.Accuracy, 0) && item.Accuracy >= 0 && item.Accuracy <= 1 {
		meta.Description += fmt.Sprintf(" Accuracy %.2f%%.", item.Accuracy*100)
	}
	if pp := item.PerformancePoints; pp != nil && item.PPSource == "official" && !math.IsNaN(*pp) && !math.IsInf(*pp, 0) && *pp >= 0 {
		meta.Description += fmt.Sprintf(" %.2f PP.", *pp)
	}
	meta.NoIndex = false
	return meta
}

func (m pageMeta) inject(indexHTML string) string {
	t := html.EscapeString(m.Title)
	d := html.EscapeString(m.Description)
	c := html.EscapeString(m.Canonical)
	preview := html.EscapeString(m.socialImage())
	block := fmt.Sprintf(
		`<title>%s</title>
    <meta name="description" content="%s" />
    <link rel="canonical" href="%s" />
    <meta property="og:title" content="%s" />
    <meta property="og:description" content="%s" />
    <meta property="og:type" content="%s" />
    <meta property="og:url" content="%s" />
    <meta property="og:site_name" content="AimMod Hub" />
    <meta property="og:image" content="%s" />
    <meta property="og:image:width" content="1200" />
    <meta property="og:image:height" content="630" />
    <meta property="og:image:type" content="image/png" />
    <meta property="og:image:alt" content="%s" />
    <meta name="twitter:card" content="summary_large_image" />
    <meta name="twitter:image" content="%s" />
    <meta name="twitter:image:alt" content="%s" />
    <meta name="twitter:title" content="%s" />
    <meta name="twitter:description" content="%s" />`,
		t, d, c, t, d, html.EscapeString(m.OGType), c, preview, t, preview, t, t, d,
	)
	robots := "index, follow"
	if m.NoIndex {
		robots = "noindex, nofollow"
	}
	block += fmt.Sprintf(`<meta name="robots" content="%s" />`, robots)
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
						owned = owned || value == "description" || value == "robots" || strings.HasPrefix(value, "og:") || strings.HasPrefix(value, "twitter:")
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

func (m pageMeta) socialImage() string {
	const brand = "https://aimmod.app/brand/aimmod-v9/share-card-1200x630.png"
	canonical, err := url.Parse(m.Canonical)
	if err != nil || m.NoIndex || canonical.Host == "" {
		return brand
	}
	route := strings.TrimRight(canonical.Path, "/")
	if route == "" || route == "/app" || route == "/search" {
		return brand
	}
	_, published := seo.Published.Routes[route]
	detail := reSocialDetail.MatchString(route)
	if !published && !detail {
		return brand
	}
	return canonical.Scheme + "://" + canonical.Host + "/social-preview.png?path=" + url.QueryEscape(route) + "&v=1"
}

func isStaticAssetPath(p string) bool {
	if p == "" {
		return false
	}
	if strings.HasPrefix(p, "assets/") {
		return true
	}
	// Dots in player handles and other route parameters are not file extensions.
	route := "/" + strings.TrimSuffix(p, "/")
	if reProfile.MatchString(route) || reScenario.MatchString(route) || reRun.MatchString(route) || reSocialDetail.MatchString(route) || reClientDetail.MatchString(route) {
		return false
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

func resolvePageMeta(ctx context.Context, path, canonical string, st *store.Store, providers ...publicScoreMetadataProvider) pageMeta {
	if strings.Trim(path, "/") != "" {
		canonical = strings.TrimRight(canonical, "/")
	}
	fallback := pageMeta{
		Title:       "AimMod Hub",
		Description: "AimMod analysis and coaching for osu! and KovaaK's, plus shared practice data.",
		OGType:      "website",
		Canonical:   canonical,
	}

	cleanPath := strings.TrimSuffix(path, "/")
	if cleanPath == "" {
		cleanPath = "/"
	}
	path = cleanPath
	if seo.IsPrivateRoute(cleanPath) {
		fallback.NoIndex = true
		return fallback
	}
	if strings.HasPrefix(cleanPath, "/osu/scores/") {
		var provider publicScoreMetadataProvider
		if len(providers) > 0 {
			provider = providers[0]
		}
		return resolveOfficialScoreMeta(ctx, cleanPath, canonical, provider)
	}
	if page, ok := seo.Published.Routes[cleanPath]; ok {
		return pageMeta{Title: page.Title, Description: page.Description, OGType: "website", Canonical: canonical, NoIndex: cleanPath == "/search"}
	}
	if strings.HasPrefix(cleanPath, "/osu/learn/") {
		for _, guide := range seo.Published.Guides {
			if cleanPath == "/osu/learn/"+guide.Slug {
				return pageMeta{Title: guide.Title + " · AimMod Hub", Description: guide.Description, OGType: "article", Canonical: canonical}
			}
		}
		fallback.Title, fallback.NoIndex = "Guide unavailable · AimMod Hub", true
		return fallback
	}
	if strings.HasPrefix(cleanPath, "/osu/replays/") || strings.HasPrefix(cleanPath, "/osu/profiles/") {
		if st == nil {
			return unavailableOsuMeta(canonical)
		}
		return resolveOsuDetailMeta(ctx, cleanPath, canonical, st)
	}
	if cleanPath == "/osu" || strings.HasPrefix(cleanPath, "/osu/") || cleanPath == "/app/osu" {
		fallback.Title = "osu! Analysis and Community · AimMod Hub"
		fallback.Description = "Explore osu! beatmaps, skins, player profiles and shared replays with AimMod analysis and coaching."
		// Published routes and valid detail routes were resolved above.
		fallback.NoIndex = true
		fallback.NotFound = true
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
		if st == nil {
			fallback.NoIndex = true
			return fallback
		}
		meta, err := st.GetProfileMeta(ctx, m[1])
		if err != nil || meta == nil {
			return pageMeta{Title: "Profile unavailable · AimMod Hub", Description: fallback.Description, OGType: "profile", Canonical: canonical, NoIndex: true}
		}
		name := meta.DisplayName
		if name == "" {
			name = meta.Handle
		}
		if strings.HasPrefix(path, "/profiles/"+m[1]+"/benchmarks") {
			title := "Benchmarks"
			if m[2] != "" {
				title = "Benchmark " + m[2]
			}
			return pageMeta{Title: fmt.Sprintf("%s by %s · AimMod Hub", title, name), Description: fmt.Sprintf("Explore %s's KovaaK's benchmark ranks and scenario results on AimMod Hub.", name), OGType: "website", Canonical: canonical}
		}
		if m[3] != "" {
			scenario, err := st.GetScenarioMeta(ctx, m[3])
			if err != nil {
				fallback.NoIndex = true
				return fallback
			}
			return pageMeta{Title: fmt.Sprintf("%s by %s · AimMod Hub", scenario.Name, name), Description: fmt.Sprintf("Explore %s's scores, practice history and progress in %s.", name, scenario.Name), OGType: "website", Canonical: canonical}
		}
		return pageMeta{
			Title:       fmt.Sprintf("%s (@%s) · AimMod Hub", name, meta.Handle),
			Description: fmt.Sprintf("%d runs across %d scenarios on AimMod Hub.", meta.RunCount, meta.ScenarioCount),
			OGType:      "profile",
			Canonical:   canonical,
		}
	}

	if m := reScenario.FindStringSubmatch(path); m != nil {
		if st == nil {
			fallback.NoIndex = true
			return fallback
		}
		meta, err := st.GetScenarioMeta(ctx, m[1])
		if err != nil {
			return pageMeta{Title: "Scenario unavailable · AimMod Hub", Description: fallback.Description, OGType: "website", Canonical: canonical, NoIndex: true}
		}
		return pageMeta{
			Title:       fmt.Sprintf("%s · AimMod Hub", meta.Name),
			Description: fmt.Sprintf("%d runs · Best score %.0f · Avg accuracy %.1f%%", meta.RunCount, meta.BestScore, meta.AvgAcc),
			OGType:      "website",
			Canonical:   canonical,
		}
	}

	if m := reRun.FindStringSubmatch(path); m != nil {
		if st == nil {
			fallback.NoIndex = true
			return fallback
		}
		meta, err := st.GetRunMeta(ctx, m[1])
		if err != nil {
			fallback.NoIndex = true
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

	// These existing public routes hydrate their detail metadata in the client.
	fallback.NoIndex = !reClientDetail.MatchString(cleanPath)
	fallback.NotFound = fallback.NoIndex
	return fallback
}

func NewSPAHandler(dir string, st *store.Store, origin string, providers ...publicScoreMetadataProvider) http.Handler {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		origin = "https://aimmod.app"
	}
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
			fileServer.ServeHTTP(&spaStaticResponse{ResponseWriter: w, assetPath: p}, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")

		if p != "" {
			indexPath := path.Join(p, "index.html")
			rawRouteHTML, readErr := fs.ReadFile(fsys, indexPath)
			if readErr == nil {
				if seo.IsPrivateRoute(r.URL.Path) || strings.HasPrefix(r.URL.Path, "/osu/scores/") {
					meta := resolvePageMeta(r.Context(), r.URL.Path, origin+r.URL.EscapedPath(), st, providers...)
					rawRouteHTML = []byte(meta.inject(string(rawRouteHTML)))
					if meta.NoIndex {
						w.Header().Set("X-Robots-Tag", "noindex, nofollow")
					}
				}
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
		canonical := origin + r.URL.EscapedPath()
		meta := resolvePageMeta(r.Context(), r.URL.Path, canonical, st, providers...)
		if meta.NoIndex {
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if meta.NotFound || (meta.NoIndex && strings.HasPrefix(r.URL.Path, "/osu/learn/")) {
			w.WriteHeader(http.StatusNotFound)
		}
		_, _ = w.Write([]byte(meta.inject(indexHTML)))
	})
}
