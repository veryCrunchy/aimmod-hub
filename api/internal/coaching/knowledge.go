package coaching

import (
	"sort"
	"strings"
	"sync"
)

const (
	DefaultQueryLimit = 6
	MaxQueryLimit     = 12
	CacheTTLSecs      = 60 * 60 * 24
)

type Drill struct {
	Label  string `json:"label"`
	Query  string `json:"query"`
	Reason string `json:"reason"`
}

type Entry struct {
	ID                   string        `json:"id"`
	Title                string        `json:"title"`
	Summary              string        `json:"summary"`
	ScenarioTypes        []string      `json:"scenarioTypes"`
	ScenarioNames        []string      `json:"scenarioNames"`
	SignalKeys           []string      `json:"signalKeys"`
	ContextTags          []string      `json:"contextTags"`
	FocusAreas           []string      `json:"focusAreas"`
	ChallengePreferences []string      `json:"challengePreferences"`
	TimePreferences      []string      `json:"timePreferences"`
	Why                  []string      `json:"why"`
	Actions              []string      `json:"actions"`
	Drills               []Drill       `json:"drills"`
	Avoid                []string      `json:"avoid"`
	Priority             string        `json:"priority"`
	Flaw                 *FlawRef      `json:"flaw,omitempty"`
	Mechanics            []MechanicRef `json:"mechanics,omitempty"`
	Scenarios            []ScenarioRef `json:"scenarios,omitempty"`
	Evidence             []EvidenceRef `json:"evidence,omitempty"`
	Sources              []SourceRef   `json:"sources,omitempty"`
}

type Base struct {
	Version         string  `json:"version"`
	UpdatedAtISO    string  `json:"updatedAtIso"`
	ToolInstruction string  `json:"toolInstruction"`
	Entries         []Entry `json:"entries"`
}

type Query struct {
	ScenarioName        string   `json:"scenarioName"`
	ScenarioType        string   `json:"scenarioType"`
	SignalKeys          []string `json:"signalKeys"`
	ContextTags         []string `json:"contextTags"`
	FocusArea           string   `json:"focusArea"`
	ChallengePreference string   `json:"challengePreference"`
	TimePreference      string   `json:"timePreference"`
	Question            string   `json:"question"`
	Limit               int      `json:"limit"`
}

type QueryEcho struct {
	ScenarioName        string   `json:"scenarioName"`
	ScenarioType        string   `json:"scenarioType"`
	SignalKeys          []string `json:"signalKeys"`
	ContextTags         []string `json:"contextTags"`
	FocusArea           string   `json:"focusArea"`
	ChallengePreference string   `json:"challengePreference"`
	TimePreference      string   `json:"timePreference"`
	Question            string   `json:"question"`
	Limit               int      `json:"limit"`
}

type MatchInfo struct {
	ScenarioName  bool     `json:"scenarioName"`
	ScenarioType  bool     `json:"scenarioType"`
	SignalKeys    []string `json:"signalKeys"`
	ContextTags   []string `json:"contextTags"`
	Preferences   []string `json:"preferences"`
	QuestionTerms []string `json:"questionTerms"`
}

type MatchedEntry struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Summary       string    `json:"summary"`
	ScenarioTypes []string  `json:"scenarioTypes"`
	ScenarioNames []string  `json:"scenarioNames"`
	SignalKeys    []string  `json:"signalKeys"`
	ContextTags   []string  `json:"contextTags"`
	Why           []string  `json:"why"`
	Actions       []string  `json:"actions"`
	Drills        []Drill   `json:"drills"`
	Avoid         []string  `json:"avoid"`
	Priority      string    `json:"priority"`
	Match         MatchInfo `json:"match"`
}

type Response struct {
	Version         string         `json:"version"`
	UpdatedAtISO    string         `json:"updatedAtIso"`
	CacheTTLSecs    int            `json:"cacheTtlSecs"`
	ToolInstruction string         `json:"toolInstruction"`
	Query           QueryEcho      `json:"query"`
	Items           []MatchedEntry `json:"items"`
}

type Manifest struct {
	Version       string   `json:"version"`
	UpdatedAtISO  string   `json:"updatedAtIso"`
	CacheTTLSecs  int      `json:"cacheTtlSecs"`
	EntryCount    int      `json:"entryCount"`
	SignalKeys    []string `json:"signalKeys"`
	ContextTags   []string `json:"contextTags"`
	ScenarioTypes []string `json:"scenarioTypes"`
}

var (
	loadOnce sync.Once
	loadBase Base
	loadErr  error
)

func Load() (*Base, error) {
	loadOnce.Do(func() {
		base, err := compileKnowledgeFromContent()
		if err != nil {
			loadErr = err
			return
		}
		loadBase = *base
		for i := range loadBase.Entries {
			normalizeEntry(&loadBase.Entries[i])
		}
	})
	if loadErr != nil {
		return nil, loadErr
	}
	return &loadBase, nil
}

