package service

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

// ── Benchmark catalog quality ────────────────────────────────────────────────

// minBenchmarkPlayers is the KovaaK's player estimate below which a
// benchmark is hidden from the default catalog.
const minBenchmarkPlayers = 25

var (
	longDigitRun  = regexp.MustCompile(`\d{6,}`)
	testLikeNames = map[string]bool{
		"test": true, "testing": true, "tests": true, "asdf": true, "aaa": true, "new benchmark": true,
		"benchmark": true, "my benchmark": true, "untitled": true, "temp": true, "qwerty": true,
	}
)

// benchmarkQuality decides whether a benchmark belongs in the default
// catalog using KovaaK's own signals, never whether AimMod players ranked it.
// synced reports whether the definition (scenario list) has been read.
func benchmarkQuality(name string, scenarioCount int, synced bool, playerEstimate *int64) (hidden bool, reason string) {
	trimmed := strings.TrimSpace(name)
	lower := strings.ToLower(trimmed)
	letters := 0
	for _, r := range trimmed {
		if unicode.IsLetter(r) {
			letters++
		}
	}
	switch {
	case trimmed == "" || letters < 2:
		return true, "Name looks like a placeholder"
	case testLikeNames[lower] || strings.HasPrefix(lower, "test ") || strings.HasSuffix(lower, " test"):
		return true, "Looks like a test benchmark"
	case longDigitRun.MatchString(trimmed):
		return true, "Name looks like a test (long run of digits)"
	case repeatedChunk(lower):
		return true, "Name looks like a test (repeated characters)"
	case synced && scenarioCount == 0:
		return true, "No scenarios"
	case playerEstimate != nil && *playerEstimate < minBenchmarkPlayers:
		return true, "Barely played on KovaaK's"
	}
	return false, ""
}

// repeatedChunk catches names like "aaaaaa" or "abcabcabc".
func repeatedChunk(value string) bool {
	compact := strings.ReplaceAll(value, " ", "")
	if len(compact) < 6 {
		return false
	}
	for size := 1; size <= 3; size++ {
		if len(compact)%size != 0 {
			continue
		}
		chunk := compact[:size]
		if strings.Repeat(chunk, len(compact)/size) == compact {
			return true
		}
	}
	return false
}

// ── Search ranking ───────────────────────────────────────────────────────────

