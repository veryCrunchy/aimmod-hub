package osu

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

// GetPublicScoreMetadata never contacts osu!, refreshes OAuth, or writes to the
// index. A crawl of distinct HTML pages must not consume the interactive quota.
func (s *Server) GetPublicScoreMetadata(ctx context.Context, id int64) (OfficialScoreDetail, error) {
	unavailable := OfficialScoreDetail{Status: "unavailable"}
	if err := ctx.Err(); err != nil {
		return unavailable, err
	}
	if s == nil || s.official == nil || id <= 0 {
		return unavailable, nil
	}
	a := s.official
	key := "GET score-v20220705 " + a.client.resolve("/api/v2/scores/"+strconv.FormatInt(id, 10), nil)
	if body, ok := a.client.cache.get(key); ok {
		var score officialScore
		if json.Unmarshal(body, &score) == nil && score.ID == id && score.UserID > 0 && score.RulesetID != nil && scoreMode(*score.RulesetID) != "" {
			items := MergePublicScores(nil, []OfficialPublicScore{normalizePublicScore(score, scoreMode(*score.RulesetID))})
			if len(items) == 1 {
				return OfficialScoreDetail{Status: "available", Item: &items[0]}, nil
			}
		}
	}
	reader, ok := a.playerIndex.(interface {
		GetIndexedOsuScoreMetadata(context.Context, int64) (json.RawMessage, error)
	})
	if !ok {
		return unavailable, nil
	}
	readCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	body, err := reader.GetIndexedOsuScoreMetadata(readCtx, id)
	if err != nil {
		return unavailable, err
	}
	var item PublicScoreItem
	if json.Unmarshal(body, &item) != nil || item.Source != "official" || item.Visibility != store.OsuVisibilityPublic || item.OnlineScoreID != id || item.OfficialScoreID != strconv.FormatInt(id, 10) || item.OsuUserID <= 0 || item.BeatmapID <= 0 {
		return unavailable, nil
	}
	return OfficialScoreDetail{Status: "available", Item: &item}, nil
}
