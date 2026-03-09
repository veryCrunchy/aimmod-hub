package service

import (
	"fmt"
	"math"
	"strings"
	"time"

	"connectrpc.com/connect"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	hubv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1"
)

func validateIngestRequest(req *hubv1.IngestSessionRequest) error {
	if strings.TrimSpace(req.GetUserExternalId()) == "" {
		return fmt.Errorf("user_external_id is required")
	}
	if strings.TrimSpace(req.GetSessionId()) == "" {
		return fmt.Errorf("session_id is required")
	}
	if strings.TrimSpace(req.GetScenarioName()) == "" {
		return fmt.Errorf("scenario_name is required")
	}
	if strings.TrimSpace(req.GetPlayedAtIso()) == "" {
		return fmt.Errorf("played_at_iso is required")
	}
	return nil
}

func parsePlayedAt(value string) (time.Time, error) {
	playedAt, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return playedAt.UTC(), nil
	}
	playedAt, err = time.Parse(time.RFC3339, value)
	if err == nil {
		return playedAt.UTC(), nil
	}
	return time.Time{}, err
}

func invalidArgument(err error) error {
	return connect.NewError(connect.CodeInvalidArgument, err)
}

func summaryNumber(summary map[string]*hubv1.SessionSummaryValue, key string) (float64, bool) {
	value, ok := summary[key]
	if !ok || value == nil || value.Kind == nil {
		return 0, false
	}
	number, ok := value.Kind.(*hubv1.SessionSummaryValue_NumberValue)
	if !ok {
		return 0, false
	}
	return number.NumberValue, true
}

func deriveAccuracyPercent(shots uint32, hits uint32) (float64, bool) {
	if shots == 0 {
		return 0, false
	}
	return (float64(hits) / float64(shots)) * 100.0, true
}

func normalizeAccuracyPercent(raw float64, shots uint32, hits uint32, fallbackPercent *float64) float64 {
	if derived, ok := deriveAccuracyPercent(shots, hits); ok {
		if raw > 1.0 {
			if math.Abs(raw-derived) <= 1.0 {
				return raw
			}
			return derived
		}
		if raw >= 0.0 {
			if math.Abs((raw*100.0)-derived) <= 1.0 {
				return derived
			}
			return derived
		}
		return derived
	}

	if fallbackPercent != nil && *fallbackPercent > 1.0 {
		if raw <= 1.0 {
			return *fallbackPercent
		}
		return raw
	}

	if raw > 1.0 {
		return raw
	}
	if raw >= 0.0 {
		return raw * 100.0
	}
	return raw
}

func buildIngestedRun(req *hubv1.IngestSessionRequest) (store.IngestedRun, error) {
	if err := validateIngestRequest(req); err != nil {
		return store.IngestedRun{}, invalidArgument(err)
	}

	playedAt, err := parsePlayedAt(req.GetPlayedAtIso())
	if err != nil {
		return store.IngestedRun{}, invalidArgument(fmt.Errorf("played_at_iso must be RFC3339: %w", err))
	}

	summaryJSON, err := store.SummaryMapToJSON(req.GetSummary())
	if err != nil {
		return store.IngestedRun{}, connect.NewError(connect.CodeInvalidArgument, err)
	}

	var csvAccuracyPtr *float64
	if csvAccuracy, ok := summaryNumber(req.GetSummary(), "csvAccuracy"); ok {
		csvAccuracyPtr = &csvAccuracy
	}

	featureJSON, err := store.SummaryMapToJSON(req.GetFeatureSet())
	if err != nil {
		return store.IngestedRun{}, connect.NewError(connect.CodeInvalidArgument, err)
	}

	timeline := make([]store.TimelineSecond, 0, len(req.GetTimelineSeconds()))
	for _, point := range req.GetTimelineSeconds() {
		if point == nil {
			continue
		}
		normalizedAccuracy := normalizeAccuracyPercent(
			point.GetAccuracy(),
			point.GetShots(),
			point.GetHits(),
			csvAccuracyPtr,
		)
		timeline = append(timeline, store.TimelineSecond{
			Second:    point.GetTSec(),
			Score:     point.GetScore(),
			Accuracy:  normalizedAccuracy,
			DamageEff: point.GetDamageEff(),
			SPM:       point.GetSpm(),
			Shots:     point.GetShots(),
			Hits:      point.GetHits(),
			Kills:     point.GetKills(),
			Paused:    point.GetPaused(),
		})
	}

	contextWindows := make([]store.ContextWindow, 0, len(req.GetContextWindows()))
	for _, window := range req.GetContextWindows() {
		if window == nil {
			continue
		}
		featureSummaryJSON, err := store.SummaryMapToJSON(window.GetFeatureSummary())
		if err != nil {
			return store.IngestedRun{}, connect.NewError(connect.CodeInvalidArgument, err)
		}
		contextWindows = append(contextWindows, store.ContextWindow{
			StartMS:            window.GetStartMs(),
			EndMS:              window.GetEndMs(),
			WindowType:         window.GetWindowType(),
			Label:              window.GetLabel(),
			FeatureSummaryJSON: featureSummaryJSON,
			CoachingTags:       append([]string(nil), window.GetCoachingTags()...),
		})
	}

	var finalShots uint32
	var finalHits uint32
	for _, point := range req.GetTimelineSeconds() {
		if point == nil {
			continue
		}
		if point.GetShots() >= finalShots {
			finalShots = point.GetShots()
			finalHits = point.GetHits()
		}
	}

	normalizedRunAccuracy := normalizeAccuracyPercent(req.GetAccuracy(), finalShots, finalHits, csvAccuracyPtr)

	return store.IngestedRun{
		AppVersion:     req.GetAppVersion(),
		SchemaVersion:  req.GetSchemaVersion(),
		UserExternalID: req.GetUserExternalId(),
		SessionID:      req.GetSessionId(),
		ScenarioName:   req.GetScenarioName(),
		ScenarioType:   req.GetScenarioType(),
		Score:          req.GetScore(),
		Accuracy:       normalizedRunAccuracy,
		DurationMS:     req.GetDurationMs(),
		PlayedAt:       playedAt,
		SummaryJSON:    summaryJSON,
		FeatureJSON:    featureJSON,
		Timeline:       timeline,
		ContextWindows: contextWindows,
	}, nil
}

func validateDiscordLink(req *hubv1.LinkDiscordAccountRequest) error {
	if strings.TrimSpace(req.GetUserExternalId()) == "" {
		return fmt.Errorf("user_external_id is required")
	}
	if strings.TrimSpace(req.GetDiscordUserId()) == "" {
		return fmt.Errorf("discord_user_id is required")
	}
	if strings.TrimSpace(req.GetUsername()) == "" {
		return fmt.Errorf("username is required")
	}
	return nil
}

func buildDiscordLink(req *hubv1.LinkDiscordAccountRequest) (store.DiscordLink, error) {
	if err := validateDiscordLink(req); err != nil {
		return store.DiscordLink{}, invalidArgument(err)
	}
	return store.DiscordLink{
		UserExternalID: req.GetUserExternalId(),
		DiscordUserID:  req.GetDiscordUserId(),
		Username:       req.GetUsername(),
		GlobalName:     req.GetGlobalName(),
		AvatarURL:      req.GetAvatarUrl(),
	}, nil
}
