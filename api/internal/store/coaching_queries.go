package store

import (
	"context"
	"sort"
	"strings"
	"time"
	"unicode"
)

// CoachingQueryRecord is a single logged coaching knowledge query.
type CoachingQueryRecord struct {
	ScenarioName        string
	ScenarioType        string
	Question            string
	SignalKeys          []string
	ContextTags         []string
	FocusArea           string
	ItemsReturned       int
	MatchedEntryIDs     []string
	QuestionTermsMatched []string
	IsMiss              bool
}

// CoachingQueryRow is a row returned in the recent-query log.
type CoachingQueryRow struct {
	ID                   int64     `json:"id"`
	ScenarioName         string    `json:"scenarioName"`
	ScenarioType         string    `json:"scenarioType"`
	Question             string    `json:"question"`
	SignalKeys           []string  `json:"signalKeys"`
	ContextTags          []string  `json:"contextTags"`
	FocusArea            string    `json:"focusArea"`
	ItemsReturned        int       `json:"itemsReturned"`
	MatchedEntryIDs      []string  `json:"matchedEntryIds"`
	QuestionTermsMatched []string  `json:"questionTermsMatched"`
	IsMiss               bool      `json:"isMiss"`
	CreatedAt            time.Time `json:"createdAt"`
}

// CoachingTermFreq is a question term and how often it appeared in miss queries.
type CoachingTermFreq struct {
	Term  string `json:"term"`
	Count int    `json:"count"`
}

// CoachingScenarioFreq is a scenario name and query count.
type CoachingScenarioFreq struct {
	ScenarioName string `json:"scenarioName"`
	Count        int    `json:"count"`
}

// CoachingEntryFreq is a matched entry ID and how often it was returned.
type CoachingEntryFreq struct {
	EntryID string `json:"entryId"`
	Count   int    `json:"count"`
}

// CoachingAnalytics aggregates coaching query data for the admin dashboard.
type CoachingAnalytics struct {
	Days              int                    `json:"days"`
	TotalQueries      int                    `json:"totalQueries"`
	HitCount          int                    `json:"hitCount"`
	MissCount         int                    `json:"missCount"`
	HitRatePct        float64                `json:"hitRatePct"`
	AvgItemsPerHit    float64                `json:"avgItemsPerHit"`
	TopMissTerms      []CoachingTermFreq     `json:"topMissTerms"`
	TopScenarios      []CoachingScenarioFreq `json:"topScenarios"`
	TopMatchedEntries []CoachingEntryFreq    `json:"topMatchedEntries"`
	RecentQueries     []CoachingQueryRow     `json:"recentQueries"`
}

