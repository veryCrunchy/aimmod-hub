package osu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
)

type collectorAdapter struct {
	client *upstreamClient
}

const collectorSearchHydrationConcurrency = 4
const collectorSearchHydrationItems = 4
const collectorSearchDifficultyLimit = 50

type collectorTimestamp struct {
	Seconds int64 `json:"_seconds"`
}

type collectorUploader struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
}

type collectorCollection struct {
	ID               uint64             `json:"id"`
	Name             string             `json:"name"`
	Description      string             `json:"description"`
	Uploader         collectorUploader  `json:"uploader"`
	BeatmapCount     uint64             `json:"beatmapCount"`
	Favourites       uint64             `json:"favourites"`
	DateLastModified collectorTimestamp `json:"dateLastModified"`
	DateUploaded     collectorTimestamp `json:"dateUploaded"`
	DifficultySpread map[string]uint64  `json:"difficultySpread"`
	BPMSpread        map[string]uint64  `json:"bpmSpread"`
	Modes            map[string]uint64  `json:"modes"`
}

type collectorSearchResponse struct {
	NextPageCursor uint64                `json:"nextPageCursor"`
	HasMore        bool                  `json:"hasMore"`
	Collections    []collectorCollection `json:"collections"`
}

type collectorBeatmapsResponse struct {
	NextPageCursor uint64             `json:"nextPageCursor"`
	HasMore        bool               `json:"hasMore"`
	Beatmaps       []collectorBeatmap `json:"beatmaps"`
}

type collectorBeatmap struct {
	ID               uint64  `json:"id"`
	BeatmapsetID     uint64  `json:"beatmapset_id"`
	Checksum         string  `json:"checksum"`
	Version          string  `json:"version"`
	Mode             string  `json:"mode"`
	Status           string  `json:"status"`
	DifficultyRating float64 `json:"difficulty_rating"`
	Accuracy         float64 `json:"accuracy"`
	Drain            float64 `json:"drain"`
	BPM              float64 `json:"bpm"`
	CS               float64 `json:"cs"`
	AR               float64 `json:"ar"`
	HitLength        uint32  `json:"hit_length"`
	Beatmapset       struct {
		ID      uint64 `json:"id"`
		Creator string `json:"creator"`
		Artist  string `json:"artist"`
		Title   string `json:"title"`
		Covers  struct {
			Card  string `json:"card"`
			Cover string `json:"cover"`
		} `json:"covers"`
	} `json:"beatmapset"`
}

func newCollectorAdapter(client *upstreamClient) *collectorAdapter {
	return &collectorAdapter{client: client}
}

func (a *collectorAdapter) status(ctx context.Context) *osuv1.ProviderStatus {
	status := baseProviderStatus(osuv1.Provider_PROVIDER_OSU_COLLECTOR)
	status.Configured = true
	status.Authentication = "No credentials; public read-only site API"
	status.ContractIsDocumented = false
	if _, err := a.client.get(ctx, "/api/collections/recent", nil, ""); err != nil {
		status.Message = "osu!Collector is unavailable: " + err.Error()
		return status
	}
	status.Available = true
	status.SupportsSearch = true
	status.SupportsDetail = true
	status.SupportsDownloadHandoff = true
	status.Message = "osu!Collector public read-only API is available. Its site API is not formally documented and may change."
	return status
}

func (a *collectorAdapter) search(ctx context.Context, req *osuv1.SearchBeatmapItemsRequest, pageToken string) ([]*osuv1.BeatmapItem, string, error) {
	query := url.Values{}
	query.Set("search", strings.TrimSpace(req.GetQuery()))
	if strings.TrimSpace(req.GetQuery()) == "" {
		query.Set("sortBy", "dateUploaded")
	} else {
		query.Set("sortBy", "_text_match")
	}
	query.Set("orderBy", "desc")
	if pageToken != "" {
		if _, err := parsePositiveID(pageToken, "page token"); err != nil {
			return nil, "", err
		}
		query.Set("cursor", pageToken)
	}
	body, err := a.client.get(ctx, "/api/collections/search", query, "")
	if err != nil {
		return nil, "", err
	}
	var response collectorSearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, "", err
	}
	items := make([]*osuv1.BeatmapItem, 0, len(response.Collections))
	for i := range response.Collections {
		item := normalizeCollectorCollection(&response.Collections[i])
		if collectionMatchesFilters(item, req.GetFilters()) {
			items = append(items, item)
		}
	}
	items = a.hydrateSearchItems(ctx, items)
	next := ""
	if response.HasMore && response.NextPageCursor > 0 {
		next = strconv.FormatUint(response.NextPageCursor, 10)
	}
	return items, next, nil
}