func GetManifest() (Manifest, error) {
	base, err := Load()
	if err != nil {
		return Manifest{}, err
	}
	signals := map[string]struct{}{}
	tags := map[string]struct{}{}
	scenarioTypes := map[string]struct{}{}
	for _, entry := range base.Entries {
		for _, value := range entry.SignalKeys {
			signals[value] = struct{}{}
		}
		for _, value := range entry.ContextTags {
			tags[value] = struct{}{}
		}
		for _, value := range entry.ScenarioTypes {
			scenarioTypes[value] = struct{}{}
		}
	}
	return Manifest{
		Version:       base.Version,
		UpdatedAtISO:  base.UpdatedAtISO,
		CacheTTLSecs:  CacheTTLSecs,
		EntryCount:    len(base.Entries),
		SignalKeys:    sortedKeys(signals),
		ContextTags:   sortedKeys(tags),
		ScenarioTypes: sortedKeys(scenarioTypes),
	}, nil
}

func QueryKnowledge(input Query) (Response, error) {
	base, err := Load()
	if err != nil {
		return Response{}, err
	}

	query := normalizeQuery(input)
	limit := query.Limit
	if limit <= 0 {
		limit = DefaultQueryLimit
	}
	if limit > MaxQueryLimit {
		limit = MaxQueryLimit
	}

	type scoredEntry struct {
		entry MatchedEntry
		score int
	}

	matches := make([]scoredEntry, 0, len(base.Entries))
	for _, entry := range base.Entries {
		match, score := scoreEntry(entry, query)
		if score <= 0 {
			continue
		}
		matches = append(matches, scoredEntry{entry: match, score: score})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		if priorityRank(matches[i].entry.Priority) != priorityRank(matches[j].entry.Priority) {
			return priorityRank(matches[i].entry.Priority) > priorityRank(matches[j].entry.Priority)
		}
		return matches[i].entry.Title < matches[j].entry.Title
	})

	items := make([]MatchedEntry, 0, min(limit, len(matches)))
	for _, match := range matches {
		items = append(items, match.entry)
		if len(items) >= limit {
			break
		}
	}

	return Response{
		Version:         base.Version,
		UpdatedAtISO:    base.UpdatedAtISO,
		CacheTTLSecs:    CacheTTLSecs,
		ToolInstruction: base.ToolInstruction,
		Query: QueryEcho{
			ScenarioName:        query.ScenarioName,
			ScenarioType:        query.ScenarioType,
			SignalKeys:          query.SignalKeys,
			ContextTags:         query.ContextTags,
			FocusArea:           query.FocusArea,
			ChallengePreference: query.ChallengePreference,
			TimePreference:      query.TimePreference,
			Question:            query.Question,
			Limit:               limit,
		},
		Items: items,
	}, nil
}

func normalizeEntry(entry *Entry) {
	entry.ScenarioTypes = normalizeList(entry.ScenarioTypes)
	entry.ScenarioNames = normalizeList(entry.ScenarioNames)
	entry.SignalKeys = normalizeList(entry.SignalKeys)
	entry.ContextTags = normalizeList(entry.ContextTags)
	entry.FocusAreas = normalizeList(entry.FocusAreas)
	entry.ChallengePreferences = normalizeList(entry.ChallengePreferences)
	entry.TimePreferences = normalizeList(entry.TimePreferences)
	entry.Priority = normalizeToken(entry.Priority)
	if entry.Flaw != nil {
		entry.Flaw.ID = normalizeToken(entry.Flaw.ID)
		entry.Flaw.SignalKeys = normalizeList(entry.Flaw.SignalKeys)
		entry.Flaw.ContextTags = normalizeList(entry.Flaw.ContextTags)
		entry.Flaw.Telltales = normalizeStringList(entry.Flaw.Telltales)
		entry.Flaw.Contraindications = normalizeStringList(entry.Flaw.Contraindications)
		entry.Flaw.Avoid = normalizeStringList(entry.Flaw.Avoid)
	}
	for idx := range entry.Mechanics {
		entry.Mechanics[idx].ID = normalizeToken(entry.Mechanics[idx].ID)
		entry.Mechanics[idx].RelatedSignalKeys = normalizeList(entry.Mechanics[idx].RelatedSignalKeys)
		entry.Mechanics[idx].Cues = normalizeStringList(entry.Mechanics[idx].Cues)
		entry.Mechanics[idx].FailureModes = normalizeStringList(entry.Mechanics[idx].FailureModes)
	}
	for idx := range entry.Scenarios {
		entry.Scenarios[idx].ID = normalizeToken(entry.Scenarios[idx].ID)
		entry.Scenarios[idx].ScenarioTypes = normalizeList(entry.Scenarios[idx].ScenarioTypes)
		entry.Scenarios[idx].GoodForFlaws = normalizeList(entry.Scenarios[idx].GoodForFlaws)
		entry.Scenarios[idx].Aliases = normalizeStringList(entry.Scenarios[idx].Aliases)
		entry.Scenarios[idx].WhatItTrains = normalizeStringList(entry.Scenarios[idx].WhatItTrains)
		entry.Scenarios[idx].Cautions = normalizeStringList(entry.Scenarios[idx].Cautions)
	}
}

