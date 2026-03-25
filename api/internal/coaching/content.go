package coaching

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed content/*.json content/*/*.json
var contentFS embed.FS

type ConfigDoc struct {
	Version         string `json:"version"`
	UpdatedAtISO    string `json:"updatedAtIso"`
	ToolInstruction string `json:"toolInstruction"`
}

type SourceDoc struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Title          string `json:"title"`
	Author         string `json:"author"`
	URL            string `json:"url"`
	PublishedAtISO string `json:"publishedAtIso"`
	Notes          string `json:"notes"`
}

type MechanicDoc struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Summary           string   `json:"summary"`
	Cues              []string `json:"cues"`
	FailureModes      []string `json:"failureModes"`
	RelatedSignalKeys []string `json:"relatedSignalKeys"`
}

type FlawDoc struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Summary           string   `json:"summary"`
	SignalKeys        []string `json:"signalKeys"`
	ContextTags       []string `json:"contextTags"`
	MechanicIDs       []string `json:"mechanicIds"`
	Telltales         []string `json:"telltales"`
	Contraindications []string `json:"contraindications"`
	Avoid             []string `json:"avoid"`
}

type ScenarioDoc struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Aliases       []string `json:"aliases"`
	ScenarioTypes []string `json:"scenarioTypes"`
	Summary       string   `json:"summary"`
	WhatItTrains  []string `json:"whatItTrains"`
	GoodForFlaws  []string `json:"goodForFlaws"`
	Cautions      []string `json:"cautions"`
}

type EvidenceDoc struct {
	SourceID     string  `json:"sourceId"`
	Claim        string  `json:"claim"`
	Excerpt      string  `json:"excerpt"`
	StartSec     float64 `json:"startSec"`
	EndSec       float64 `json:"endSec"`
	Confidence   string  `json:"confidence"`
	ReviewStatus string  `json:"reviewStatus"`
}

type RecommendationDoc struct {
	ID                   string        `json:"id"`
	FlawID               string        `json:"flawId"`
	Title                string        `json:"title"`
	Summary              string        `json:"summary"`
	ScenarioTypes        []string      `json:"scenarioTypes"`
	ScenarioNames        []string      `json:"scenarioNames"`
	FocusAreas           []string      `json:"focusAreas"`
	ChallengePreferences []string      `json:"challengePreferences"`
	TimePreferences      []string      `json:"timePreferences"`
	MechanicIDs          []string      `json:"mechanicIds"`
	ScenarioIDs          []string      `json:"scenarioIds"`
	Why                  []string      `json:"why"`
	Actions              []string      `json:"actions"`
	Avoid                []string      `json:"avoid"`
	Drills               []Drill       `json:"drills"`
	Priority             string        `json:"priority"`
	Evidence             []EvidenceDoc `json:"evidence"`
}

type FlawRef struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Summary           string   `json:"summary"`
	SignalKeys        []string `json:"signalKeys"`
	ContextTags       []string `json:"contextTags"`
	Telltales         []string `json:"telltales"`
	Contraindications []string `json:"contraindications"`
	Avoid             []string `json:"avoid"`
}

type MechanicRef struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Summary           string   `json:"summary"`
	Cues              []string `json:"cues"`
	FailureModes      []string `json:"failureModes"`
	RelatedSignalKeys []string `json:"relatedSignalKeys"`
}

type ScenarioRef struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Aliases       []string `json:"aliases"`
	ScenarioTypes []string `json:"scenarioTypes"`
	Summary       string   `json:"summary"`
	WhatItTrains  []string `json:"whatItTrains"`
	GoodForFlaws  []string `json:"goodForFlaws"`
	Cautions      []string `json:"cautions"`
}

type SourceRef struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Title          string `json:"title"`
	Author         string `json:"author"`
	URL            string `json:"url"`
	PublishedAtISO string `json:"publishedAtIso"`
}

type EvidenceRef struct {
	SourceID     string  `json:"sourceId"`
	Claim        string  `json:"claim"`
	Excerpt      string  `json:"excerpt"`
	StartSec     float64 `json:"startSec"`
	EndSec       float64 `json:"endSec"`
	Confidence   string  `json:"confidence"`
	ReviewStatus string  `json:"reviewStatus"`
}

type Catalog struct {
	Config          ConfigDoc
	Sources         map[string]SourceDoc
	Mechanics       map[string]MechanicDoc
	Flaws           map[string]FlawDoc
	Scenarios       map[string]ScenarioDoc
	Recommendations map[string]RecommendationDoc
}

func compileKnowledgeFromContent() (*Base, error) {
	catalog, err := loadCatalog()
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(catalog.Recommendations))
	recommendationIDs := sortedKeysFromMap(catalog.Recommendations)
	for _, id := range recommendationIDs {
		entry, err := compileRecommendation(catalog, catalog.Recommendations[id])
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	base := &Base{
		Version:         strings.TrimSpace(catalog.Config.Version),
		UpdatedAtISO:    strings.TrimSpace(catalog.Config.UpdatedAtISO),
		ToolInstruction: strings.TrimSpace(catalog.Config.ToolInstruction),
		Entries:         entries,
	}
	if base.Version == "" {
		return nil, fmt.Errorf("coaching config version is required")
	}
	if base.UpdatedAtISO == "" {
		return nil, fmt.Errorf("coaching config updatedAtIso is required")
	}
	return base, nil
}