func normalizeSearchText(value string) string {
	var b strings.Builder
	lastSpace := true
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

// matchScore rates how well text matches a query, from 0 (no match) to 1
// (exact). It prefers exact, then prefix, then word-prefix, then substring
// matches, and finally texts that contain every query word.
func matchScore(query, text string) float64 {
	q := normalizeSearchText(query)
	t := normalizeSearchText(text)
	if q == "" || t == "" {
		return 0
	}
	compactQ := strings.ReplaceAll(q, " ", "")
	compactT := strings.ReplaceAll(t, " ", "")
	switch {
	case t == q || compactT == compactQ:
		return 1
	case strings.HasPrefix(t, q) || strings.HasPrefix(compactT, compactQ):
		return 0.85
	case strings.Contains(" "+t, " "+q):
		return 0.7
	case strings.Contains(compactT, compactQ):
		return 0.5
	}
	words := strings.Fields(q)
	if len(words) > 1 {
		for _, word := range words {
			if !strings.Contains(t, word) {
				return 0
			}
		}
		return 0.4
	}
	return 0
}

// relevance combines match quality with popularity so that, between equally
// good matches, the more played item comes first.
func relevance(match float64, popularity uint64) float64 {
	if match <= 0 {
		return 0
	}
	return match*100 + math.Log10(1+float64(popularity))*2.2
}

// ── Score statistics ─────────────────────────────────────────────────────────

// percentileScores returns the score at each requested percentile of a
// descending list of player bests (nearest-rank method on ascending order).
func percentileScores(bestsDesc []float64, percentiles []uint32) map[uint32]float64 {
	out := map[uint32]float64{}
	n := len(bestsDesc)
	if n == 0 {
		return out
	}
	for _, p := range percentiles {
		if p > 100 {
			continue
		}
		// ascending index for percentile p
		idx := int(math.Ceil(float64(p)/100*float64(n))) - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= n {
			idx = n - 1
		}
		out[p] = bestsDesc[n-1-idx]
	}
	return out
}

// beatsShare is the share of other players with a strictly lower best, 0-100.
func beatsShare(below, players uint32) float64 {
	if players <= 1 {
		return 100
	}
	return math.Round(float64(below)/float64(players-1)*1000) / 10
}

// consistencyScore turns the spread of recent scores into 0-100, where 100
// means every run scored the same. Fewer than three runs gives 0 (unknown).
func consistencyScore(scores []float64) float64 {
	if len(scores) < 3 {
		return 0
	}
	var sum float64
	for _, s := range scores {
		sum += s
	}
	mean := sum / float64(len(scores))
	if mean <= 0 {
		return 0
	}
	var variance float64
	for _, s := range scores {
		variance += (s - mean) * (s - mean)
	}
	variance /= float64(len(scores))
	cv := math.Sqrt(variance) / mean
	score := 100 * (1 - cv*2.5)
	return math.Round(math.Max(0, math.Min(100, score))*10) / 10
}

// trendPercent compares the average of the newest five runs with the five
// before them. Scores are newest first. Fewer than four runs gives 0.
func trendPercent(newestFirst []float64) float64 {
	if len(newestFirst) < 4 {
		return 0
	}
	window := 5
	if len(newestFirst) < 2*window {
		window = len(newestFirst) / 2
	}
	avg := func(values []float64) float64 {
		var sum float64
		for _, v := range values {
			sum += v
		}
		return sum / float64(len(values))
	}
	recent := avg(newestFirst[:window])
	previous := avg(newestFirst[window : 2*window])
	if previous <= 0 {
		return 0
	}
	return math.Round((recent-previous)/previous*1000) / 10
}

// ── Benchmark rank history ──────────────────────────────────────────────────

type historyScenario struct {
	Name      string
	RankMaxes []float64
}

type historyRun struct {
	ScenarioName string
	Score        float64
	PlayedAt     time.Time
}

type historyPoint struct {
	Day              time.Time
	OverallRankIndex uint32
	AverageRankIndex float64
	RankedScenarios  uint32
}

// scenarioRankIndex is the highest rank whose threshold the score meets.
// rankMaxes[n] is the minimum score for rank n+1.
func scenarioRankIndex(score float64, rankMaxes []float64) uint32 {
	rank := uint32(0)
	for i, threshold := range rankMaxes {
		if score >= threshold {
			rank = uint32(i + 1)
		} else {
			break
		}
	}
	return rank
}

// benchmarkRankHistory replays a player's runs day by day and reports how
// their benchmark standing changed. The overall rank uses the same
// weakest-link rule as the benchmark page.
func benchmarkRankHistory(scenarios []historyScenario, runs []historyRun) []historyPoint {
	if len(scenarios) == 0 || len(runs) == 0 {
		return nil
	}
	index := map[string]int{}
	for i, scenario := range scenarios {
		index[strings.ToLower(strings.TrimSpace(scenario.Name))] = i
	}
	sorted := append([]historyRun(nil), runs...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].PlayedAt.Before(sorted[j].PlayedAt) })

	best := make([]float64, len(scenarios))
	var out []historyPoint
	var last *historyPoint
	flush := func(day time.Time) {
		var sum float64
		var ranked uint32
		overall := uint32(math.MaxUint32)
		for i, scenario := range scenarios {
			rank := scenarioRankIndex(best[i], scenario.RankMaxes)
			sum += float64(rank)
			if rank > 0 {
				ranked++
			}
			if rank < overall {
				overall = rank
			}
		}
		point := historyPoint{
			Day:              day,
			OverallRankIndex: overall,
			AverageRankIndex: math.Round(sum/float64(len(scenarios))*100) / 100,
			RankedScenarios:  ranked,
		}
		if last != nil && last.OverallRankIndex == point.OverallRankIndex && last.AverageRankIndex == point.AverageRankIndex {
			return
		}
		out = append(out, point)
		last = &out[len(out)-1]
	}
	var currentDay time.Time
	for _, run := range sorted {
		i, ok := index[strings.ToLower(strings.TrimSpace(run.ScenarioName))]
		if !ok {
			continue
		}
		day := run.PlayedAt.UTC().Truncate(24 * time.Hour)
		if !currentDay.IsZero() && !day.Equal(currentDay) {
			flush(currentDay)
		}
		currentDay = day
		if run.Score > best[i] {
			best[i] = run.Score
		}
	}
	if !currentDay.IsZero() {
		flush(currentDay)
	}
	return out
}

// rangeSince converts a range name into a lower time bound (nil for all time).
func rangeSince(value string, now time.Time) (*time.Time, string) {
	var d time.Duration
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "7d":
		d = 7 * 24 * time.Hour
	case "30d":
		d = 30 * 24 * time.Hour
	case "90d":
		d = 90 * 24 * time.Hour
	case "365d", "1y":
		d = 365 * 24 * time.Hour
		value = "365d"
	default:
		return nil, "all"
	}
	since := now.Add(-d)
	return &since, strings.ToLower(strings.TrimSpace(value))
}
