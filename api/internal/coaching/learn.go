package coaching

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var ErrLearnEntryNotFound = errors.New("coaching learn entry not found")

type LearnEntryPreview struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Summary       string   `json:"summary"`
	Priority      string   `json:"priority"`
	ScenarioTypes []string `json:"scenarioTypes"`
	ContextTags   []string `json:"contextTags"`
	SignalKeys    []string `json:"signalKeys"`
	FocusAreas    []string `json:"focusAreas"`
	SourceCount   int      `json:"sourceCount"`
	DrillCount    int      `json:"drillCount"`
}

type LearnIndexResponse struct {
	Version           string             `json:"version"`
	UpdatedAtISO      string             `json:"updatedAtIso"`
	EntryCount        int                `json:"entryCount"`
	SourceCount       int                `json:"sourceCount"`
	SignalKeyCount    int                `json:"signalKeyCount"`
	ScenarioTypeCount int                `json:"scenarioTypeCount"`
	ContextTagCount   int                `json:"contextTagCount"`
	FeaturedEntries   []LearnEntryPreview `json:"featuredEntries"`
	Entries           []LearnEntryPreview `json:"entries"`
	TopContextTags    []string           `json:"topContextTags"`
	TopScenarioTypes  []string           `json:"topScenarioTypes"`
}

type LearnEntryResponse struct {
	Version        string             `json:"version"`
	UpdatedAtISO   string             `json:"updatedAtIso"`
	Entry          Entry              `json:"entry"`
	RelatedEntries []LearnEntryPreview `json:"relatedEntries"`
}

type LearnTopicResponse struct {
	Version        string              `json:"version"`
	UpdatedAtISO   string              `json:"updatedAtIso"`
	Topic          string              `json:"topic"`
	Title          string              `json:"title"`
	Description    string              `json:"description"`
	EntryCount     int                 `json:"entryCount"`
	FeaturedEntries []LearnEntryPreview `json:"featuredEntries"`
	Entries        []LearnEntryPreview `json:"entries"`
	RelatedTopics  []string            `json:"relatedTopics"`
}

func GetLearnTopics() ([]string, error) {
	base, err := Load()
	if err != nil {
		return nil, err
	}
	freq := map[string]int{}
	for _, entry := range base.Entries {
		for _, value := range entry.ContextTags {
			normalized := normalizeToken(value)
			if normalized != "" {
				freq[normalized]++
			}
		}
		for _, value := range entry.ScenarioTypes {
			normalized := normalizeToken(value)
			if normalized != "" {
				freq[normalized]++
			}
		}
	}
	return topKeysByFrequency(freq, len(freq)), nil
}

func GetLearnIndex() (LearnIndexResponse, error) {
	base, err := Load()
	if err != nil {
		return LearnIndexResponse{}, err
	}

	previews := make([]LearnEntryPreview, 0, len(base.Entries))
	sourceIDs := make(map[string]struct{})
	signalKeys := make(map[string]struct{})
	scenarioTypes := make(map[string]struct{})
	contextTags := make(map[string]struct{})
	contextTagFreq := make(map[string]int)
	scenarioTypeFreq := make(map[string]int)

	for _, entry := range base.Entries {
		previews = append(previews, previewForEntry(entry))
		for _, source := range entry.Sources {
			if source.ID != "" {
				sourceIDs[source.ID] = struct{}{}
			}
		}
		for _, value := range entry.SignalKeys {
			signalKeys[value] = struct{}{}
		}
		for _, value := range entry.ScenarioTypes {
			scenarioTypes[value] = struct{}{}
			scenarioTypeFreq[value]++
		}
		for _, value := range entry.ContextTags {
			contextTags[value] = struct{}{}
			contextTagFreq[value]++
		}
	}

	sortLearnPreviews(previews)

	featuredCount := 8
	if len(previews) < featuredCount {
		featuredCount = len(previews)
	}

	return LearnIndexResponse{
		Version:           base.Version,
		UpdatedAtISO:      base.UpdatedAtISO,
		EntryCount:        len(base.Entries),
		SourceCount:       len(sourceIDs),
		SignalKeyCount:    len(signalKeys),
		ScenarioTypeCount: len(scenarioTypes),
		ContextTagCount:   len(contextTags),
		FeaturedEntries:   append([]LearnEntryPreview(nil), previews[:featuredCount]...),
		Entries:           previews,
		TopContextTags:    topKeysByFrequency(contextTagFreq, 12),
		TopScenarioTypes:  topKeysByFrequency(scenarioTypeFreq, 8),
	}, nil
}

