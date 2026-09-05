package httpserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	htmlparser "golang.org/x/net/html"
)

func readBrandHead(t *testing.T, source string) map[string][]string {
	t.Helper()
	doc, err := htmlparser.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	values := map[string][]string{}
	var visit func(*htmlparser.Node)
	visit = func(n *htmlparser.Node) {
		if n.Type == htmlparser.ElementNode {
			attrs := map[string]string{}
			for _, a := range n.Attr {
				attrs[a.Key] = a.Val
			}
			if n.Data == "meta" {
				key := attrs["name"]
				if key == "" {
					key = attrs["property"]
				}
				key = strings.ToLower(key)
				values[key] = append(values[key], attrs["content"])
			}
			if n.Data == "title" && n.FirstChild != nil {
				values["title"] = append(values["title"], n.FirstChild.Data)
			}
			if n.Data == "link" && attrs["rel"] == "canonical" {
				values["canonical"] = append(values["canonical"], attrs["href"])
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	return values
}

func TestBrandSPAHTTPMetadata(t *testing.T) {
	dir := t.TempDir()
	index, err := os.ReadFile("../../../web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), index, 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewSPAHandler(dir, nil, "https://aimmod.app"))
	defer server.Close()
	for _, route := range []string{"/", "/osu", "/osu/beatmaps", "/osu/beatmaps/", "/osu/skins", "/app", "/app/osu"} {
		t.Run(route, func(t *testing.T) {
			response, err := http.Get(server.URL + route)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != http.StatusOK {
				t.Fatalf("status %d", response.StatusCode)
			}
			meta := readBrandHead(t, string(body))
			for _, key := range []string{"title", "description", "canonical", "og:title", "og:description", "og:type", "og:url", "og:site_name", "og:image", "og:image:width", "og:image:height", "og:image:type", "og:image:alt", "twitter:card", "twitter:title", "twitter:description", "twitter:image", "twitter:image:alt"} {
				if len(meta[key]) != 1 {
					t.Fatalf("%s: expected one entry, got %v", key, meta[key])
				}
			}
			if meta["twitter:card"][0] != "summary_large_image" {
				t.Fatal(meta["twitter:card"])
			}
			expectedImage := (pageMeta{Canonical: "https://aimmod.app" + route}).socialImage()
			if meta["og:image"][0] != expectedImage || meta["og:image"][0] != meta["twitter:image"][0] {
				t.Fatal("incorrect social image")
			}
			canonicalPath := strings.TrimRight(route, "/")
			if canonicalPath == "" {
				canonicalPath = "/"
			}
			if meta["canonical"][0] != "https://aimmod.app"+canonicalPath {
				t.Fatal("incorrect canonical URL")
			}
			if strings.Contains(route, "/osu") && (!strings.Contains(meta["title"][0], "osu!") || strings.Contains(meta["description"][0], "KovaaK")) {
				t.Fatal("stale osu route metadata")
			}
			if !strings.Contains(string(body), `/brand/aimmod-v9/app-icon.svg`) || !strings.Contains(string(body), `/src/main.tsx`) {
				t.Fatal("lost favicon or app script")
			}
		})
	}
}

func TestBrandMetadataReplacementIsIdempotentAndEscaped(t *testing.T) {
	input := `<!doctype html><html><head data-test="keep"><TITLE>Old</TITLE><meta content='stale' NAME='DESCRIPTION'><meta content='summary' name='twitter:card'><meta property='og:image' content='old'><link href='old' rel='canonical'><meta name='theme-color' content='#101113'><script>const value = '<meta name="description">';</script></head><body>Keep this body</body></html>`
	meta := pageMeta{Title: `A < B & "C"`, Description: `No <script> tag`, Canonical: "https://aimmod.app/osu", OGType: "website"}
	output := meta.inject(meta.inject(input))
	values := readBrandHead(t, output)
	if len(values["title"]) != 1 || values["title"][0] != meta.Title || len(values["description"]) != 1 || values["description"][0] != meta.Description {
		t.Fatal("escaped metadata did not round trip")
	}
	if len(values["twitter:card"]) != 1 || values["twitter:card"][0] != "summary_large_image" {
		t.Fatal("conflicting Twitter card")
	}
	if !strings.Contains(output, `<script>const value = '<meta name="description">';</script>`) || !strings.Contains(output, "Keep this body") || values["theme-color"][0] != "#101113" {
		t.Fatal("unrelated markup changed")
	}
}
