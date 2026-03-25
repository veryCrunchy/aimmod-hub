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
	ID            string      `json:"id"`
	Title         string      `json:"title"`
	Summary       string      `json:"summary"`
	ScenarioTypes []string    `json:"scenarioTypes"`
	ScenarioNames []string    `json:"scenarioNames"`
	SignalKeys    []string    `json:"signalKeys"`
	ContextTags   []string    `json:"contextTags"`
	Why           []string    `json:"why"`
	Actions       []string    `json:"actions"`
	Drills        []Drill     `json:"drills"`
	Avoid         []string    `json:"avoid"`
	Priority      string      `json:"priority"`
	Sources       []SourceRef `json:"sources,omitempty"`
	Match         MatchInfo   `json:"match"`
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
	questionLower := strings.ToLower(strings.TrimSpace(query.Question))
	query.ScenarioName = normalizeToken(query.ScenarioName)
	query.ScenarioType = normalizeToken(query.ScenarioType)
	if isBroadRecommendationQuestion(questionLower) {
		query.ScenarioName = ""
		if derived := deriveScenarioTypeFromQuestion(questionLower); derived != "" {
			query.ScenarioType = derived
		} else {
			query.ScenarioType = ""
		}
	} else if derived := deriveScenarioTypeFromQuestion(questionLower); derived != "" {
		query.ScenarioType = derived
	}
	query.SignalKeys = normalizeList(append(query.SignalKeys, deriveSignalKeysFromQuestion(questionLower)...))
	query.ContextTags = normalizeList(append(query.ContextTags, deriveContextTagsFromQuestion(questionLower)...))
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
		Sources:       entry.Sources,
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
	queryTokens := expandedSearchTokens(question)
	if len(queryTokens) == 0 {
		return nil
	}
	entryTokens := entrySearchTokenSet(entry)
	seen := map[string]struct{}{}
	terms := []string{}
	for _, token := range queryTokens {
		if _, ok := seen[token]; ok {
			continue
		}
		if _, ok := entryTokens[token]; ok {
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

func tokenizeSearchText(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	value = strings.ToLower(value)
	tokens := make([]string, 0, 16)
	var current strings.Builder
	flush := func() {
		if current.Len() == 0 {
			return
		}
		token := current.String()
		current.Reset()
		if len(token) < 3 {
			return
		}
		tokens = append(tokens, token)
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			current.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func expandedSearchTokens(value string) []string {
	raw := tokenizeSearchText(value)
	if len(raw) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(raw)*2)
	for _, token := range raw {
		for _, variant := range expandSearchToken(token) {
			if len(variant) < 3 {
				continue
			}
			if _, ok := seen[variant]; ok {
				continue
			}
			seen[variant] = struct{}{}
			result = append(result, variant)
		}
	}
	sort.Strings(result)
	return result
}

func expandSearchToken(token string) []string {
	variants := []string{token}
	switch token {
	case "valorant", "valerant":
		variants = append(variants, "tactical", "shooter", "tacticalshooter")
	case "cs2":
		variants = append(variants, "counter", "strike", "counterstrike", "tactical", "shooter", "tacticalshooter")
	case "counter":
		variants = append(variants, "counterstrike")
	case "strike":
		variants = append(variants, "counterstrike")
	case "tracking":
		variants = append(variants, "track", "reactive", "control", "precision")
	case "switching":
		variants = append(variants, "switch", "targetswitching")
	case "clicking":
		variants = append(variants, "click", "timing")
	case "flicks", "flick":
		variants = append(variants, "switching", "clicking")
	case "scenario", "scenarios":
		variants = append(variants, "drill", "drills", "playlist")
	}
	if strings.HasSuffix(token, "ies") && len(token) > 4 {
		variants = append(variants, strings.TrimSuffix(token, "ies")+"y")
	}
	if strings.HasSuffix(token, "ing") && len(token) > 5 {
		variants = append(variants, strings.TrimSuffix(token, "ing"))
	}
	if strings.HasSuffix(token, "ed") && len(token) > 4 {
		variants = append(variants, strings.TrimSuffix(token, "ed"))
	}
	if strings.HasSuffix(token, "s") && len(token) > 4 && !strings.HasSuffix(token, "ss") {
		variants = append(variants, strings.TrimSuffix(token, "s"))
	}
	return variants
}

func entrySearchTokenSet(entry Entry) map[string]struct{} {
	parts := []string{
		entry.Title,
		entry.Summary,
		strings.Join(entry.ScenarioTypes, " "),
		strings.Join(entry.ScenarioNames, " "),
		strings.Join(entry.SignalKeys, " "),
		strings.Join(entry.ContextTags, " "),
		strings.Join(entry.FocusAreas, " "),
		strings.Join(entry.ChallengePreferences, " "),
		strings.Join(entry.TimePreferences, " "),
		strings.Join(entry.Why, " "),
		strings.Join(entry.Actions, " "),
		strings.Join(entry.Avoid, " "),
	}
	for _, drill := range entry.Drills {
		parts = append(parts, drill.Label, drill.Query, drill.Reason)
	}
	if entry.Flaw != nil {
		parts = append(
			parts,
			entry.Flaw.Title,
			entry.Flaw.Summary,
			strings.Join(entry.Flaw.SignalKeys, " "),
			strings.Join(entry.Flaw.ContextTags, " "),
			strings.Join(entry.Flaw.Telltales, " "),
			strings.Join(entry.Flaw.Contraindications, " "),
			strings.Join(entry.Flaw.Avoid, " "),
		)
	}
	for _, mechanic := range entry.Mechanics {
		parts = append(
			parts,
			mechanic.Title,
			mechanic.Summary,
			strings.Join(mechanic.Cues, " "),
			strings.Join(mechanic.FailureModes, " "),
			strings.Join(mechanic.RelatedSignalKeys, " "),
		)
	}
	for _, scenario := range entry.Scenarios {
		parts = append(
			parts,
			scenario.Name,
			scenario.Summary,
			strings.Join(scenario.Aliases, " "),
			strings.Join(scenario.ScenarioTypes, " "),
			strings.Join(scenario.WhatItTrains, " "),
			strings.Join(scenario.GoodForFlaws, " "),
			strings.Join(scenario.Cautions, " "),
		)
	}
	for _, evidence := range entry.Evidence {
		parts = append(parts, evidence.Claim, evidence.Excerpt, evidence.Confidence, evidence.ReviewStatus)
	}
	for _, source := range entry.Sources {
		parts = append(parts, source.Title, source.Author, source.Kind)
	}

	set := map[string]struct{}{}
	for _, part := range parts {
		for _, token := range expandedSearchTokens(part) {
			set[token] = struct{}{}
		}
	}
	return set
}

func isBroadRecommendationQuestion(question string) bool {
	asksForScenarios := containsAny(question,
		"what scenarios",
		"which scenarios",
		"good scenarios",
		"best scenarios",
		"playlist",
		"routine",
		"practice",
		"good for",
	)
	mentionsGame := containsAny(question,
		"valorant",
		"valerant",
		"cs2",
		"counter-strike",
		"counter strike",
		"overwatch",
		"apex",
		"battlefield",
		"cod",
		"call of duty",
	)
	return asksForScenarios || mentionsGame
}

func deriveScenarioTypeFromQuestion(question string) string {
	switch {
	case containsAny(question, "target switching", "switching"):
		return "targetswitching"
	case containsAny(question, "static", "one shot", "oneshot"):
		return "staticclicking"
	case containsAny(question, "dynamic", "click timing", "moving clicking"):
		return "dynamicclicking"
	case strings.Contains(question, "reactive tracking"):
		return "reactivetracking"
	case containsAny(question, "precise tracking", "precision tracking"):
		return "precisetracking"
	case strings.Contains(question, "control tracking"):
		return "controltracking"
	case strings.Contains(question, "tracking"):
		return "tracking"
	default:
		return ""
	}
}

func deriveSignalKeysFromQuestion(question string) []string {
	values := make([]string, 0, 4)
	if strings.Contains(question, "tracking") {
		values = append(values, "tracking")
	}
	if strings.Contains(question, "switching") {
		values = append(values, "switching")
	}
	if strings.Contains(question, "click") {
		values = append(values, "clicking")
	}
	if containsAny(question, "motion mapping", "transfer") {
		values = append(values, "transfer")
	}
	return values
}

func deriveContextTagsFromQuestion(question string) []string {
	values := make([]string, 0, 6)
	if containsAny(question, "valorant", "valerant") {
		values = append(values, "valorant", "tactical_shooter")
	}
	if containsAny(question, "cs2", "counter-strike", "counter strike") {
		values = append(values, "counter_strike", "tactical_shooter")
	}
	if strings.Contains(question, "overwatch") {
		values = append(values, "overwatch")
	}
	if strings.Contains(question, "apex") {
		values = append(values, "apex")
	}
	if containsAny(question, "battlefield", "cod", "call of duty") {
		values = append(values, "arcade_fps")
	}
	if containsAny(question, "transfer", "good for") {
		values = append(values, "transfer")
	}
	return values
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
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
