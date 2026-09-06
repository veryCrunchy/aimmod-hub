package osu

import (
	"context"
	"encoding/json"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

func (s *Server) retainPublicScores(ctx context.Context, scores []OfficialPublicScore) error {
	if s == nil || s.official == nil || s.official.playerIndex == nil {
		return nil
	}
	items := MergePublicScores(nil, scores)
	records := make([]store.IndexedOsuScore, 0, len(items))
	players := map[string]map[int64]store.OsuPublicProfile{}
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			return err
		}
		records = append(records, store.IndexedOsuScore{ID: item.OnlineScoreID, UserID: item.OsuUserID, Mode: item.Ruleset, PlayedAt: item.PlayedAt, HasReplay: item.OfficialReplayExists, Item: data})
		if item.OsuUsername != "" {
			if players[item.Ruleset] == nil {
				players[item.Ruleset] = map[int64]store.OsuPublicProfile{}
			}
			players[item.Ruleset][item.OsuUserID] = store.OsuPublicProfile{OsuUserID: item.OsuUserID, OsuUsername: item.OsuUsername, AvatarURL: item.AvatarURL, CountryCode: item.CountryCode}
		}
	}
	if err := s.official.playerIndex.SaveIndexedOsuScores(ctx, records); err != nil {
		return err
	}
	for mode, byID := range players {
		list := []store.OsuPublicProfile{}
		for _, p := range byID {
			list = append(list, p)
		}
		if err := s.official.playerIndex.SaveIndexedOsuPlayers(ctx, mode, list); err != nil {
			return err
		}
	}
	return nil
}
func (s *Server) ListPublicIndexedScores(ctx context.Context, limit int, replaysOnly bool) ([]OfficialPublicScore, error) {
	if s == nil || s.official == nil || s.official.playerIndex == nil {
		return []OfficialPublicScore{}, nil
	}
	rows, err := s.official.playerIndex.ListIndexedOsuScores(ctx, limit, replaysOnly)
	if err != nil {
		return nil, err
	}
	out := []OfficialPublicScore{}
	for _, row := range rows {
		var item PublicScoreItem
		if err = json.Unmarshal(row, &item); err != nil {
			return nil, err
		}
		out = append(out, OfficialPublicScore{Replay: item.OsuPublicReplay, OfficialReplayExists: item.OfficialReplayExists, PPCalculation: item.PPCalculation})
	}
	return out, nil
}

func (s *Server) IndexPublicScores(ctx context.Context, scores []OfficialPublicScore) error {
	return s.retainPublicScores(ctx, scores)
}
