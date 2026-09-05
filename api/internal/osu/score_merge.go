package osu

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

type PublicScoreItem struct {
	store.OsuPublicReplay
	Source           string                  `json:"source"`
	OfficialScoreID  string                  `json:"officialScoreId,omitempty"`
	OfficialScoreURL string                  `json:"officialScoreUrl,omitempty"`
	PPSource         string                  `json:"ppSource"`
	Uploads          []store.OsuPublicReplay `json:"uploads,omitempty"`
}

// MergePublicScores never treats an accuracy or map match as a play identity.
// Exact IDs are scoped by owner, map and ruleset. Fallback is deliberately limited
// to unambiguous standard no-mod content with an exact timestamp and checksum.
func MergePublicScores(local []store.OsuPublicReplay, official []OfficialPublicScore) []PublicScoreItem {
	items := make([]PublicScoreItem, 0, len(local)+len(official))
	ids := map[string]int{}
	fallback := map[string][]int{}
	for _, score := range official {
		r := score.Replay
		key := onlineIdentity(r)
		if key == "" {
			continue
		}
		if _, exists := ids[key]; exists {
			continue
		}
		ids[key] = len(items)
		if score.FallbackEligible {
			if content := contentIdentity(r); content != "" {
				fallback[content] = append(fallback[content], len(items))
			}
		}
		ppSource := "unavailable"
		if r.PerformancePoints != nil {
			ppSource = "official"
		}
		id := strconv.FormatInt(r.OnlineScoreID, 10)
		items = append(items, PublicScoreItem{OsuPublicReplay: r, Source: "official", OfficialScoreID: id, OfficialScoreURL: "https://osu.ppy.sh/scores/" + id, PPSource: ppSource})
	}
	localContentCounts := map[string]int{}
	for _, r := range local {
		if key := contentIdentity(r); key != "" {
			localContentCounts[key]++
		}
	}
	for _, r := range local {
		if r.Visibility != store.OsuVisibilityPublic {
			continue
		}
		index, matched := ids[onlineIdentity(r)]
		if !matched && r.OnlineScoreID == 0 {
			content := contentIdentity(r)
			if candidates := fallback[content]; content != "" && len(candidates) == 1 && localContentCounts[content] == 1 {
				index, matched = candidates[0], true
			}
		}
		if matched {
			item := &items[index]
			if item.Source == "official" {
				pp := item.PerformancePoints
				id := item.OnlineScoreID
				item.OsuPublicReplay = r
				item.OnlineScoreID = id
				if pp != nil {
					item.PerformancePoints = pp
				} else if r.PerformancePoints != nil {
					item.PPSource = "local"
				}
			}
			item.Source = "merged"
			item.Uploads = append(item.Uploads, r)
			continue
		}
		ppSource := "unavailable"
		if r.PerformancePoints != nil {
			ppSource = "local"
		}
		items = append(items, PublicScoreItem{OsuPublicReplay: r, Source: "local", PPSource: ppSource})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].PlayedAt.After(items[j].PlayedAt) })
	return items
}

func onlineIdentity(r store.OsuPublicReplay) string {
	if r.OnlineScoreID <= 0 || r.OsuUserID <= 0 || r.BeatmapID <= 0 || !validScoreMode(r.Ruleset) {
		return ""
	}
	return fmt.Sprintf("%d/%d/%s/%d", r.OsuUserID, r.BeatmapID, r.Ruleset, r.OnlineScoreID)
}

func contentIdentity(r store.OsuPublicReplay) string {
	if r.OsuUserID <= 0 || r.BeatmapID <= 0 || r.Ruleset != "osu" || r.PlayedAt.IsZero() || r.TotalScore <= 0 || r.MaxCombo <= 0 || len(r.Mods) != 0 || len(r.BeatmapChecksum) != 32 || r.Count300+r.Count100+r.Count50+r.CountMiss == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d/%s/%s/%d/%d/%d/%d/%d/%d/%t", r.OsuUserID, r.BeatmapID, strings.ToLower(r.BeatmapChecksum), r.PlayedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"), r.TotalScore, r.MaxCombo, r.Count300, r.Count100, r.Count50, r.CountMiss, r.Passed)
}