func loadCatalog() (Catalog, error) {
	configs, err := readJSONDir[ConfigDoc]("content", func(name string) bool {
		return filepath.Base(name) == "config.json"
	})
	if err != nil {
		return Catalog{}, err
	}
	if len(configs) != 1 {
		return Catalog{}, fmt.Errorf("expected exactly one coaching config.json, got %d", len(configs))
	}

	sources, err := readJSONDir[SourceDoc]("content/sources", nil)
	if err != nil {
		return Catalog{}, err
	}
	mechanics, err := readJSONDir[MechanicDoc]("content/mechanics", nil)
	if err != nil {
		return Catalog{}, err
	}
	flaws, err := readJSONDir[FlawDoc]("content/flaws", nil)
	if err != nil {
		return Catalog{}, err
	}
	scenarios, err := readJSONDir[ScenarioDoc]("content/scenarios", nil)
	if err != nil {
		return Catalog{}, err
	}
	recommendations, err := readJSONDir[RecommendationDoc]("content/recommendations", nil)
	if err != nil {
		return Catalog{}, err
	}

	catalog := Catalog{
		Config:          configs[0],
		Sources:         indexByID(sources, func(value SourceDoc) string { return value.ID }),
		Mechanics:       indexByID(mechanics, func(value MechanicDoc) string { return value.ID }),
		Flaws:           indexByID(flaws, func(value FlawDoc) string { return value.ID }),
		Scenarios:       indexByID(scenarios, func(value ScenarioDoc) string { return value.ID }),
		Recommendations: indexByID(recommendations, func(value RecommendationDoc) string { return value.ID }),
	}

	if err := validateCatalog(catalog); err != nil {
		return Catalog{}, err
	}
	return catalog, nil
}

func validateCatalog(catalog Catalog) error {
	for _, flaw := range catalog.Flaws {
		if strings.TrimSpace(flaw.ID) == "" || strings.TrimSpace(flaw.Title) == "" {
			return fmt.Errorf("flaw is missing id or title")
		}
		for _, mechanicID := range flaw.MechanicIDs {
			if _, ok := catalog.Mechanics[mechanicID]; !ok {
				return fmt.Errorf("flaw %s references unknown mechanic %s", flaw.ID, mechanicID)
			}
		}
	}

	for _, scenario := range catalog.Scenarios {
		if strings.TrimSpace(scenario.ID) == "" || strings.TrimSpace(scenario.Name) == "" {
			return fmt.Errorf("scenario is missing id or name")
		}
		for _, flawID := range scenario.GoodForFlaws {
			if _, ok := catalog.Flaws[normalizeToken(flawID)]; !ok {
				return fmt.Errorf("scenario %s references unknown flaw %s", scenario.ID, flawID)
			}
		}
	}

	for _, recommendation := range catalog.Recommendations {
		if strings.TrimSpace(recommendation.ID) == "" || strings.TrimSpace(recommendation.Title) == "" {
			return fmt.Errorf("recommendation is missing id or title")
		}
		if _, ok := catalog.Flaws[normalizeToken(recommendation.FlawID)]; !ok {
			return fmt.Errorf("recommendation %s references unknown flaw %s", recommendation.ID, recommendation.FlawID)
		}
		for _, mechanicID := range recommendation.MechanicIDs {
			if _, ok := catalog.Mechanics[mechanicID]; !ok {
				return fmt.Errorf("recommendation %s references unknown mechanic %s", recommendation.ID, mechanicID)
			}
		}
		for _, scenarioID := range recommendation.ScenarioIDs {
			if _, ok := catalog.Scenarios[scenarioID]; !ok {
				return fmt.Errorf("recommendation %s references unknown scenario %s", recommendation.ID, scenarioID)
			}
		}
		for _, evidence := range recommendation.Evidence {
			if _, ok := catalog.Sources[evidence.SourceID]; !ok {
				return fmt.Errorf("recommendation %s references unknown source %s", recommendation.ID, evidence.SourceID)
			}
		}
	}
	return nil
}

