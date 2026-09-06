package httpserver

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/veryCrunchy/aimmod-hub/api/internal/coaching"
	"github.com/veryCrunchy/aimmod-hub/api/internal/seo"
)

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

func newRobotsHandler(origin string) http.Handler {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		origin = "https://aimmod.app"
	}
	body := fmt.Sprintf("User-agent: *\nAllow: /\n\nSitemap: %s/sitemap.xml\n", origin)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write([]byte(body))
	})
}

func newStaticSitemapHandler(origin string) http.Handler {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		origin = "https://aimmod.app"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index, err := coaching.GetLearnIndex()
		if err != nil {
			http.Error(w, "could not build sitemap", http.StatusInternalServerError)
			return
		}
		topics, err := coaching.GetLearnTopics()
		if err != nil {
			http.Error(w, "could not build sitemap", http.StatusInternalServerError)
			return
		}

		urls := []sitemapURL{}
		paths := []string{}
		for route := range seo.Published.Routes {
			if !seo.IsPrivateRoute(route) {
				paths = append(paths, route)
			}
		}
		sort.Strings(paths)
		for _, route := range paths {
			urls = append(urls, sitemapURL{Loc: origin + route})
		}
		for _, guide := range seo.Published.Guides {
			urls = append(urls, sitemapURL{Loc: origin + "/osu/learn/" + guide.Slug, LastMod: seo.Published.UpdatedAt})
		}
		for _, entry := range index.Entries {
			urls = append(urls, sitemapURL{
				Loc:     origin + "/learn/" + entry.ID,
				LastMod: index.UpdatedAtISO,
			})
		}
		for _, topic := range topics {
			urls = append(urls, sitemapURL{
				Loc:     origin + "/learn/topics/" + topic,
				LastMod: index.UpdatedAtISO,
			})
		}

		writeSitemap(w, r, sitemapURLSet{
			Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
			URLs:  urls,
		})
	})
}
