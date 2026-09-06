package httpserver

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

type osuMetadataStore interface {
	GetOsuPublicReplay(context.Context, string) (store.OsuPublicReplay, error)
	GetOsuPublicProfile(context.Context, string, int) (store.OsuPublicProfile, error)
}

func unavailableOsuMeta(canonical string) pageMeta {
	return pageMeta{Title: "osu! share unavailable · AimMod Hub", Description: "Browse public osu! players and shared replays on AimMod Hub.", OGType: "website", Canonical: canonical, NoIndex: true}
}

func resolveOsuDetailMeta(ctx context.Context, route, canonical string, st osuMetadataStore) pageMeta {
	fallback := unavailableOsuMeta(canonical)
	if raw, ok := strings.CutPrefix(route, "/osu/scores/"); ok {
		id, err := strconv.ParseInt(raw, 10, 64)
		index, available := st.(interface {
			GetIndexedOsuScore(context.Context, int64) (store.OsuPublicReplay, error)
		})
		if err != nil || id <= 0 || !available {
			return fallback
		}
		score, err := index.GetIndexedOsuScore(ctx, id)
		if err != nil || score.Visibility != store.OsuVisibilityPublic {
			return fallback
		}
		return pageMeta{Title: fmt.Sprintf("%s - %s [%s] · osu! score · AimMod Hub", score.Artist, score.Title, score.Difficulty), Description: fmt.Sprintf("%.2f%% accuracy by %s. Public osu! score and replay on AimMod Hub.", score.Accuracy*100, score.OsuUsername), OGType: "website", Canonical: canonical}
	}
	if id, ok := strings.CutPrefix(route, "/osu/replays/"); ok {
		if id == "" || strings.Contains(id, "/") {
			return fallback
		}
		replay, err := st.GetOsuPublicReplay(ctx, id)
		// Unlisted link access is not consent to publish identifying social-preview metadata.
		if err != nil || replay.Visibility != store.OsuVisibilityPublic {
			return fallback
		}
		return pageMeta{Title: fmt.Sprintf("%s - %s [%s] · osu! replay · AimMod Hub", replay.Artist, replay.Title, replay.Difficulty),
			Description: fmt.Sprintf("%.2f%% accuracy by %s. Public osu! replay analysis on AimMod Hub.", replay.Accuracy*100, replay.OsuUsername),
			OGType:      "website", Canonical: canonical}
	}
	handle := strings.TrimPrefix(route, "/osu/profiles/")
	if handle == "" || strings.Contains(handle, "/") {
		return fallback
	}
	profile, err := st.GetOsuPublicProfile(ctx, handle, 1)
	if err != nil || profile.SharedReplayCount <= 0 {
		if index, ok := st.(interface {
			GetIndexedOsuProfile(context.Context, string, string) (store.OsuPublicProfile, error)
		}); ok {
			profile, err = index.GetIndexedOsuProfile(ctx, handle, "")
			if err == nil && profile.OsuUserID > 0 {
				prefix := strings.TrimSuffix(canonical, "/"+handle)
				return pageMeta{Title: profile.OsuUsername + " · osu! player · AimMod Hub", Description: profile.OsuUsername + "'s public osu! profile, performance and scores.", OGType: "profile", Canonical: prefix + "/" + strconv.FormatInt(profile.OsuUserID, 10)}
			}
		}
		return fallback
	}
	return pageMeta{Title: profile.OsuUsername + " · osu! player · AimMod Hub",
		Description: fmt.Sprintf("%d public osu! replays shared by %s on AimMod Hub.", profile.SharedReplayCount, profile.OsuUsername), OGType: "profile", Canonical: canonical}
}
