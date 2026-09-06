package httpserver

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Small shards stay comfortably below both Google's 50,000 URL and 50MB limits.
const sitemapPageSize = 5000
const sitemapNamespace = "http://www.sitemaps.org/schemas/sitemap/0.9"

type sitemapStore interface {
	SitemapCounts(context.Context) (map[string]int64, error)
	SitemapPaths(context.Context, string, int, int) ([]string, error)
}

type sitemapIndex struct {
	XMLName  xml.Name     `xml:"sitemapindex"`
	Xmlns    string       `xml:"xmlns,attr"`
	Sitemaps []sitemapURL `xml:"sitemap"`
}

func newSitemapHandler(origin string, st sitemapStore) http.Handler {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		origin = "https://aimmod.app"
	}
	static := newStaticSitemapHandler(origin)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path == "/sitemaps/pages.xml" {
			static.ServeHTTP(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		unavailable := func() {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Retry-After", "60")
			http.Error(w, "sitemap temporarily unavailable", http.StatusServiceUnavailable)
		}
		if st == nil {
			unavailable()
			return
		}
		if r.URL.Path == "/sitemap.xml" {
			counts, err := st.SitemapCounts(ctx)
			if err != nil {
				unavailable()
				return
			}
			index := sitemapIndex{Xmlns: sitemapNamespace, Sitemaps: []sitemapURL{{Loc: origin + "/sitemaps/pages.xml"}}}
			kinds := make([]string, 0, len(counts))
			for kind := range counts {
				kinds = append(kinds, kind)
			}
			sort.Strings(kinds)
			for _, kind := range kinds {
				pages := (counts[kind] + sitemapPageSize - 1) / sitemapPageSize
				if pages > 49999-int64(len(index.Sitemaps)) {
					unavailable()
					return
				}
				for page := int64(1); page <= pages; page++ {
					index.Sitemaps = append(index.Sitemaps, sitemapURL{Loc: fmt.Sprintf("%s/sitemaps/%s-%d.xml", origin, kind, page)})
				}
			}
			writeSitemap(w, r, index)
			return
		}
		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/sitemaps/"), ".xml")
		split := strings.LastIndex(name, "-")
		if split < 1 || !strings.HasSuffix(r.URL.Path, ".xml") {
			http.NotFound(w, r)
			return
		}
		kind, rawPage := name[:split], name[split+1:]
		page, err := strconv.Atoi(rawPage)
		if err != nil || page < 1 || page > 50000 || strconv.Itoa(page) != rawPage {
			http.NotFound(w, r)
			return
		}
		// Validate the category without querying counts for every shard request.
		switch kind {
		case "profiles", "runs", "scenarios", "player-scenarios", "profile-benchmarks", "osu-profiles", "osu-public-players", "osu-public-scores", "osu-replays", "external-profiles", "osu-scores", "benchmarks", "player-benchmarks", "external-benchmarks", "kovaaks-profiles":
		default:
			http.NotFound(w, r)
			return
		}
		paths, err := st.SitemapPaths(ctx, kind, sitemapPageSize, (page-1)*sitemapPageSize)
		if err != nil {
			unavailable()
			return
		}
		if len(paths) == 0 {
			http.NotFound(w, r)
			return
		}
		set := sitemapURLSet{Xmlns: sitemapNamespace}
		seen := map[string]bool{}
		for _, path := range paths {
			u, err := url.Parse(path)
			if err != nil || u.IsAbs() || u.Host != "" || !strings.HasPrefix(path, "/") || u.RawQuery != "" || u.Fragment != "" || seen[path] {
				continue
			}
			seen[path] = true
			set.URLs = append(set.URLs, sitemapURL{Loc: origin + path})
		}
		writeSitemap(w, r, set)
	})
}

func writeSitemap(w http.ResponseWriter, r *http.Request, document any) {
	var body bytes.Buffer
	body.WriteString(xml.Header)
	if err := xml.NewEncoder(&body).Encode(document); err != nil || body.Len() > 50*1024*1024 {
		http.Error(w, "could not build sitemap", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300, must-revalidate")
	if r.Method != http.MethodHead {
		_, _ = w.Write(body.Bytes())
	}
}