func normalizeQuery(query Query) Query {
	query.ScenarioName = normalizeToken(query.ScenarioName)
	query.ScenarioType = normalizeToken(query.ScenarioType)
	query.SignalKeys = normalizeList(query.SignalKeys)
	query.ContextTags = normalizeList(query.ContextTags)
	query.FocusArea = normalizeToken(query.FocusArea)
	query.ChallengePreference = normalizeToken(query.ChallengePreference)
	query.TimePreference = normalizeToken(query.TimePreference)
	query.Question = strings.TrimSpace(query.Question)
	return query
}

func scoreEntry(entry Entry, query Query) (MatchedEntry, int) {
	if !isCompatible(entry, query) {
		return MatchedEntry{}, 0
	}

	score := 0
	anchored := false
	match := MatchInfo{}

	if query.ScenarioName != "" && contains(entry.ScenarioNames, query.ScenarioName) {
		score += 8
		anchored = true
		match.ScenarioName = true
	}
	if query.ScenarioType != "" && contains(entry.ScenarioTypes, query.ScenarioType) {
		score += 5
		anchored = true
		match.ScenarioType = true
	}

	matchedSignals := intersection(entry.SignalKeys, query.SignalKeys)
	if len(matchedSignals) > 0 {
		score += min(len(matchedSignals)*3, 12)
		anchored = true
		match.SignalKeys = matchedSignals
	}

	matchedTags := intersection(entry.ContextTags, query.ContextTags)
	if len(matchedTags) > 0 {
		score += min(len(matchedTags)*2, 8)
		anchored = true
		match.ContextTags = matchedTags
	}

	if query.FocusArea != "" && contains(entry.FocusAreas, query.FocusArea) {
		score++
		match.Preferences = append(match.Preferences, "focus_area")
	}
	if query.ChallengePreference != "" && contains(entry.ChallengePreferences, query.ChallengePreference) {
		score++
		match.Preferences = append(match.Preferences, "challenge_preference")
	}
	if query.TimePreference != "" && contains(entry.TimePreferences, query.TimePreference) {
		score++
		match.Preferences = append(match.Preferences, "time_preference")
	}

	if terms := matchedQuestionTerms(entry, query.Question); len(terms) > 0 {
		score += min(len(terms), 2)
		anchored = true
		match.QuestionTerms = terms
	}

	if score == 0 || !anchored {
		return MatchedEntry{}, 0
	}

	return MatchedEntry{
		ID:            entry.ID,
		Title:         entry.Title,
		Summary:       entry.Summary,
		ScenarioTypes: entry.ScenarioTypes,
		ScenarioNames: entry.ScenarioNames,
		SignalKeys:    entry.SignalKeys,
		ContextTags:   entry.ContextTags,
		Why:           entry.Why,
		Actions:       entry.Actions,
		Drills:        entry.Drills,
		Avoid:         entry.Avoid,
		Priority:      entry.Priority,
		Match:         match,
	}, score
}

func isCompatible(entry Entry, query Query) bool {
	if query.ScenarioName != "" && len(entry.ScenarioNames) > 0 && !contains(entry.ScenarioNames, query.ScenarioName) {
		return false
	}
	if query.ScenarioType != "" && len(entry.ScenarioTypes) > 0 && !contains(entry.ScenarioTypes, query.ScenarioType) {
		return false
	}
	return true
}

func matchedQuestionTerms(entry Entry, question string) []string {
	if strings.TrimSpace(question) == "" {
		return nil
	}
	haystack := strings.ToLower(strings.Join(append(append([]string{
		entry.Title,
		entry.Summary,
	}, entry.Why...), entry.Actions...), " "))
	seen := map[string]struct{}{}
	terms := []string{}
	for _, token := range strings.Fields(strings.ToLower(question)) {
		token = strings.Trim(token, " ,.!?;:()[]{}\"'")
		if len(token) < 4 {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		if strings.Contains(haystack, token) {
			seen[token] = struct{}{}
			terms = append(terms, token)
		}
	}
	sort.Strings(terms)
	return terms
}

func normalizeToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		token := normalizeToken(value)
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		result = append(result, token)
	}
	sort.Strings(result)
	return result
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func intersection(left, right []string) []string {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}
	rightSet := map[string]struct{}{}
	for _, value := range right {
		rightSet[value] = struct{}{}
	}
	result := make([]string, 0, min(len(left), len(right)))
	for _, value := range left {
		if _, ok := rightSet[value]; ok {
			result = append(result, value)
		}
	}
	return result
}

func priorityRank(value string) int {
	switch normalizeToken(value) {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func sortedKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