func GetLearnEntry(id string) (LearnEntryResponse, error) {
	base, err := Load()
	if err != nil {
		return LearnEntryResponse{}, err
	}

	normalizedID := normalizeToken(id)
	if normalizedID == "" {
		return LearnEntryResponse{}, ErrLearnEntryNotFound
	}

	var current *Entry
	for i := range base.Entries {
		if normalizeToken(base.Entries[i].ID) == normalizedID {
			current = &base.Entries[i]
			break
		}
	}
	if current == nil {
		return LearnEntryResponse{}, ErrLearnEntryNotFound
	}

	type relatedMatch struct {
		preview LearnEntryPreview
		score   int
	}

	related := make([]relatedMatch, 0, len(base.Entries))
	for _, candidate := range base.Entries {
		if candidate.ID == current.ID {
			continue
		}
		score := overlapScore(*current, candidate)
		if score <= 0 {
			continue
		}
		related = append(related, relatedMatch{
			preview: previewForEntry(candidate),
			score:   score,
		})
	}

	sort.Slice(related, func(i, j int) bool {
		if related[i].score != related[j].score {
			return related[i].score > related[j].score
		}
		if priorityRank(related[i].preview.Priority) != priorityRank(related[j].preview.Priority) {
			return priorityRank(related[i].preview.Priority) > priorityRank(related[j].preview.Priority)
		}
		return related[i].preview.Title < related[j].preview.Title
	})

	relatedPreviews := make([]LearnEntryPreview, 0, min(6, len(related)))
	for _, match := range related {
		relatedPreviews = append(relatedPreviews, match.preview)
		if len(relatedPreviews) >= 6 {
			break
		}
	}

	return LearnEntryResponse{
		Version:        base.Version,
		UpdatedAtISO:   base.UpdatedAtISO,
		Entry:          *current,
		RelatedEntries: relatedPreviews,
	}, nil
}

func GetLearnTopic(topic string) (LearnTopicResponse, error) {
	base, err := Load()
	if err != nil {
		return LearnTopicResponse{}, err
	}

	normalizedTopic := normalizeToken(topic)
	if normalizedTopic == "" {
		return LearnTopicResponse{}, ErrLearnEntryNotFound
	}

	previews := make([]LearnEntryPreview, 0)
	relatedFreq := map[string]int{}

	for _, entry := range base.Entries {
		if !entryMatchesTopic(entry, normalizedTopic) {
			continue
		}
		previews = append(previews, previewForEntry(entry))
		for _, tag := range entry.ContextTags {
			normalized := normalizeToken(tag)
			if normalized != "" && normalized != normalizedTopic {
				relatedFreq[normalized]++
			}
		}
		for _, value := range entry.ScenarioTypes {
			normalized := normalizeToken(value)
			if normalized != "" && normalized != normalizedTopic {
				relatedFreq[normalized]++
			}
		}
	}

	if len(previews) == 0 {
		return LearnTopicResponse{}, ErrLearnEntryNotFound
	}

	sortLearnPreviews(previews)
	featuredCount := 6
	if len(previews) < featuredCount {
		featuredCount = len(previews)
	}

	title := humanizeLearnToken(normalizedTopic)
	return LearnTopicResponse{
		Version:         base.Version,
		UpdatedAtISO:    base.UpdatedAtISO,
		Topic:           normalizedTopic,
		Title:           title,
		Description:     fmt.Sprintf("KB-backed aim training guides related to %s, generated from AimMod's coaching knowledge.", strings.ToLower(title)),
		EntryCount:      len(previews),
		FeaturedEntries: append([]LearnEntryPreview(nil), previews[:featuredCount]...),
		Entries:         previews,
		RelatedTopics:   topKeysByFrequency(relatedFreq, 10),
	}, nil
}

