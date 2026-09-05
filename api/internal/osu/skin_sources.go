package osu

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
	"google.golang.org/protobuf/proto"
)

func skinPageURL(provider osuv1.SkinProvider, id string) string {
	switch provider {
	case osuv1.SkinProvider_SKIN_PROVIDER_OSU_SKINS:
		if osuSkinsIDPattern.MatchString(id) {
			return "https://osuskins.net/skin/" + id
		}
	case osuv1.SkinProvider_SKIN_PROVIDER_OSUCK:
		if n, err := strconv.ParseUint(id, 10, 64); err == nil && n > 0 && strconv.FormatUint(n, 10) == id {
			return "https://skins.osuck.net/skins/" + id
		}
	}
	return ""
}

func browserSkinHandoff(page, message string) *osuv1.SkinDownloadHandoff {
	return &osuv1.SkinDownloadHandoff{Kind: osuv1.SkinDownloadHandoffKind_SKIN_DOWNLOAD_HANDOFF_KIND_BROWSER_URL, Available: true, Uri: page, RequiresInteractiveVerification: true, Message: message}
}

// Never discard URL queries or fragments: these can identify different variants,
// signed files, or Mega decryption keys. Only host and scheme are case-insensitive.
func skinArtifactIdentity(h *osuv1.SkinDownloadHandoff) string {
	if h == nil || !h.Available || h.Kind != osuv1.SkinDownloadHandoffKind_SKIN_DOWNLOAD_HANDOFF_KIND_DIRECT_URL {
		return ""
	}
	u, err := url.Parse(h.Uri)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return ""
	}
	u.Host = strings.ToLower(u.Host)
	return u.String()
}

func normalizeSkinSources(item *osuv1.SkinItem) {
	if len(item.Sources) == 0 {
		item.Sources = []*osuv1.SkinSource{{Provider: item.Provider, SourceId: item.SourceId, PageUrl: skinPageURL(item.Provider, item.SourceId), DownloadHandoff: item.DownloadHandoff}}
	}
	item.NormalizedId = fmt.Sprintf("%d:%s", item.Provider, item.SourceId)
	if artifact := skinArtifactIdentity(item.DownloadHandoff); artifact != "" && len(item.Sources) == 1 {
		item.NormalizedId = fmt.Sprintf("artifact:%x", sha256.Sum256([]byte(artifact)))
	}
}

// Source IDs are authoritative within a provider. Cross-provider merging requires
// the exact same verified archive, never a shared name, thumbnail, or browser page.
func deduplicateSkins(items []*osuv1.SkinItem) []*osuv1.SkinItem {
	result := make([]*osuv1.SkinItem, 0, len(items))
	groups := map[string]*osuv1.SkinItem{}
	for _, input := range items {
		if input == nil {
			continue
		}
		item := proto.Clone(input).(*osuv1.SkinItem)
		normalizeSkinSources(item)
		key := item.NormalizedId
		if existing := groups[key]; existing != nil {
			for _, source := range item.Sources {
				duplicate := false
				for _, old := range existing.Sources {
					if proto.Equal(old, source) {
						duplicate = true
						break
					}
				}
				if !duplicate {
					existing.Sources = append(existing.Sources, source)
				}
			}
			continue
		}
		groups[key] = item
		result = append(result, item)
	}
	return result
}
