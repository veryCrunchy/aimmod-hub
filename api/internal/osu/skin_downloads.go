package osu

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
	"golang.org/x/net/html"
)

const maxSkinArchiveBytes = 512 << 20

func skinUpstreamClient(client *upstreamClient) *upstreamClient {
	copyClient := *client
	httpClient := *client.http
	httpClient.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 5 || request.URL.User != nil || request.URL.Scheme != client.baseURL.Scheme || request.URL.Host != client.baseURL.Host {
			return fmt.Errorf("skin provider redirect outside configured origin")
		}
		return nil
	}
	copyClient.http = &httpClient
	return &copyClient
}

// External file hosts remain browser handoffs. Never rewrite Drive/Mega pages
// into guessed download endpoints, and never request their URLs from the server.
func externalSkinBrowserURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return ""
	}
	switch strings.ToLower(u.Hostname()) {
	case "drive.google.com":
		if !strings.HasPrefix(u.Path, "/file/d/") && u.Path != "/open" {
			return ""
		}
	case "mega.nz", "mega.co.nz":
		if !strings.HasPrefix(u.Path, "/file/") && !strings.HasPrefix(u.Fragment, "!") {
			return ""
		}
	default:
		return ""
	}
	return u.String()
}

func (a *osuSkinsAdapter) resolvePublishedDownloads(ctx context.Context, body []byte, item *osuv1.SkinItem) {
	normalizeSkinSources(item)
	document, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return
	}
	seen := map[string]bool{}
	candidates := 0
	walkHTML(document, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "a" || candidates >= 8 {
			return
		}
		raw := htmlAttribute(node, "href")
		if seen[raw] {
			return
		}
		seen[raw] = true
		var handoff *osuv1.SkinDownloadHandoff
		if browser := externalSkinBrowserURL(raw); browser != "" {
			candidates++
			handoff = browserSkinHandoff(browser, "Open this file host in your browser to download this variant.")
		} else {
			u, err := url.Parse(raw)
			if err != nil || u.User != nil {
				return
			}
			u = a.client.baseURL.ResolveReference(u)
			if u.Scheme != a.client.baseURL.Scheme || u.Host != a.client.baseURL.Host || !strings.HasSuffix(strings.ToLower(u.Path), ".osk") || u.Fragment != "" {
				return
			}
			candidates++
			handoff, err = verifySkinArchive(ctx, a.client, u)
			if err != nil {
				return
			}
		}
		item.Sources = append(item.Sources, &osuv1.SkinSource{Provider: item.Provider, SourceId: item.SourceId, PageUrl: skinPageURL(item.Provider, item.SourceId), DownloadHandoff: handoff, Variant: strings.TrimSpace(htmlText(node))})
	})
	// A variant list is never silently collapsed into one chosen archive.
	if len(item.Sources) == 2 && item.Sources[1].DownloadHandoff.Kind == osuv1.SkinDownloadHandoffKind_SKIN_DOWNLOAD_HANDOFF_KIND_DIRECT_URL {
		item.DownloadHandoff = item.Sources[1].DownloadHandoff
	}
}

func verifySkinArchive(ctx context.Context, client *upstreamClient, u *url.URL) (*osuv1.SkinDownloadHandoff, error) {
	if u.User != nil || u.Host != client.baseURL.Host || u.Scheme != client.baseURL.Scheme {
		return nil, fmt.Errorf("archive must be on the configured provider origin")
	}
	// Do not follow even same-host redirects: signed redirects to external storage
	// need a separately verified contract, not an arbitrary URL fetcher.
	httpClient := *client.http
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if err := client.limiter.wait(ctx); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Range", "bytes=0-3")
	request.Header.Set("User-Agent", client.userAgent)
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("archive returned HTTP %d", response.StatusCode)
	}
	var size int64
	if response.StatusCode == http.StatusPartialContent {
		var end int64
		if _, err := fmt.Sscanf(response.Header.Get("Content-Range"), "bytes 0-%d/%d", &end, &size); err != nil || end != 3 {
			return nil, fmt.Errorf("invalid archive range")
		}
	} else {
		size = response.ContentLength
	}
	if size < 4 || size > maxSkinArchiveBytes {
		return nil, fmt.Errorf("archive size unavailable or outside limit")
	}
	prefix, err := io.ReadAll(io.LimitReader(response.Body, 4))
	if err != nil || string(prefix) != "PK\x03\x04" {
		return nil, fmt.Errorf("response is not a ZIP archive")
	}
	return &osuv1.SkinDownloadHandoff{Kind: osuv1.SkinDownloadHandoffKind_SKIN_DOWNLOAD_HANDOFF_KIND_DIRECT_URL, Available: true, Uri: u.String(), FileName: path.Base(u.Path), ExpectedSizeBytes: uint64(size), MaxDownloadBytes: maxSkinArchiveBytes, Message: "Archive response checked; the client must validate the downloaded skin before import."}, nil
}
