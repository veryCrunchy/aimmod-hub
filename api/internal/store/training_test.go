package store

import (
	"math"
	"strings"
	"testing"
	"time"
)

func trainingFixture() TrainingSession {
	accuracy, spread, rate := 97.0, 20.0, 4.0
	return TrainingSession{ID: "00000000-0000-4000-8000-000000000001", CompletedAt: time.Now().UTC(), Visibility: "private",
		Setup: TrainingSetup{Mode: "steady", Engine: "osu-moving-v2", Bpm: 120, Seconds: 30, AimSpacing: 100, CircleSize: 4, ApproachRate: 7, SliderBeats: 1, MusicSource: "aimmod", ConfigurationHash: strings.Repeat("a", 64)},
		Notes: 100, Hits: 98, Within25: 90, Accuracy: &accuracy, SpreadMs: &spread, PeakNps: &rate, PlayedSeconds: 30}
}
func TestTrainingValidation(t *testing.T) {
	good := trainingFixture()
	if err := good.Validate(time.Now()); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*TrainingSession){
		"future": func(s *TrainingSession) { s.CompletedAt = time.Now().Add(time.Hour) }, "unknown mode": func(s *TrainingSession) { s.Setup.Mode = "unknown" },
		"private path": func(s *TrainingSession) { s.Setup.ConfigurationHash = "song.osu" }, "hits exceed notes": func(s *TrainingSession) { s.Hits = s.Notes + 1 },
		"negative spread": func(s *TrainingSession) { v := -1.; s.SpreadMs = &v }, "nan": func(s *TrainingSession) { v := math.NaN(); s.Accuracy = &v },
		"infinite": func(s *TrainingSession) { s.PlayedSeconds = math.Inf(1) }, "too long": func(s *TrainingSession) { s.PlayedSeconds = 400 },
		"unlisted": func(s *TrainingSession) { s.Visibility = "unlisted" }, "accuracy scale": func(s *TrainingSession) { v := 101.; s.Accuracy = &v },
	} {
		t.Run(name, func(t *testing.T) {
			bad := good
			mutate(&bad)
			if bad.Validate(time.Now()) == nil {
				t.Fatal("accepted invalid session")
			}
		})
	}
}
func TestTrainingProgressRequiresComparableNonOverlappingRuns(t *testing.T) {
	sessions := []TrainingSession{}
	for i := 0; i < 6; i++ {
		s := trainingFixture()
		s.CompletedAt = s.CompletedAt.Add(time.Duration(i) * time.Hour)
		acc := 94. + float64(i)
		spread := 25. - float64(i)
		s.Accuracy = &acc
		s.SpreadMs = &spread
		sessions = append(sessions, s)
	}
	profile := SummarizeTraining(sessions, 30, false)
	skill := profile.Skills[0]
	if skill.ComparedSessions != 6 || skill.AccuracyChange == nil || *skill.AccuracyChange != 3 || *skill.SpreadChange != -3 {
		t.Fatalf("bad progress: %+v", skill)
	}
	if profile.Seconds != 180 || profile.Sessions != 6 || len(profile.Recent) != 6 {
		t.Fatalf("bad totals: %+v", profile)
	}
	for _, count := range []int{1, 3, 5} {
		if SummarizeTraining(sessions[:count], 30, false).Skills[0].ComparedSessions != 0 {
			t.Fatal("overlapping baseline")
		}
	}
	sessions[5].Setup.Bpm = 150
	if SummarizeTraining(sessions, 30, false).Skills[0].ComparedSessions != 0 {
		t.Fatal("mixed BPM")
	}
	sessions[5].Setup.Bpm = 120
	sessions[5].Setup.ConfigurationHash = strings.Repeat("b", 64)
	if SummarizeTraining(sessions, 30, false).Skills[0].ComparedSessions != 0 {
		t.Fatal("mixed song or settings")
	}
	for i := range sessions {
		sessions[i].Setup.Randomized = true
	}
	if SummarizeTraining(sessions, 30, false).Skills[0].ComparedSessions != 0 {
		t.Fatal("compared random layouts")
	}
}
func TestTrainingCleanPaceNeedsRepeatedSuccessfulRuns(t *testing.T) {
	sessions := []TrainingSession{trainingFixture(), trainingFixture(), trainingFixture()}
	if SummarizeTraining(sessions[:2], 30, false).Skills[0].CleanPeakNps != nil {
		t.Fatal("single-run skill claim")
	}
	if v := SummarizeTraining(sessions, 30, false).Skills[0].CleanPeakNps; v == nil || *v != 4 {
		t.Fatal("missing supported pace")
	}
	sessions[0].Extras = 20
	if SummarizeTraining(sessions, 30, false).Skills[0].CleanPeakNps != nil {
		t.Fatal("spam counted as clean")
	}
}
func TestTrainingEmptyAndTruncatedCoverage(t *testing.T) {
	p := SummarizeTraining(nil, 90, true)
	if p.Skills == nil || p.Activity == nil || p.Recent == nil || !p.Truncated || p.Days != 90 {
		t.Fatal("invalid empty result")
	}
}
func TestTrainingInstalledSongDoesNotClaimFixedTempo(t *testing.T) {
	sessions := make([]TrainingSession, 6)
	for i := range sessions {
		sessions[i] = trainingFixture()
		sessions[i].Setup.MusicSource = "installed"
	}
	skill := SummarizeTraining(sessions, 30, false).Skills[0]
	if skill.ComparedSessions != 6 || skill.ComparisonBpm != 0 {
		t.Fatal("installed timing reported as fixed BPM")
	}
}
