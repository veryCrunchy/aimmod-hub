package coaching

import (
	"fmt"
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
	ScenarioName        string      `json:"scenarioName"`
	ScenarioType        string      `json:"scenarioType"`
	SignalKeys          []string    `json:"signalKeys"`
	ContextTags         []string    `json:"contextTags"`
	FocusArea           string      `json:"focusArea"`
	ChallengePreference string      `json:"challengePreference"`
	TimePreference      string      `json:"timePreference"`
	Question            string      `json:"question"`
	Limit               int         `json:"limit"`
	CoachFacts          []CoachFact `json:"coachFacts,omitempty"`
	// General bypasses scenario-compatibility filtering so entries that don't
	// match the current scenario type/name are still returned. Use this for
	// questions about general aim training, authors, or topics that aren't
	// specific to the scenario the player is currently practising.
	General bool `json:"general"`
}

type CoachFact struct {
	Key          string   `json:"key"`
	Label        string   `json:"label"`
	ValueText    string   `json:"valueText"`
	NumericValue *float64 `json:"numericValue,omitempty"`
	BoolValue    *bool    `json:"boolValue,omitempty"`
	Direction    string   `json:"direction"`
	Confidence   string   `json:"confidence"`
}

type QueryEcho struct {
	ScenarioName        string      `json:"scenarioName"`
	ScenarioType        string      `json:"scenarioType"`
	SignalKeys          []string    `json:"signalKeys"`
	ContextTags         []string    `json:"contextTags"`
	FocusArea           string      `json:"focusArea"`
	ChallengePreference string      `json:"challengePreference"`
	TimePreference      string      `json:"timePreference"`
	Question            string      `json:"question"`
	Limit               int         `json:"limit"`
	CoachFacts          []CoachFact `json:"coachFacts,omitempty"`
	General             bool        `json:"general"`
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

type AnswerPlan struct {
	Intent             string   `json:"intent"`
	ResponseShape      string   `json:"responseShape"`
	MustAnswerDirectly bool     `json:"mustAnswerDirectly"`
	PrimaryFindings    []string `json:"primaryFindings"`
	SuggestedActions   []string `json:"suggestedActions"`
	ClarifyingQuestion string   `json:"clarifyingQuestion"`
}

type Response struct {
	Version         string         `json:"version"`
	UpdatedAtISO    string         `json:"updatedAtIso"`
	CacheTTLSecs    int            `json:"cacheTtlSecs"`
	ToolInstruction string         `json:"toolInstruction"`
	Query           QueryEcho      `json:"query"`
	AnswerPlan      AnswerPlan     `json:"answerPlan"`
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
			CoachFacts:          query.CoachFacts,
			General:             query.General,
		},
		AnswerPlan: buildAnswerPlan(query, items),
		Items:      items,
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
	if query.General {
		// General mode: ignore scenario context entirely so all entries are eligible.
		query.ScenarioName = ""
		query.ScenarioType = ""
	} else if isBroadRecommendationQuestion(questionLower) {
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
	for idx := range query.CoachFacts {
		query.CoachFacts[idx].Key = normalizeToken(query.CoachFacts[idx].Key)
		query.CoachFacts[idx].Label = strings.TrimSpace(query.CoachFacts[idx].Label)
		query.CoachFacts[idx].ValueText = strings.TrimSpace(query.CoachFacts[idx].ValueText)
		query.CoachFacts[idx].Direction = normalizeToken(query.CoachFacts[idx].Direction)
		query.CoachFacts[idx].Confidence = normalizeToken(query.CoachFacts[idx].Confidence)
	}
	return query
}

func buildAnswerPlan(query Query, items []MatchedEntry) AnswerPlan {
	intent := classifyQuestionIntent(query.Question)
	plan := AnswerPlan{
		Intent:             intent,
		ResponseShape:      responseShapeForIntent(intent),
		MustAnswerDirectly: mustAnswerDirectly(intent),
		PrimaryFindings:    buildPrimaryFindings(query, intent, items),
		SuggestedActions:   buildSuggestedActions(query, intent, items),
	}
	if (intent == "setup" || intent == "scenario_recommendation" || intent == "proactive_summary") && len(items) > 0 {
		for _, item := range items[:min(2, len(items))] {
			if strings.TrimSpace(item.Summary) != "" {
				plan.PrimaryFindings = append(plan.PrimaryFindings, item.Summary)
			}
		}
		plan.PrimaryFindings = dedupeStrings(plan.PrimaryFindings)
	}
	if len(items) == 0 && len(plan.PrimaryFindings) == 0 {
		plan.ClarifyingQuestion = clarifyingQuestionForIntent(intent)
	}
	return plan
}

func classifyQuestionIntent(question string) string {
	normalized := strings.ToLower(strings.TrimSpace(question))
	switch {
	case normalized == "":
		return "proactive_summary"
	case containsAny(normalized, "summary", "overview", "how am i doing", "what stands out", "main takeaway"):
		return "proactive_summary"
	case containsAny(normalized, " sens", "sens ", "sensitivity", "cm/360", "cm360", " dpi", "edpi", "polling rate", "mousepad", "mouse pad", "mouse "):
		return "setup"
	case strings.HasPrefix(normalized, "who is ") || strings.HasPrefix(normalized, "who's "):
		return "identity"
	case containsAny(normalized, "context of", "about this scenario", "what is the context", "what does this scenario train", "what is this scenario"):
		return "scenario_context"
	case containsAny(normalized, "what scenarios", "which scenarios", "best scenarios", "good scenarios", "playlist", "routine", "what should i play"):
		return "scenario_recommendation"
	case containsAny(normalized, "plateau", "trend", "variance", "slope", "why am i", "what should i work on", "how do i improve", "break a plateau"):
		return "performance_analysis"
	default:
		return "general_coaching"
	}
}

func responseShapeForIntent(intent string) string {
	switch intent {
	case "setup", "identity", "scenario_context", "scenario_recommendation":
		return "direct_answer"
	case "proactive_summary", "performance_analysis":
		return "diagnosis_why_next"
	default:
		return "direct_answer_with_context"
	}
}

func mustAnswerDirectly(intent string) bool {
	switch intent {
	case "setup", "identity", "scenario_context", "scenario_recommendation":
		return true
	default:
		return false
	}
}

func clarifyingQuestionForIntent(intent string) string {
	switch intent {
	case "setup":
		return "What is your current cm/360 and what feels wrong right now?"
	case "scenario_recommendation":
		return "Which game or aiming weakness do you want the scenarios to target?"
	case "performance_analysis", "proactive_summary":
		return "Do you want the biggest current issue or the best next drill block?"
	default:
		return ""
	}
}

func buildPrimaryFindings(query Query, intent string, items []MatchedEntry) []string {
	findings := []string{}
	switch intent {
	case "scenario_recommendation":
		findings = append(findings, buildScenarioRecommendationFindings(query, items)...)
	case "proactive_summary":
		findings = append(findings, buildProactiveSummaryFindings(query, items)...)
	}
	if plateau, ok := boolFact(query.CoachFacts, "plateau_detected"); ok && plateau {
		findings = append(findings, "A recent plateau is currently detected in this scenario.")
	}
	if warmupDrop, ok := numericFact(query.CoachFacts, "warmup_drop_pct"); ok && warmupDrop >= 5 {
		findings = append(findings, fmt.Sprintf("Opening runs are landing about %.0f%% below settled-in runs, so readiness is part of the current picture.", warmupDrop))
	}
	if scoreCV, ok := numericFact(query.CoachFacts, "score_cv_pct"); ok {
		switch {
		case scoreCV >= 12:
			findings = append(findings, fmt.Sprintf("Recent score variance is elevated at about %.1f%%, which points to unstable execution rather than a clean ceiling.", scoreCV))
		case scoreCV > 0 && scoreCV <= 6:
			findings = append(findings, fmt.Sprintf("Recent score variance is low at about %.1f%%, so performance is relatively stable right now.", scoreCV))
		}
	}
	if slope, ok := numericFact(query.CoachFacts, "score_slope_pts_per_run"); ok {
		switch {
		case slope >= 8:
			findings = append(findings, fmt.Sprintf("The current score slope is positive at about +%.0f pts/run, so progress is still trending upward.", slope))
		case slope <= -5:
			findings = append(findings, fmt.Sprintf("The current score slope is negative at about %.0f pts/run, so form is drifting downward instead of consolidating.", slope))
		case slope > -3 && slope < 3:
			findings = append(findings, fmt.Sprintf("The current score slope is nearly flat at about %.0f pts/run, so score movement is limited right now.", slope))
		}
	}
	if recentAvg, ok := numericFact(query.CoachFacts, "recent_avg_score"); ok {
		if best, ok := numericFact(query.CoachFacts, "all_time_best_score"); ok && best > 0 {
			findings = append(findings, fmt.Sprintf("Recent average score is around %.0f compared with an all-time best of %.0f, which helps frame how much of the gap is consistency versus peak ability.", recentAvg, best))
		}
	}
	return dedupeStrings(findings)
}

func buildSuggestedActions(query Query, intent string, items []MatchedEntry) []string {
	actions := []string{}
	switch intent {
	case "scenario_recommendation":
		actions = append(actions, buildScenarioRecommendationActions(query, items)...)
	case "proactive_summary":
		actions = append(actions, buildProactiveSummaryActions(query, items)...)
	}
	if warmupDrop, ok := numericFact(query.CoachFacts, "warmup_drop_pct"); ok && warmupDrop >= 5 {
		actions = append(actions, "Address warm-up readiness before judging whether the scenario itself needs to change.")
	}
	if plateau, ok := boolFact(query.CoachFacts, "plateau_detected"); ok && plateau {
		actions = append(actions, "Use a short change in drill emphasis or scenario difficulty instead of grinding the same block unchanged.")
	}
	if variance, ok := numericFact(query.CoachFacts, "score_cv_pct"); ok && variance >= 12 {
		actions = append(actions, "Prioritize repeatable execution quality first, because high variance usually means the floor is not settled yet.")
	}
	for _, item := range items {
		for _, action := range item.Actions {
			trimmed := strings.TrimSpace(action)
			if trimmed != "" {
				actions = append(actions, trimmed)
			}
			if len(actions) >= 4 {
				return dedupeStrings(actions)
			}
		}
	}
	return dedupeStrings(actions)
}

func buildScenarioRecommendationFindings(query Query, items []MatchedEntry) []string {
	findings := []string{}
	if game := primaryGameTag(query.ContextTags); game != "" {
		findings = append(findings, fmt.Sprintf("This looks like a %s recommendation question, so transfer to that game matters more than generic benchmark grinding.", game))
	}
	if query.ScenarioType != "" {
		findings = append(findings, fmt.Sprintf("The current request is anchored to the **%s** family, but the hub can still recommend adjacent categories if transfer looks better.", query.ScenarioType))
	}
	for _, item := range items {
		if strings.TrimSpace(item.Summary) == "" {
			continue
		}
		findings = append(findings, item.Summary)
		if len(findings) >= 4 {
			break
		}
	}
	return dedupeStrings(findings)
}

func buildProactiveSummaryFindings(query Query, items []MatchedEntry) []string {
	findings := []string{}
	if query.ScenarioName != "" {
		findings = append(findings, fmt.Sprintf("Current summary is anchored to **%s**.", query.ScenarioName))
	}
	if query.ScenarioType != "" {
		findings = append(findings, proactiveScenarioTypeFinding(query.ScenarioType))
	}
	for _, item := range items {
		if strings.TrimSpace(item.Summary) == "" {
			continue
		}
		findings = append(findings, item.Summary)
		if len(findings) >= 4 {
			break
		}
	}
	return dedupeStrings(findings)
}

func buildScenarioRecommendationActions(query Query, items []MatchedEntry) []string {
	actions := []string{}
	for _, item := range items {
		for _, drill := range item.Drills {
			label := strings.TrimSpace(drill.Label)
			reason := strings.TrimSpace(drill.Reason)
			queryText := strings.TrimSpace(drill.Query)
			switch {
			case label != "" && reason != "":
				actions = append(actions, fmt.Sprintf("Play **%s** first — %s", label, reason))
			case label != "":
				actions = append(actions, fmt.Sprintf("Play **%s** first.", label))
			case queryText != "":
				actions = append(actions, fmt.Sprintf("Try a drill matching **%s**.", queryText))
			}
			if len(actions) >= 3 {
				return dedupeStrings(actions)
			}
		}
	}
	for _, item := range items {
		for _, action := range item.Actions {
			trimmed := strings.TrimSpace(action)
			if trimmed != "" {
				actions = append(actions, trimmed)
			}
			if len(actions) >= 4 {
				return dedupeStrings(actions)
			}
		}
	}
	if len(actions) == 0 && query.ScenarioType != "" {
		actions = append(actions, fmt.Sprintf("Build the next block around one or two %s drills that clearly target the weakness you care about.", query.ScenarioType))
	}
	return dedupeStrings(actions)
}

func buildProactiveSummaryActions(query Query, items []MatchedEntry) []string {
	actions := []string{}
	for _, item := range items {
		for _, action := range item.Actions {
			trimmed := strings.TrimSpace(action)
			if trimmed != "" {
				actions = append(actions, trimmed)
			}
			if len(actions) >= 3 {
				return dedupeStrings(actions)
			}
		}
	}
	if len(actions) == 0 && query.ScenarioType != "" {
		actions = append(actions, fmt.Sprintf("Keep the next block focused on one clear %s priority instead of mixing too many aims at once.", query.ScenarioType))
	}
	return dedupeStrings(actions)
}

func primaryGameTag(tags []string) string {
	for _, tag := range tags {
		switch normalizeToken(tag) {
		case "valorant":
			return "Valorant"
		case "counter_strike":
			return "Counter-Strike"
		case "overwatch":
			return "Overwatch"
		case "apex":
			return "Apex"
		case "arcade_fps":
			return "arcade FPS"
		}
	}
	return ""
}

func proactiveScenarioTypeFinding(scenarioType string) string {
	switch normalizeToken(scenarioType) {
	case "tracking", "puretracking", "controltracking", "precisetracking", "reactivetracking":
		return "Because this is a tracking-focused context, contact quality, correction timing, and pacing matter more than chasing raw headline speed."
	case "targetswitching", "switching", "multihitclicking":
		return "Because this is a switching-focused context, the key issue is usually finish quality and chaining rather than only the first snap."
	case "staticclicking", "oneshotclicking", "clicking":
		return "Because this is a static-clicking context, arrival quality and minimizing extra correction usually matter more than raw tempo."
	case "dynamicclicking", "movingclicking", "reactiveclicking", "clicktiming":
		return "Because this is a dynamic-clicking context, placement, reading, and minimal unnecessary movement matter more than panic speed."
	default:
		return "The current summary should stay anchored to the main mechanic this scenario family is trying to train."
	}
}

func numericFact(facts []CoachFact, key string) (float64, bool) {
	target := normalizeToken(key)
	for _, fact := range facts {
		if fact.Key == target && fact.NumericValue != nil {
			return *fact.NumericValue, true
		}
	}
	return 0, false
}

func boolFact(facts []CoachFact, key string) (bool, bool) {
	target := normalizeToken(key)
	for _, fact := range facts {
		if fact.Key == target && fact.BoolValue != nil {
			return *fact.BoolValue, true
		}
	}
	return false, false
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
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
	if query.General {
		return true
	}
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
	case "sens":
		variants = append(variants, "sensitivity", "setup")
	case "cm360", "cm":
		variants = append(variants, "sensitivity")
	case "dpi", "edpi", "polling":
		variants = append(variants, "setup", "sensitivity")
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
	if containsAny(question, " sens", "sens ", "sensitivity", "cm/360", "cm360", " dpi", "edpi", "polling rate") {
		values = append(values, "mouse_control_adaptation")
	}
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
	if containsAny(question, " sens", "sens ", "sensitivity", "cm/360", "cm360", " dpi", "edpi", "polling rate", "mousepad", "mouse pad", "mouse ") {
		values = append(values, "sensitivity", "setup")
	}
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