func (a *collectorAdapter) hydrateSearchItems(ctx context.Context, items []*osuv1.BeatmapItem) []*osuv1.BeatmapItem {
	if len(items) == 0 {
		return items
	}
	hydrated := make([]*osuv1.BeatmapItem, len(items))
	copy(hydrated, items)
	semaphore := make(chan struct{}, collectorSearchHydrationConcurrency)
	var waitGroup sync.WaitGroup
	for index, summary := range items {
		if index >= collectorSearchHydrationItems {
			break
		}
		index, summary := index, summary
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}
			detail, _, err := a.detail(ctx, summary.GetSourceId(), "")
			if err == nil && detail != nil && len(detail.GetDifficulties()) > 0 {
				if len(detail.Difficulties) > collectorSearchDifficultyLimit {
					detail.Difficulties = detail.Difficulties[:collectorSearchDifficultyLimit]
				}
				hydrated[index] = detail
			}
		}()
	}
	waitGroup.Wait()
	return hydrated
}

func (a *collectorAdapter) detail(ctx context.Context, sourceID, pageToken string) (*osuv1.BeatmapItem, string, error) {
	if _, err := parsePositiveID(sourceID, "source_id"); err != nil {
		return nil, "", err
	}
	if pageToken != "" {
		if _, err := parsePositiveID(pageToken, "page token"); err != nil {
			return nil, "", err
		}
	}
	metadataBody, err := a.client.get(ctx, "/api/collections/"+sourceID, nil, "")
	if err != nil {
		return nil, "", err
	}
	var collection collectorCollection
	if err := json.Unmarshal(metadataBody, &collection); err != nil {
		return nil, "", err
	}
	query := url.Values{}
	if pageToken != "" {
		query.Set("cursor", pageToken)
	}
	beatmapsBody, err := a.client.get(ctx, "/api/collections/"+sourceID+"/beatmapsv2", query, "")
	if err != nil {
		return nil, "", err
	}
	var response collectorBeatmapsResponse
	if err := json.Unmarshal(beatmapsBody, &response); err != nil {
		return nil, "", err
	}
	item := normalizeCollectorCollection(&collection)
	item.Difficulties = make([]*osuv1.BeatmapDifficulty, 0, len(response.Beatmaps))
	for i := range response.Beatmaps {
		beatmap := &response.Beatmaps[i]
		item.Difficulties = append(item.Difficulties, normalizeCollectorDifficulty(beatmap))
		if item.CoverUrl == "" {
			item.CoverUrl = firstNonEmpty(beatmap.Beatmapset.Covers.Card, beatmap.Beatmapset.Covers.Cover)
		}
	}
	next := ""
	if response.HasMore && response.NextPageCursor > 0 {
		next = strconv.FormatUint(response.NextPageCursor, 10)
	}
	return item, next, nil
}

func normalizeCollectorCollection(collection *collectorCollection) *osuv1.BeatmapItem {
	item := &osuv1.BeatmapItem{
		Provider:        osuv1.Provider_PROVIDER_OSU_COLLECTOR,
		Kind:            osuv1.ItemKind_ITEM_KIND_COLLECTION,
		SourceId:        strconv.FormatUint(collection.ID, 10),
		Title:           collection.Name,
		Creator:         collection.Uploader.Username,
		Description:     collection.Description,
		BeatmapCount:    clampUint32(collection.BeatmapCount),
		FavouriteCount:  clampUint32(collection.Favourites),
		SubmittedAtIso:  collectorTime(collection.DateUploaded),
		UpdatedAtIso:    collectorTime(collection.DateLastModified),
		RulesetCounts:   collectorRulesetCounts(collection.Modes),
		DownloadHandoff: unavailableHandoff("osu!Collector does not expose a verified bulk lazer download contract. Choose a beatmapset from the collection."),
	}
	item.MinimumStars, item.MaximumStars = spreadRange(collection.DifficultySpread)
	item.MinimumBpm, item.MaximumBpm = spreadRange(collection.BPMSpread)
	return item
}