func compileRecommendation(catalog Catalog, recommendation RecommendationDoc) (Entry, error) {
	flaw := catalog.Flaws[normalizeToken(recommendation.FlawID)]
	mechanics := make([]MechanicRef, 0, len(recommendation.MechanicIDs))
	for _, mechanicID := range recommendation.MechanicIDs {
		mechanics = append(mechanics, newMechanicRef(catalog.Mechanics[mechanicID]))
	}
	scenarios := make([]ScenarioRef, 0, len(recommendation.ScenarioIDs))
	for _, scenarioID := range recommendation.ScenarioIDs {
		scenarios = append(scenarios, newScenarioRef(catalog.Scenarios[scenarioID]))
	}
	evidence := make([]EvidenceRef, 0, len(recommendation.Evidence))
	sourceSet := map[string]struct{}{}
	for _, item := range recommendation.Evidence {
		evidence = append(evidence, EvidenceRef{
			SourceID:     item.SourceID,
			Claim:        item.Claim,
			Excerpt:      item.Excerpt,
			StartSec:     item.StartSec,
			EndSec:       item.EndSec,
			Confidence:   item.Confidence,
			ReviewStatus: item.ReviewStatus,
		})
		sourceSet[item.SourceID] = struct{}{}
	}
	sources := make([]SourceRef, 0, len(sourceSet))
	for _, sourceID := range sortedKeys(sourceSet) {
		sources = append(sources, newSourceRef(catalog.Sources[sourceID]))
	}

	signalKeys := normalizeList(append(append([]string{}, recommendation.ScenarioTypes...), append(flaw.SignalKeys, recommendation.ScenarioNames...)...))
	_ = signalKeys

	entry := Entry{
		ID:                   normalizeToken(recommendation.ID),
		Title:                recommendation.Title,
		Summary:              recommendation.Summary,
		ScenarioTypes:        normalizeList(recommendation.ScenarioTypes),
		ScenarioNames:        normalizeList(recommendation.ScenarioNames),
		SignalKeys:           normalizeList(append(append([]string{}, flaw.SignalKeys...), collectMechanicSignals(mechanics)...)),
		ContextTags:          normalizeList(append([]string{}, flaw.ContextTags...)),
		FocusAreas:           normalizeList(recommendation.FocusAreas),
		ChallengePreferences: normalizeList(recommendation.ChallengePreferences),
		TimePreferences:      normalizeList(recommendation.TimePreferences),
		Why:                  append([]string{}, recommendation.Why...),
		Actions:              append([]string{}, recommendation.Actions...),
		Drills:               append([]Drill{}, recommendation.Drills...),
		Avoid:                normalizeStringList(append(append([]string{}, flaw.Avoid...), recommendation.Avoid...)),
		Priority:             normalizeToken(recommendation.Priority),
		Flaw:                 newFlawRef(flaw),
		Mechanics:            mechanics,
		Scenarios:            scenarios,
		Evidence:             evidence,
		Sources:              sources,
	}
	if len(entry.SignalKeys) == 0 {
		entry.SignalKeys = normalizeList(flaw.SignalKeys)
	}
	if len(entry.ContextTags) == 0 {
		entry.ContextTags = normalizeList(flaw.ContextTags)
	}
	return entry, nil
}

func newFlawRef(value FlawDoc) *FlawRef {
	return &FlawRef{
		ID:                value.ID,
		Title:             value.Title,
		Summary:           value.Summary,
		SignalKeys:        normalizeList(value.SignalKeys),
		ContextTags:       normalizeList(value.ContextTags),
		Telltales:         normalizeStringList(value.Telltales),
		Contraindications: normalizeStringList(value.Contraindications),
		Avoid:             normalizeStringList(value.Avoid),
	}
}

func newMechanicRef(value MechanicDoc) MechanicRef {
	return MechanicRef{
		ID:                value.ID,
		Title:             value.Title,
		Summary:           value.Summary,
		Cues:              normalizeStringList(value.Cues),
		FailureModes:      normalizeStringList(value.FailureModes),
		RelatedSignalKeys: normalizeList(value.RelatedSignalKeys),
	}
}

func newScenarioRef(value ScenarioDoc) ScenarioRef {
	return ScenarioRef{
		ID:            value.ID,
		Name:          value.Name,
		Aliases:       normalizeStringList(value.Aliases),
		ScenarioTypes: normalizeList(value.ScenarioTypes),
		Summary:       value.Summary,
		WhatItTrains:  normalizeStringList(value.WhatItTrains),
		GoodForFlaws:  normalizeList(value.GoodForFlaws),
		Cautions:      normalizeStringList(value.Cautions),
	}
}

func newSourceRef(value SourceDoc) SourceRef {
	return SourceRef{
		ID:             value.ID,
		Kind:           value.Kind,
		Title:          value.Title,
		Author:         value.Author,
		URL:            value.URL,
		PublishedAtISO: value.PublishedAtISO,
	}
}

func collectMechanicSignals(values []MechanicRef) []string {
	out := make([]string, 0, len(values)*2)
	for _, value := range values {
		out = append(out, value.RelatedSignalKeys...)
	}
	return normalizeList(out)
}

func normalizeStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func readJSONDir[T any](dir string, allow func(string) bool) ([]T, error) {
	entries, err := fs.ReadDir(contentFS, dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	values := make([]T, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.ToSlash(filepath.Join(dir, entry.Name()))
		if allow != nil && !allow(path) {
			continue
		}
		raw, err := contentFS.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var value T
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		values = append(values, value)
	}
	return values, nil
}

func indexByID[T any](values []T, idFn func(T) string) map[string]T {
	indexed := make(map[string]T, len(values))
	for _, value := range values {
		indexed[normalizeToken(idFn(value))] = value
	}
	return indexed
}

func sortedKeysFromMap[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