func previewForEntry(entry Entry) LearnEntryPreview {
	return LearnEntryPreview{
		ID:            entry.ID,
		Title:         entry.Title,
		Summary:       entry.Summary,
		Priority:      entry.Priority,
		ScenarioTypes: append([]string(nil), entry.ScenarioTypes...),
		ContextTags:   append([]string(nil), entry.ContextTags...),
		SignalKeys:    append([]string(nil), entry.SignalKeys...),
		FocusAreas:    append([]string(nil), entry.FocusAreas...),
		SourceCount:   len(entry.Sources),
		DrillCount:    len(entry.Drills),
	}
}

func sortLearnPreviews(previews []LearnEntryPreview) {
	sort.Slice(previews, func(i, j int) bool {
		if priorityRank(previews[i].Priority) != priorityRank(previews[j].Priority) {
			return priorityRank(previews[i].Priority) > priorityRank(previews[j].Priority)
		}
		if previews[i].SourceCount != previews[j].SourceCount {
			return previews[i].SourceCount > previews[j].SourceCount
		}
		if previews[i].DrillCount != previews[j].DrillCount {
			return previews[i].DrillCount > previews[j].DrillCount
		}
		return previews[i].Title < previews[j].Title
	})
}

func topKeysByFrequency(freq map[string]int, limit int) []string {
	type item struct {
		key   string
		count int
	}
	items := make([]item, 0, len(freq))
	for key, count := range freq {
		items = append(items, item{key: key, count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		return items[i].key < items[j].key
	})
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.key)
	}
	return out
}

func entryMatchesTopic(entry Entry, topic string) bool {
	if sharedCount([]string{topic}, entry.ContextTags) > 0 {
		return true
	}
	if sharedCount([]string{topic}, entry.ScenarioTypes) > 0 {
		return true
	}
	if sharedCount([]string{topic}, entry.SignalKeys) > 0 {
		return true
	}
	if sharedCount([]string{topic}, entry.FocusAreas) > 0 {
		return true
	}
	if entry.Flaw != nil {
		if sharedCount([]string{topic}, entry.Flaw.SignalKeys) > 0 {
			return true
		}
		if sharedCount([]string{topic}, entry.Flaw.ContextTags) > 0 {
			return true
		}
	}
	return false
}

func humanizeLearnToken(value string) string {
	value = strings.ReplaceAll(strings.ReplaceAll(value, "_", " "), "-", " ")
	parts := strings.Fields(value)
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}

func overlapScore(a, b Entry) int {
	score := 0
	score += sharedCount(a.SignalKeys, b.SignalKeys) * 4
	score += sharedCount(a.ContextTags, b.ContextTags) * 3
	score += sharedCount(a.ScenarioTypes, b.ScenarioTypes) * 3
	score += sharedCount(a.FocusAreas, b.FocusAreas) * 2

	if a.Flaw != nil && b.Flaw != nil && normalizeToken(a.Flaw.ID) == normalizeToken(b.Flaw.ID) {
		score += 5
	}
	if sharesMechanicID(a.Mechanics, b.Mechanics) {
		score += 3
	}
	return score
}

func sharedCount(a, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := make(map[string]struct{}, len(a))
	for _, value := range a {
		normalized := normalizeToken(value)
		if normalized != "" {
			set[normalized] = struct{}{}
		}
	}
	count := 0
	seen := map[string]struct{}{}
	for _, value := range b {
		normalized := normalizeToken(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		if _, ok := set[normalized]; ok {
			count++
			seen[normalized] = struct{}{}
		}
	}
	return count
}

func sharesMechanicID(a, b []MechanicRef) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(a))
	for _, value := range a {
		normalized := normalizeToken(value.ID)
		if normalized != "" {
			set[normalized] = struct{}{}
		}
	}
	for _, value := range b {
		if _, ok := set[normalizeToken(value.ID)]; ok {
			return true
		}
	}
	return false
}

func LearnCanonicalID(id string) string {
	return strings.TrimSpace(normalizeToken(id))
}