func normalizeCollectorDifficulty(beatmap *collectorBeatmap) *osuv1.BeatmapDifficulty {
	return &osuv1.BeatmapDifficulty{
		BeatmapId:         strconv.FormatUint(beatmap.ID, 10),
		BeatmapsetId:      strconv.FormatUint(beatmap.BeatmapsetID, 10),
		Checksum:          beatmap.Checksum,
		Name:              beatmap.Version,
		Ruleset:           rulesetFromName(beatmap.Mode),
		Status:            beatmap.Status,
		Stars:             beatmap.DifficultyRating,
		Bpm:               beatmap.BPM,
		ApproachRate:      beatmap.AR,
		CircleSize:        beatmap.CS,
		OverallDifficulty: beatmap.Accuracy,
		DrainRate:         beatmap.Drain,
		LengthSeconds:     beatmap.HitLength,
		Title:             beatmap.Beatmapset.Title,
		Artist:            beatmap.Beatmapset.Artist,
		Creator:           beatmap.Beatmapset.Creator,
		CoverUrl:          firstNonEmpty(beatmap.Beatmapset.Covers.Card, beatmap.Beatmapset.Covers.Cover),
		DownloadHandoff:   lazerHandoff(strconv.FormatUint(beatmap.BeatmapsetID, 10), false),
	}
}

func collectorRulesetCounts(modes map[string]uint64) []*osuv1.RulesetCount {
	counts := make(map[osuv1.Ruleset]uint32)
	for name, count := range modes {
		counts[rulesetFromName(name)] = clampUint32(count)
	}
	return sortedRulesetCounts(counts)
}

func spreadRange(spread map[string]uint64) (float64, float64) {
	values := make([]float64, 0, len(spread))
	for key, count := range spread {
		if count == 0 {
			continue
		}
		value, err := strconv.ParseFloat(key, 64)
		if err == nil {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return 0, 0
	}
	sort.Float64s(values)
	return values[0], values[len(values)-1]
}

func collectorTime(value collectorTimestamp) string {
	if value.Seconds <= 0 {
		return ""
	}
	return time.Unix(value.Seconds, 0).UTC().Format(time.RFC3339)
}

func collectionMatchesFilters(item *osuv1.BeatmapItem, filters *osuv1.BeatmapSearchFilters) bool {
	if filters == nil {
		return true
	}
	if filters.GetStatus() != "" || filters.GetLengthSeconds() != nil || filters.GetApproachRate() != nil || filters.GetCircleSize() != nil || filters.GetOverallDifficulty() != nil {
		return false
	}
	if ruleset := filters.GetRuleset(); ruleset != osuv1.Ruleset_RULESET_UNSPECIFIED {
		matched := false
		for _, count := range item.GetRulesetCounts() {
			if count.GetRuleset() == ruleset && count.GetCount() > 0 {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return rangesOverlap(item.MinimumStars, item.MaximumStars, filters.GetStars()) &&
		rangesOverlap(item.MinimumBpm, item.MaximumBpm, filters.GetBpm())
}

func rangesOverlap(itemMinimum, itemMaximum float64, filter *osuv1.NumberRange) bool {
	if filter == nil {
		return true
	}
	if itemMinimum == 0 && itemMaximum == 0 {
		return false
	}
	if filter.Minimum != nil && itemMaximum < filter.GetMinimum() {
		return false
	}
	if filter.Maximum != nil && itemMinimum > filter.GetMaximum() {
		return false
	}
	return true
}

func unsupportedCollectorFilters(filters *osuv1.BeatmapSearchFilters) []string {
	if filters == nil {
		return nil
	}
	unsupported := make([]string, 0, 6)
	if filters.GetStatus() != "" {
		unsupported = append(unsupported, "status")
	}
	if filters.GetLengthSeconds() != nil {
		unsupported = append(unsupported, "length")
	}
	if filters.GetApproachRate() != nil {
		unsupported = append(unsupported, "approach rate")
	}
	if filters.GetCircleSize() != nil {
		unsupported = append(unsupported, "circle size")
	}
	if filters.GetOverallDifficulty() != nil {
		unsupported = append(unsupported, "overall difficulty")
	}
	return unsupported
}

func collectorFilterMessage(filters *osuv1.BeatmapSearchFilters) string {
	unsupported := unsupportedCollectorFilters(filters)
	if len(unsupported) == 0 {
		return ""
	}
	return fmt.Sprintf("osu!Collector collection search cannot apply %s filters; its results were omitted.", strings.Join(unsupported, ", "))
}
