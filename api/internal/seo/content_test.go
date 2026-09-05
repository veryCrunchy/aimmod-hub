package seo

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestPublishedKnowledgeIntegrity(t *testing.T) {
	if _, err := time.Parse("2006-01-02", Published.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	slugs := map[string]bool{}
	validSlug := regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	for _, guide := range Published.Guides {
		if !validSlug.MatchString(guide.Slug) || slugs[guide.Slug] || guide.Title == "" || guide.Description == "" {
			t.Fatalf("invalid or duplicate guide: %+v", guide)
		}
		slugs[guide.Slug] = true
	}
	for route, page := range Published.Routes {
		if !strings.HasPrefix(route, "/") || strings.ContainsAny(route, "?#") || page.Title == "" || page.Description == "" {
			t.Fatalf("invalid route %s", route)
		}
	}
	var content struct {
		Guides []struct {
			Slug     string
			Sections []struct{ Title, Body string }
			Sources  []struct{ Title, URL string }
		}
	}
	if err := json.Unmarshal(raw, &content); err != nil {
		t.Fatal(err)
	}
	for _, guide := range content.Guides {
		if len(guide.Sections) == 0 || len(guide.Sources) == 0 {
			t.Fatal("guide needs content and sources:", guide.Slug)
		}
		for _, section := range guide.Sections {
			if strings.TrimSpace(section.Title) == "" || strings.TrimSpace(section.Body) == "" {
				t.Fatal("empty section:", guide.Slug)
			}
		}
		for _, source := range guide.Sources {
			u, err := url.Parse(source.URL)
			if err != nil || u.Scheme != "https" || u.Host == "" || source.Title == "" {
				t.Fatal("invalid source:", guide.Slug, source.URL)
			}
		}
	}
}
