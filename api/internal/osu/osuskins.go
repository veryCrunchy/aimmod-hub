package osu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
	"golang.org/x/net/html"
)

var (
	osuSkinsIDPattern   = regexp.MustCompile(`^[A-Za-z0-9]{7}$`)
	osuSkinsSizePattern = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*MB\s+file size`)
)

type osuSkinsAdapter struct {
	client *upstreamClient
}

type osuSkinsDirectoryResponse struct {
	Items []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"items"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Total  int `json:"total"`
}

type osuSkinsArticle struct {
	Type          string `json:"@type"`
	Headline      string `json:"headline"`
	Description   string `json:"description"`
	Image         string `json:"image"`
	DatePublished string `json:"datePublished"`
	DateModified  string `json:"dateModified"`
	Author        []struct {
		Name string `json:"name"`
	} `json:"author"`
	InteractionStatistic []struct {
		InteractionType      string `json:"interactionType"`
		UserInteractionCount string `json:"userInteractionCount"`
	} `json:"interactionStatistic"`
}

func newOsuSkinsAdapter(client *upstreamClient) *osuSkinsAdapter {
	return &osuSkinsAdapter{client: client}
}

func (a *osuSkinsAdapter) status(ctx context.Context) *osuv1.SkinProviderStatus {
	status := baseSkinProviderStatus(osuv1.SkinProvider_SKIN_PROVIDER_OSU_SKINS)
	status.ContractIsDocumented = false
	status.RequiresInteractiveDownloadVerification = true
	body, err := a.client.get(ctx, "/", nil, "")
	if err != nil {
		status.Message = "osuskins.net browse catalog is unavailable: " + err.Error()
		return status
	}
	items, _, err := parseOsuSkinsSearch(body, nil)
	if err != nil || len(items) == 0 {
		status.Message = "osuskins.net did not return a recognizable browse catalog."
		return status
	}
	status.Available = true
	status.SupportsSearch = true
	status.SupportsDetail = true
	status.SupportsScreenshots = true
	status.Message = "Browse, filters, details, and screenshots use osuskins.net's public HTML catalog. Downloads require an interactive Turnstile check and are not automated by Hub."
	return status
}

func (a *osuSkinsAdapter) search(ctx context.Context, req *osuv1.SearchSkinsRequest, pageToken string) ([]*osuv1.SkinItem, string, error) {
	query := url.Values{}
	if value := strings.TrimSpace(req.GetQuery()); value != "" {
		query.Set("q", value)
	}
	for _, ruleset := range req.GetFilters().GetRulesets() {
		mode, ok := osuSkinsMode(ruleset)
		if !ok {
			return nil, "", fmt.Errorf("osuskins.net does not recognize ruleset %s", ruleset)
		}
		query.Add("mode[]", mode)
	}
	if creator := strings.TrimSpace(req.GetFilters().GetCreator()); creator != "" {
		id, err := a.resolveDirectoryID(ctx, "creator", creator)
		if err != nil {
			return nil, "", err
		}
		query.Add("creator[]", id)
	}
	if player := strings.TrimSpace(req.GetFilters().GetPlayer()); player != "" {
		id, err := a.resolveDirectoryID(ctx, "player", player)
		if err != nil {
			return nil, "", err
		}
		query.Add("player[]", id)
	}
	if sortValue := osuSkinsSort(req.GetSort()); sortValue != "" {
		query.Set("sortby", sortValue)
	}
	if req.GetDirection() == osuv1.SortDirection_SORT_DIRECTION_ASCENDING {
		query.Set("order", "asc")
	} else if req.GetDirection() == osuv1.SortDirection_SORT_DIRECTION_DESCENDING {
		query.Set("order", "desc")
	}
	page := uint64(1)
	if pageToken != "" {
		parsed, err := parsePositiveID(pageToken, "page token")
		if err != nil || parsed > 10000 {
			return nil, "", fmt.Errorf("page token must be an integer from 1 through 10000")
		}
		page = parsed
	}
	query.Set("p", strconv.FormatUint(page, 10))
	body, err := a.client.get(ctx, "/", query, "")
	if err != nil {
		// osuskins.net uses HTTP 404 for a valid search with no matching
		// catalogue entries. Treat that as an empty result set rather than a
		// provider outage so the native client can render an honest empty state.
		if isUpstreamHTTPStatus(err, 404) {
			return []*osuv1.SkinItem{}, "", nil
		}
		return nil, "", err
	}
	items, next, err := parseOsuSkinsSearch(body, req.GetFilters().GetRulesets())
	if err != nil {
		return nil, "", err
	}
	return items, next, nil
}

func (a *osuSkinsAdapter) detail(ctx context.Context, sourceID string) (*osuv1.SkinItem, error) {
	if !osuSkinsIDPattern.MatchString(sourceID) {
		return nil, fmt.Errorf("osuskins.net source_id must contain seven ASCII letters or digits")
	}
	body, err := a.client.get(ctx, "/skin/"+sourceID, nil, "")
	if err != nil {
		return nil, err
	}
	return parseOsuSkinsDetail(body, sourceID)
}

func (a *osuSkinsAdapter) resolveDirectoryID(ctx context.Context, kind, wanted string) (string, error) {
	if kind != "creator" && kind != "player" {
		return "", fmt.Errorf("unsupported osuskins.net directory %q", kind)
	}
	const pageSize = 20
	for offset := 0; offset < 500; offset += pageSize {
		query := url.Values{
			"type":   []string{kind},
			"offset": []string{strconv.Itoa(offset)},
			"limit":  []string{strconv.Itoa(pageSize)},
		}
		body, err := a.client.get(ctx, "/load-more-filters", query, "")
		if err != nil {
			return "", err
		}
		var response osuSkinsDirectoryResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return "", fmt.Errorf("parse osuskins.net %s directory: %w", kind, err)
		}
		for _, item := range response.Items {
			if strings.EqualFold(strings.TrimSpace(item.Name), wanted) {
				return item.ID, nil
			}
		}
		if offset+len(response.Items) >= response.Total || len(response.Items) == 0 {
			return "", fmt.Errorf("osuskins.net %s %q was not found", kind, wanted)
		}
	}
	return "", fmt.Errorf("osuskins.net %s directory exceeded the 500-entry lookup limit", kind)
}

func parseOsuSkinsSearch(body []byte, requestedRulesets []osuv1.Ruleset) ([]*osuv1.SkinItem, string, error) {
	document, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, "", err
	}
	items := make([]*osuv1.SkinItem, 0, 30)
	walkHTML(document, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "a" {
			return
		}
		href := htmlAttribute(node, "href")
		if !strings.HasPrefix(href, "/skin/") || firstHTMLDescendant(node, "h2") == nil {
			return
		}
		sourceID := strings.TrimPrefix(href, "/skin/")
		if !osuSkinsIDPattern.MatchString(sourceID) {
			return
		}
		nameNode := firstHTMLDescendant(node, "h2")
		imageNode := firstHTMLDescendant(node, "img")
		item := &osuv1.SkinItem{
			Provider:             osuv1.SkinProvider_SKIN_PROVIDER_OSU_SKINS,
			SourceId:             sourceID,
			Name:                 strings.TrimSpace(htmlText(nameNode)),
			ThumbnailUrl:         htmlAttribute(imageNode, "src"),
			Rulesets:             append([]osuv1.Ruleset(nil), requestedRulesets...),
			DownloadHandoff:      unavailableSkinHandoff(true, "osuskins.net requires an interactive Cloudflare Turnstile check before download. AimMod Hub does not bypass it."),
			CountsAreApproximate: true,
		}
		item.DownloadCount = osuSkinsMarkedCount(node, "#arrowDown")
		item.ViewCount = osuSkinsMarkedCount(node, "#eyeSolid")
		items = append(items, item)
	})
	if len(items) == 0 {
		return nil, "", fmt.Errorf("osuskins.net returned no recognizable skin cards")
	}
	next := ""
	walkHTML(document, func(node *html.Node) {
		if next != "" || node.Type != html.ElementNode || node.Data != "a" || strings.TrimSpace(htmlText(node)) != "Next" {
			return
		}
		if parsed, err := url.Parse(htmlAttribute(node, "href")); err == nil {
			next = parsed.Query().Get("p")
		}
	})
	return items, next, nil
}

func parseOsuSkinsDetail(body []byte, sourceID string) (*osuv1.SkinItem, error) {
	document, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	var article osuSkinsArticle
	walkHTML(document, func(node *html.Node) {
		if article.Type != "" || node.Type != html.ElementNode || node.Data != "script" || htmlAttribute(node, "type") != "application/ld+json" {
			return
		}
		var candidate osuSkinsArticle
		if json.Unmarshal([]byte(htmlText(node)), &candidate) == nil && candidate.Type == "Article" {
			article = candidate
		}
	})
	if article.Type != "Article" || strings.TrimSpace(article.Headline) == "" {
		return nil, fmt.Errorf("osuskins.net detail did not contain Article metadata")
	}
	item := &osuv1.SkinItem{
		Provider:        osuv1.SkinProvider_SKIN_PROVIDER_OSU_SKINS,
		SourceId:        sourceID,
		Name:            strings.TrimSpace(article.Headline),
		ThumbnailUrl:    article.Image,
		SubmittedAtIso:  normalizeTimestamp(article.DatePublished),
		UpdatedAtIso:    normalizeTimestamp(article.DateModified),
		DownloadHandoff: unavailableSkinHandoff(true, "osuskins.net requires an interactive Cloudflare Turnstile check before download. AimMod Hub does not bypass it."),
	}
	if len(article.Author) > 0 {
		item.Creator = strings.TrimSpace(article.Author[0].Name)
	}
	for _, statistic := range article.InteractionStatistic {
		count, _ := strconv.ParseUint(strings.TrimSpace(statistic.UserInteractionCount), 10, 64)
		switch {
		case strings.HasSuffix(statistic.InteractionType, "ViewAction"):
			item.ViewCount = count
		case strings.HasSuffix(statistic.InteractionType, "DownloadAction"):
			item.DownloadCount = count
		}
	}
	if match := osuSkinsSizePattern.FindStringSubmatch(article.Description); len(match) == 2 {
		if sizeMB, err := strconv.ParseFloat(match[1], 64); err == nil && sizeMB > 0 {
			item.FileSizeBytes = uint64(sizeMB * 1000 * 1000)
			item.FileSizeIsApproximate = true
		}
	}
	walkHTML(document, func(node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}
		if node.Data == "a" {
			if ruleset, ok := osuSkinsRulesetFromHref(htmlAttribute(node, "href")); ok && !containsRuleset(item.Rulesets, ruleset) {
				item.Rulesets = append(item.Rulesets, ruleset)
			}
		}
		if screenshotKey := htmlAttribute(node, "data-sskey"); screenshotKey != "" {
			width, height := osuSkinsDimensions(htmlAttribute(node, "data-lg-size"))
			item.Screenshots = append(item.Screenshots, &osuv1.SkinScreenshot{
				Label:    firstNonEmpty(htmlAttribute(node, "title"), screenshotKey),
				ImageUrl: htmlAttribute(node, "data-src"),
				Width:    width,
				Height:   height,
			})
		}
	})
	return item, nil
}

func osuSkinsMode(ruleset osuv1.Ruleset) (string, bool) {
	switch ruleset {
	case osuv1.Ruleset_RULESET_OSU:
		return "1", true
	case osuv1.Ruleset_RULESET_MANIA:
		return "2", true
	case osuv1.Ruleset_RULESET_TAIKO:
		return "3", true
	case osuv1.Ruleset_RULESET_CATCH:
		return "4", true
	default:
		return "", false
	}
}

func osuSkinsRulesetFromHref(href string) (osuv1.Ruleset, bool) {
	parsed, err := url.Parse(href)
	if err != nil {
		return osuv1.Ruleset_RULESET_UNSPECIFIED, false
	}
	value := parsed.Query().Get("mode[]")
	for _, ruleset := range []osuv1.Ruleset{osuv1.Ruleset_RULESET_OSU, osuv1.Ruleset_RULESET_MANIA, osuv1.Ruleset_RULESET_TAIKO, osuv1.Ruleset_RULESET_CATCH} {
		if mode, _ := osuSkinsMode(ruleset); mode == value {
			return ruleset, true
		}
	}
	return osuv1.Ruleset_RULESET_UNSPECIFIED, false
}

func osuSkinsSort(value osuv1.SkinSort) string {
	switch value {
	case osuv1.SkinSort_SKIN_SORT_NEWEST:
		return "date"
	case osuv1.SkinSort_SKIN_SORT_MOST_VIEWED:
		return "views"
	case osuv1.SkinSort_SKIN_SORT_MOST_DOWNLOADED:
		return "downloads"
	case osuv1.SkinSort_SKIN_SORT_NAME:
		return "name"
	case osuv1.SkinSort_SKIN_SORT_RANDOM:
		return "random"
	default:
		return ""
	}
}

func osuSkinsMarkedCount(node *html.Node, marker string) uint64 {
	var count uint64
	walkHTML(node, func(candidate *html.Node) {
		if count != 0 || candidate.Type != html.ElementNode || candidate.Data != "use" {
			return
		}
		if htmlAttribute(candidate, "xlink:href") != marker && htmlAttribute(candidate, "href") != marker {
			return
		}
		container := candidate.Parent
		if container != nil {
			container = container.Parent
		}
		count = parseCompactCount(htmlText(container))
	})
	return count
}

func parseCompactCount(value string) uint64 {
	value = strings.ToLower(strings.TrimSpace(value))
	multiplier := float64(1)
	if strings.HasSuffix(value, "k") {
		multiplier = 1000
		value = strings.TrimSuffix(value, "k")
	} else if strings.HasSuffix(value, "m") {
		multiplier = 1000000
		value = strings.TrimSuffix(value, "m")
	} else {
		value = strings.ReplaceAll(value, ".", "")
		value = strings.ReplaceAll(value, ",", "")
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || parsed < 0 {
		return 0
	}
	return uint64(parsed * multiplier)
}

func osuSkinsDimensions(value string) (uint32, uint32) {
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return 0, 0
	}
	width, _ := strconv.ParseUint(parts[0], 10, 32)
	height, _ := strconv.ParseUint(parts[1], 10, 32)
	return uint32(width), uint32(height)
}

func walkHTML(node *html.Node, visit func(*html.Node)) {
	if node == nil {
		return
	}
	visit(node)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkHTML(child, visit)
	}
}

func firstHTMLDescendant(node *html.Node, tag string) *html.Node {
	var found *html.Node
	walkHTML(node, func(candidate *html.Node) {
		if found == nil && candidate.Type == html.ElementNode && candidate.Data == tag {
			found = candidate
		}
	})
	return found
}

func htmlAttribute(node *html.Node, name string) string {
	if node == nil {
		return ""
	}
	for _, attribute := range node.Attr {
		if attribute.Key == name {
			return attribute.Val
		}
	}
	return ""
}

func htmlText(node *html.Node) string {
	if node == nil {
		return ""
	}
	var builder strings.Builder
	walkHTML(node, func(candidate *html.Node) {
		if candidate.Type == html.TextNode {
			builder.WriteString(candidate.Data)
			builder.WriteByte(' ')
		}
	})
	return strings.Join(strings.Fields(builder.String()), " ")
}

func containsRuleset(values []osuv1.Ruleset, wanted osuv1.Ruleset) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
