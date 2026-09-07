package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"time"
)

// Training payloads deliberately exclude input configuration, local paths and replay data.
type TrainingSetup struct {
	Mode         string `json:"mode"`
	Engine       string `json:"engine"`
	Bpm          int    `json:"bpm"`
	Seconds      int    `json:"seconds"`
	Pattern      int    `json:"pattern"`
	NoteSpeed    int    `json:"noteSpeed"`
	AimStyle     int    `json:"aimStyle"`
	AimSpacing   int    `json:"aimSpacing"`
	CircleSize   int    `json:"circleSize"`
	ApproachRate int    `json:"approachRate"`
	Sliders      int    `json:"sliders"`
	SliderBeats  int    `json:"sliderBeats"`
	PathStyle    int    `json:"pathStyle"`
	Randomized   bool   `json:"randomized"`
	MusicSource  string `json:"musicSource"`
	// Opaque digest of timing-affecting settings and source identity, never a local filename.
	ConfigurationHash string `json:"configurationHash"`
}
type TrainingSession struct {
	ID            string        `json:"id"`
	CompletedAt   time.Time     `json:"completedAt"`
	Visibility    string        `json:"visibility"`
	Setup         TrainingSetup `json:"setup"`
	Notes         int           `json:"notes"`
	Hits          int           `json:"hits"`
	Within25      int           `json:"within25"`
	Extras        int           `json:"extras"`
	RepeatedKeys  int           `json:"repeatedKeys"`
	Accuracy      *float64      `json:"accuracy"`
	MeanMs        *float64      `json:"meanMs"`
	SpreadMs      *float64      `json:"spreadMs"`
	DriftMs       *float64      `json:"driftMs"`
	ResponseMs    *float64      `json:"responseMs"`
	PlayedSeconds float64       `json:"playedSeconds"`
	PeakNps       *float64      `json:"peakNps"`
	JumpDistance  *float64      `json:"jumpDistance"`
	AimVelocity   *float64      `json:"aimVelocity"`
	LongestChain  *int          `json:"longestChain"`
}

var trainingID = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
var trainingHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

func ValidTrainingID(id string) bool { return trainingID.MatchString(id) }

var TrainingModes = []string{"steady", "alternating", "bursts", "rhythm", "aim", "reading", "reaction"}
var ErrTrainingConflict = errors.New("training session already exists with different results")

func ValidTrainingMode(mode string) bool {
	for _, m := range TrainingModes {
		if m == mode {
			return true
		}
	}
	return false
}
func (s TrainingSession) Validate(now time.Time) error {
	invalid := func() error { return fmt.Errorf("invalid training session") }
	p := s.Setup
	if !trainingID.MatchString(s.ID) || s.CompletedAt.IsZero() || s.CompletedAt.After(now.Add(5*time.Minute)) || s.CompletedAt.Before(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) {
		return invalid()
	}
	if s.Visibility != "private" && s.Visibility != "public" {
		return invalid()
	}
	if !ValidTrainingMode(p.Mode) || !trainingHash.MatchString(p.ConfigurationHash) {
		return invalid()
	}
	switch p.Engine {
	case "cue", "osu", "osu-moving-v2", "osu-patterns-v3", "osu-adaptive-v4":
	default:
		return invalid()
	}
	switch p.MusicSource {
	case "cues", "aimmod", "installed":
	default:
		return invalid()
	}
	if p.Bpm < 60 || p.Bpm > 240 || p.Seconds < 15 || p.Seconds > 180 || p.Pattern < 0 || p.Pattern > 64 || p.NoteSpeed < 0 || p.NoteSpeed > 16 || p.AimStyle < 0 || p.AimStyle > 16 || p.AimSpacing < 70 || p.AimSpacing > 140 || p.CircleSize < 3 || p.CircleSize > 6 || p.ApproachRate < 3 || p.ApproachRate > 10 || p.Sliders < 0 || p.Sliders > 16 || p.SliderBeats < 1 || p.SliderBeats > 4 || p.PathStyle < 0 || p.PathStyle > 16 {
		return invalid()
	}
	if s.Notes < 1 || s.Notes > 20000 || s.Hits < 0 || s.Hits > s.Notes || s.Within25 < 0 || s.Within25 > s.Hits || s.Extras < 0 || s.Extras > 100000 || s.RepeatedKeys < 0 || s.RepeatedKeys > s.Hits {
		return invalid()
	}
	if !finiteRange(s.PlayedSeconds, 1, float64(p.Seconds)+30) {
		return invalid()
	}
	for _, v := range []struct {
		value    *float64
		min, max float64
	}{
		{s.Accuracy, 0, 100}, {s.MeanMs, -1000, 1000}, {s.SpreadMs, 0, 1000}, {s.DriftMs, -2000, 2000}, {s.ResponseMs, 0, 10000}, {s.PeakNps, 0, 100}, {s.JumpDistance, 0, 1000}, {s.AimVelocity, 0, 100000},
	} {
		if v.value != nil && !finiteRange(*v.value, v.min, v.max) {
			return invalid()
		}
	}
	if s.LongestChain != nil && (*s.LongestChain < 0 || *s.LongestChain > s.Notes) {
		return invalid()
	}
	if s.Hits == 0 && (s.MeanMs != nil || s.SpreadMs != nil || s.ResponseMs != nil) {
		return invalid()
	}
	return nil
}
func finiteRange(x, min, max float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0) && x >= min && x <= max
}