// RecordCoachingQuery logs a coaching knowledge query asynchronously (fire-and-forget).
// Errors are silently discarded so a logging failure never blocks a query response.
func (s *Store) RecordCoachingQuery(ctx context.Context, rec CoachingQueryRecord) {
	go func() {
		_, _ = s.pool.Exec(context.Background(),
			`INSERT INTO coaching_queries
				(scenario_name, scenario_type, question, signal_keys, context_tags,
				 focus_area, items_returned, matched_entry_ids, question_terms_matched, is_miss)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			rec.ScenarioName,
			rec.ScenarioType,
			rec.Question,
			stringSliceOrEmpty(rec.SignalKeys),
			stringSliceOrEmpty(rec.ContextTags),
			rec.FocusArea,
			rec.ItemsReturned,
			stringSliceOrEmpty(rec.MatchedEntryIDs),
			stringSliceOrEmpty(rec.QuestionTermsMatched),
			rec.IsMiss,
		)
	}()
}

// GetCoachingAnalytics returns aggregated coaching query analytics for the last N days.
func (s *Store) GetCoachingAnalytics(ctx context.Context, days int) (CoachingAnalytics, error) {
	if days <= 0 {
		days = 30
	}
	since := time.Now().UTC().AddDate(0, 0, -days)

	analytics := CoachingAnalytics{Days: days}

	// ── Aggregate counts ──────────────────────────────────────────────────────
	row := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*)                                                        AS total,
			COUNT(*) FILTER (WHERE NOT is_miss)                             AS hits,
			COUNT(*) FILTER (WHERE is_miss)                                 AS misses,
			COALESCE(AVG(items_returned) FILTER (WHERE NOT is_miss), 0)    AS avg_items
		FROM coaching_queries
		WHERE created_at >= $1
	`, since)
	if err := row.Scan(&analytics.TotalQueries, &analytics.HitCount, &analytics.MissCount, &analytics.AvgItemsPerHit); err != nil {
		return analytics, err
	}
	if analytics.TotalQueries > 0 {
		analytics.HitRatePct = float64(analytics.HitCount) / float64(analytics.TotalQueries) * 100
	}

	// ── Top scenarios ─────────────────────────────────────────────────────────
	scenarioRows, err := s.pool.Query(ctx, `
		SELECT scenario_name, COUNT(*) AS cnt
		FROM coaching_queries
		WHERE created_at >= $1 AND scenario_name != ''
		GROUP BY scenario_name
		ORDER BY cnt DESC
		LIMIT 15
	`, since)
	if err != nil {
		return analytics, err
	}
	defer scenarioRows.Close()
	for scenarioRows.Next() {
		var sf CoachingScenarioFreq
		if err := scenarioRows.Scan(&sf.ScenarioName, &sf.Count); err != nil {
			return analytics, err
		}
		analytics.TopScenarios = append(analytics.TopScenarios, sf)
	}
	_ = scenarioRows.Err()

	// ── Top matched entries ───────────────────────────────────────────────────
	entryRows, err := s.pool.Query(ctx, `
		SELECT entry_id, COUNT(*) AS cnt
		FROM coaching_queries, UNNEST(matched_entry_ids) AS entry_id
		WHERE created_at >= $1
		GROUP BY entry_id
		ORDER BY cnt DESC
		LIMIT 15
	`, since)
	if err != nil {
		return analytics, err
	}
	defer entryRows.Close()
	for entryRows.Next() {
		var ef CoachingEntryFreq
		if err := entryRows.Scan(&ef.EntryID, &ef.Count); err != nil {
			return analytics, err
		}
		analytics.TopMatchedEntries = append(analytics.TopMatchedEntries, ef)
	}
	_ = entryRows.Err()

	// ── Unmatched question terms (from miss queries) ───────────────────────────
	// Fetch miss questions and tokenise in Go for flexibility.
	missRows, err := s.pool.Query(ctx, `
		SELECT question
		FROM coaching_queries
		WHERE created_at >= $1 AND is_miss AND question != ''
		LIMIT 2000
	`, since)
	if err != nil {
		return analytics, err
	}
	defer missRows.Close()
	termCounts := map[string]int{}
	for missRows.Next() {
		var q string
		if err := missRows.Scan(&q); err != nil {
			return analytics, err
		}
		for _, tok := range tokeniseQuestion(q) {
			termCounts[tok]++
		}
	}
	_ = missRows.Err()
	for term, count := range termCounts {
		analytics.TopMissTerms = append(analytics.TopMissTerms, CoachingTermFreq{Term: term, Count: count})
	}
	sort.Slice(analytics.TopMissTerms, func(i, j int) bool {
		if analytics.TopMissTerms[i].Count != analytics.TopMissTerms[j].Count {
			return analytics.TopMissTerms[i].Count > analytics.TopMissTerms[j].Count
		}
		return analytics.TopMissTerms[i].Term < analytics.TopMissTerms[j].Term
	})
	if len(analytics.TopMissTerms) > 30 {
		analytics.TopMissTerms = analytics.TopMissTerms[:30]
	}

	// ── Recent queries ────────────────────────────────────────────────────────
	recentRows, err := s.pool.Query(ctx, `
		SELECT id, scenario_name, scenario_type, question, signal_keys, context_tags,
		       focus_area, items_returned, matched_entry_ids, question_terms_matched,
		       is_miss, created_at
		FROM coaching_queries
		WHERE created_at >= $1
		ORDER BY created_at DESC
		LIMIT 100
	`, since)
	if err != nil {
		return analytics, err
	}
	defer recentRows.Close()
	for recentRows.Next() {
		var row CoachingQueryRow
		if err := recentRows.Scan(
			&row.ID, &row.ScenarioName, &row.ScenarioType, &row.Question,
			&row.SignalKeys, &row.ContextTags, &row.FocusArea,
			&row.ItemsReturned, &row.MatchedEntryIDs, &row.QuestionTermsMatched,
			&row.IsMiss, &row.CreatedAt,
		); err != nil {
			return analytics, err
		}
		if row.SignalKeys == nil {
			row.SignalKeys = []string{}
		}
		if row.ContextTags == nil {
			row.ContextTags = []string{}
		}
		if row.MatchedEntryIDs == nil {
			row.MatchedEntryIDs = []string{}
		}
		if row.QuestionTermsMatched == nil {
			row.QuestionTermsMatched = []string{}
		}
		analytics.RecentQueries = append(analytics.RecentQueries, row)
	}
	if err := recentRows.Err(); err != nil {
		return analytics, err
	}

	// Ensure nil slices become empty arrays in JSON.
	if analytics.TopMissTerms == nil {
		analytics.TopMissTerms = []CoachingTermFreq{}
	}
	if analytics.TopScenarios == nil {
		analytics.TopScenarios = []CoachingScenarioFreq{}
	}
	if analytics.TopMatchedEntries == nil {
		analytics.TopMatchedEntries = []CoachingEntryFreq{}
	}
	if analytics.RecentQueries == nil {
		analytics.RecentQueries = []CoachingQueryRow{}
	}

	return analytics, nil
}

// tokeniseQuestion returns meaningful lowercase tokens from a question string.
// Mirrors the logic in coaching/knowledge.go matchedQuestionTerms for consistency.
func tokeniseQuestion(question string) []string {
	seen := map[string]struct{}{}
	var tokens []string
	for _, tok := range strings.Fields(strings.ToLower(question)) {
		tok = strings.TrimFunc(tok, func(r rune) bool {
			return unicode.IsPunct(r) || unicode.IsSymbol(r)
		})
		if len([]rune(tok)) < 4 {
			continue
		}
		if isStopWord(tok) {
			continue
		}
		if _, ok := seen[tok]; ok {
			continue
		}
		seen[tok] = struct{}{}
		tokens = append(tokens, tok)
	}
	return tokens
}

var stopWords = map[string]struct{}{
	"this": {}, "that": {}, "with": {}, "from": {}, "have": {}, "will":  {},
	"what": {}, "when": {}, "where": {}, "which": {}, "your": {}, "their": {},
	"there": {}, "about": {}, "would": {}, "could": {}, "should": {}, "been": {},
	"more": {}, "some": {}, "they": {}, "then": {}, "than": {}, "into": {},
	"also": {}, "just": {}, "does": {}, "next": {},
}

func isStopWord(tok string) bool {
	_, ok := stopWords[tok]
	return ok
}

func stringSliceOrEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