func (s *Store) SaveTraining(ctx context.Context, userID int64, sessions []TrainingSession) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, session := range sessions {
		if err := session.Validate(time.Now()); err != nil {
			return err
		}
		payload, err := json.Marshal(session)
		if err != nil {
			return err
		}
		// Visibility is managed separately; replaying an old upload must never re-publish a hidden session.
		result := session
		result.Visibility = ""
		content, _ := json.Marshal(result)
		hash := sha256.Sum256(content)
		tag, err := tx.Exec(ctx, `INSERT INTO osu_training_sessions(user_id,session_id,completed_at,mode,visibility,content_hash,session_json)
   VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(user_id,session_id) DO UPDATE SET session_id=EXCLUDED.session_id
		   WHERE osu_training_sessions.content_hash=EXCLUDED.content_hash`, userID, session.ID, session.CompletedAt, session.Setup.Mode, session.Visibility, hex.EncodeToString(hash[:]), string(payload))
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrTrainingConflict
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) ListTraining(ctx context.Context, handle string, ownerID int64, days int, mode string) ([]TrainingSession, bool, error) {
	rows, err := s.pool.Query(ctx, `SELECT t.session_json, t.visibility FROM osu_training_sessions t JOIN hub_user_identity h ON h.user_id=t.user_id
 WHERE (($2::bigint>0 AND t.user_id=$2) OR ($2=0 AND lower(h.user_handle)=lower($1) AND t.visibility='public'))
 AND t.completed_at >= NOW()-($3::int * interval '1 day') AND ($4='' OR t.mode=$4)
 ORDER BY t.completed_at DESC, t.session_id LIMIT 5001`, handle, ownerID, days, mode)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	sessions := []TrainingSession{}
	for rows.Next() {
		var raw []byte
		var visibility string
		if err := rows.Scan(&raw, &visibility); err != nil {
			return nil, false, err
		}
		var session TrainingSession
		if err := json.Unmarshal(raw, &session); err != nil {
			return nil, false, err
		}
		session.Visibility = visibility
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	truncated := len(sessions) > 5000
	if truncated {
		sessions = sessions[:5000]
	}
	return sessions, truncated, nil
}
func (s *Store) SetTrainingVisibility(ctx context.Context, userID int64, id, visibility string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE osu_training_sessions SET visibility=$3 WHERE user_id=$1 AND session_id=$2`, userID, id, visibility)
	return tag.RowsAffected() > 0, err
}

type TrainingSkill struct {
	MedianResponseMs *float64 `json:"medianResponseMs"`
	MedianSpreadMs   *float64 `json:"medianSpreadMs"`
	ResponseChange   *float64 `json:"responseChange"`
	Mode             string   `json:"mode"`
	Sessions         int      `json:"sessions"`
	Seconds          float64  `json:"seconds"`
	CleanSessions    int      `json:"cleanSessions"`
	CleanPeakNps     *float64 `json:"cleanPeakNps"`
	AccuracyChange   *float64 `json:"accuracyChange"`
	SpreadChange     *float64 `json:"spreadChange"`
	ComparedSessions int      `json:"comparedSessions"`
	ComparisonBpm    int      `json:"comparisonBpm"`
}
type TrainingDay struct {
	Date     string  `json:"date"`
	Sessions int     `json:"sessions"`
	Seconds  float64 `json:"seconds"`
}
type TrainingProfile struct {
	Sessions   int               `json:"sessions"`
	Seconds    float64           `json:"seconds"`
	ActiveDays int               `json:"activeDays"`
	Days       int               `json:"days"`
	Truncated  bool              `json:"truncated"`
	Skills     []TrainingSkill   `json:"skills"`
	Activity   []TrainingDay     `json:"activity"`
	Recent     []TrainingSession `json:"recent"`
}

// Compare the first and last three runs of one matching setup. Never mix cue accuracy with osu judgements.
func SummarizeTraining(sessions []TrainingSession, days int, truncated bool) TrainingProfile {
	p := TrainingProfile{Sessions: len(sessions), Days: days, Truncated: truncated, Skills: []TrainingSkill{}, Activity: []TrainingDay{}, Recent: []TrainingSession{}}
	ordered := append([]TrainingSession{}, sessions...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].CompletedAt.Before(ordered[j].CompletedAt) })
	activity := map[string]*TrainingDay{}
	for _, s := range ordered {
		p.Seconds += s.PlayedSeconds
		date := s.CompletedAt.UTC().Format("2006-01-02")
		if activity[date] == nil {
			activity[date] = &TrainingDay{Date: date}
		}
		activity[date].Sessions++
		activity[date].Seconds += s.PlayedSeconds
	}
	for _, day := range activity {
		p.Activity = append(p.Activity, *day)
	}
	sort.Slice(p.Activity, func(i, j int) bool { return p.Activity[i].Date < p.Activity[j].Date })
	p.ActiveDays = len(activity)
	for _, mode := range TrainingModes {
		skill := TrainingSkill{Mode: mode}
		cohorts := map[string][]TrainingSession{}
		cleanRates := []float64{}
		responses, spreads := []float64{}, []float64{}
		for _, s := range ordered {
			if s.Setup.Mode != mode {
				continue
			}
			skill.Sessions++
			skill.Seconds += s.PlayedSeconds
			if s.ResponseMs != nil {
				responses = append(responses, *s.ResponseMs)
			}
			if s.SpreadMs != nil {
				spreads = append(spreads, *s.SpreadMs)
			}
			if s.Setup.Engine != "cue" && s.Accuracy != nil && *s.Accuracy >= 95 && float64(s.Hits)/float64(s.Notes) >= .95 && s.Extras <= s.Notes/20 && s.SpreadMs != nil && *s.SpreadMs <= 25 {
				skill.CleanSessions++
				if s.PeakNps != nil {
					cleanRates = append(cleanRates, *s.PeakNps)
				}
			}
			// Random layouts document practice, but are not controlled improvement comparisons.
			if !s.Setup.Randomized && (s.Setup.Engine != "cue" || s.Setup.Mode == "reaction") {
				raw, _ := json.Marshal(s.Setup)
				key := string(raw)
				cohorts[key] = append(cohorts[key], s)
			}
		}
		if len(cleanRates) >= 3 {
			sort.Float64s(cleanRates)
			value := cleanRates[int(float64(len(cleanRates)-1)*.25)]
			skill.CleanPeakNps = &value
		}
		median := func(values []float64) *float64 {
			if len(values) == 0 {
				return nil
			}
			sort.Float64s(values)
			value := (values[(len(values)-1)/2] + values[len(values)/2]) / 2
			return &value
		}
		skill.MedianResponseMs = median(responses)
		skill.MedianSpreadMs = median(spreads)
		var selected []TrainingSession
		var selectedKey string
		for key, group := range cohorts {
			if len(group) < 6 {
				continue
			}
			if len(selected) == 0 || group[len(group)-1].CompletedAt.After(selected[len(selected)-1].CompletedAt) || group[len(group)-1].CompletedAt.Equal(selected[len(selected)-1].CompletedAt) && key < selectedKey {
				selected = group
				selectedKey = key
			}
		}
		if len(selected) >= 6 {
			skill.ComparedSessions = 6
			if selected[0].Setup.MusicSource != "installed" {
				skill.ComparisonBpm = selected[0].Setup.Bpm
			}
			delta := func(get func(TrainingSession) *float64) *float64 {
				a, b := 0.0, 0.0
				for i := 0; i < 3; i++ {
					x, y := get(selected[i]), get(selected[len(selected)-3+i])
					if x == nil || y == nil {
						return nil
					}
					a += *x
					b += *y
				}
				value := (b - a) / 3
				return &value
			}
			skill.AccuracyChange = delta(func(s TrainingSession) *float64 { return s.Accuracy })
			skill.SpreadChange = delta(func(s TrainingSession) *float64 { return s.SpreadMs })
			skill.ResponseChange = delta(func(s TrainingSession) *float64 { return s.ResponseMs })
		}
		if skill.Sessions > 0 {
			p.Skills = append(p.Skills, skill)
		}
	}
	for i := len(ordered) - 1; i >= 0 && len(p.Recent) < 50; i-- {
		p.Recent = append(p.Recent, ordered[i])
	}
	return p
}
