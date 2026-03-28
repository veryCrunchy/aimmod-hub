package service

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"

	"github.com/veryCrunchy/aimmod-hub/api/internal/coaching"
	hubv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1"
)

func (s *HubServer) GetLearningIndex(
	_ context.Context,
	_ *connect.Request[hubv1.GetLearningIndexRequest],
) (*connect.Response[hubv1.GetLearningIndexResponse], error) {
	index, err := coaching.GetLearnIndex()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&hubv1.GetLearningIndexResponse{
		Version:           index.Version,
		UpdatedAtIso:      index.UpdatedAtISO,
		EntryCount:        uint32(index.EntryCount),
		SourceCount:       uint32(index.SourceCount),
		SignalKeyCount:    uint32(index.SignalKeyCount),
		ScenarioTypeCount: uint32(index.ScenarioTypeCount),
		ContextTagCount:   uint32(index.ContextTagCount),
		FeaturedEntries:   mapLearnEntryPreviews(index.FeaturedEntries),
		Entries:           mapLearnEntryPreviews(index.Entries),
		TopContextTags:    append([]string(nil), index.TopContextTags...),
		TopScenarioTypes:  append([]string(nil), index.TopScenarioTypes...),
	}), nil
}

func (s *HubServer) GetLearningEntry(
	_ context.Context,
	req *connect.Request[hubv1.GetLearningEntryRequest],
) (*connect.Response[hubv1.GetLearningEntryResponse], error) {
	entryID := strings.TrimSpace(req.Msg.GetEntryId())
	if entryID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("entry_id is required"))
	}

	entry, err := coaching.GetLearnEntry(entryID)
	if err != nil {
		code := connect.CodeInternal
		if errors.Is(err, coaching.ErrLearnEntryNotFound) {
			code = connect.CodeNotFound
		}
		return nil, connect.NewError(code, err)
	}

	return connect.NewResponse(&hubv1.GetLearningEntryResponse{
		Version:        entry.Version,
		UpdatedAtIso:   entry.UpdatedAtISO,
		Entry:          mapLearnEntry(entry.Entry),
		RelatedEntries: mapLearnEntryPreviews(entry.RelatedEntries),
	}), nil
}

func (s *HubServer) GetLearningTopic(
	_ context.Context,
	req *connect.Request[hubv1.GetLearningTopicRequest],
) (*connect.Response[hubv1.GetLearningTopicResponse], error) {
	topic := strings.TrimSpace(req.Msg.GetTopic())
	if topic == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("topic is required"))
	}

	response, err := coaching.GetLearnTopic(topic)
	if err != nil {
		code := connect.CodeInternal
		if errors.Is(err, coaching.ErrLearnEntryNotFound) {
			code = connect.CodeNotFound
		}
		return nil, connect.NewError(code, err)
	}

	return connect.NewResponse(&hubv1.GetLearningTopicResponse{
		Version:        response.Version,
		UpdatedAtIso:   response.UpdatedAtISO,
		Topic:          response.Topic,
		Title:          response.Title,
		Description:    response.Description,
		EntryCount:     uint32(response.EntryCount),
		FeaturedEntries: mapLearnEntryPreviews(response.FeaturedEntries),
		Entries:        mapLearnEntryPreviews(response.Entries),
		RelatedTopics:  append([]string(nil), response.RelatedTopics...),
	}), nil
}

func mapLearnEntryPreviews(previews []coaching.LearnEntryPreview) []*hubv1.LearnEntryPreview {
	out := make([]*hubv1.LearnEntryPreview, 0, len(previews))
	for _, preview := range previews {
		out = append(out, &hubv1.LearnEntryPreview{
			Id:            preview.ID,
			Title:         preview.Title,
			Summary:       preview.Summary,
			Priority:      preview.Priority,
			ScenarioTypes: append([]string(nil), preview.ScenarioTypes...),
			ContextTags:   append([]string(nil), preview.ContextTags...),
			SignalKeys:    append([]string(nil), preview.SignalKeys...),
			FocusAreas:    append([]string(nil), preview.FocusAreas...),
			SourceCount:   uint32(preview.SourceCount),
			DrillCount:    uint32(preview.DrillCount),
		})
	}
	return out
}

func mapLearnEntry(entry coaching.Entry) *hubv1.LearnEntry {
	return &hubv1.LearnEntry{
		Id:                   entry.ID,
		Title:                entry.Title,
		Summary:              entry.Summary,
		ScenarioTypes:        append([]string(nil), entry.ScenarioTypes...),
		ScenarioNames:        append([]string(nil), entry.ScenarioNames...),
		SignalKeys:           append([]string(nil), entry.SignalKeys...),
		ContextTags:          append([]string(nil), entry.ContextTags...),
		FocusAreas:           append([]string(nil), entry.FocusAreas...),
		ChallengePreferences: append([]string(nil), entry.ChallengePreferences...),
		TimePreferences:      append([]string(nil), entry.TimePreferences...),
		Why:                  append([]string(nil), entry.Why...),
		Actions:              append([]string(nil), entry.Actions...),
		Drills:               mapLearnDrills(entry.Drills),
		Avoid:                append([]string(nil), entry.Avoid...),
		Priority:             entry.Priority,
		Flaw:                 mapLearnFlaw(entry.Flaw),
		Mechanics:            mapLearnMechanics(entry.Mechanics),
		Scenarios:            mapLearnScenarios(entry.Scenarios),
		Evidence:             mapLearnEvidence(entry.Evidence),
		Sources:              mapLearnSources(entry.Sources),
	}
}

func mapLearnDrills(drills []coaching.Drill) []*hubv1.LearnDrill {
	out := make([]*hubv1.LearnDrill, 0, len(drills))
	for _, drill := range drills {
		out = append(out, &hubv1.LearnDrill{
			Label:  drill.Label,
			Query:  drill.Query,
			Reason: drill.Reason,
		})
	}
	return out
}

func mapLearnFlaw(flaw *coaching.FlawRef) *hubv1.LearnFlaw {
	if flaw == nil {
		return nil
	}
	return &hubv1.LearnFlaw{
		Id:                flaw.ID,
		Title:             flaw.Title,
		Summary:           flaw.Summary,
		SignalKeys:        append([]string(nil), flaw.SignalKeys...),
		ContextTags:       append([]string(nil), flaw.ContextTags...),
		Telltales:         append([]string(nil), flaw.Telltales...),
		Contraindications: append([]string(nil), flaw.Contraindications...),
		Avoid:             append([]string(nil), flaw.Avoid...),
	}
}

func mapLearnMechanics(mechanics []coaching.MechanicRef) []*hubv1.LearnMechanic {
	out := make([]*hubv1.LearnMechanic, 0, len(mechanics))
	for _, mechanic := range mechanics {
		out = append(out, &hubv1.LearnMechanic{
			Id:                mechanic.ID,
			Title:             mechanic.Title,
			Summary:           mechanic.Summary,
			Cues:              append([]string(nil), mechanic.Cues...),
			FailureModes:      append([]string(nil), mechanic.FailureModes...),
			RelatedSignalKeys: append([]string(nil), mechanic.RelatedSignalKeys...),
		})
	}
	return out
}

func mapLearnScenarios(scenarios []coaching.ScenarioRef) []*hubv1.LearnScenario {
	out := make([]*hubv1.LearnScenario, 0, len(scenarios))
	for _, scenario := range scenarios {
		out = append(out, &hubv1.LearnScenario{
			Id:            scenario.ID,
			Name:          scenario.Name,
			Aliases:       append([]string(nil), scenario.Aliases...),
			ScenarioTypes: append([]string(nil), scenario.ScenarioTypes...),
			Summary:       scenario.Summary,
			WhatItTrains:  append([]string(nil), scenario.WhatItTrains...),
			GoodForFlaws:  append([]string(nil), scenario.GoodForFlaws...),
			Cautions:      append([]string(nil), scenario.Cautions...),
		})
	}
	return out
}

func mapLearnEvidence(evidence []coaching.EvidenceRef) []*hubv1.LearnEvidence {
	out := make([]*hubv1.LearnEvidence, 0, len(evidence))
	for _, item := range evidence {
		out = append(out, &hubv1.LearnEvidence{
			SourceId:     item.SourceID,
			Claim:        item.Claim,
			Excerpt:      item.Excerpt,
			StartSec:     item.StartSec,
			EndSec:       item.EndSec,
			Confidence:   item.Confidence,
			ReviewStatus: item.ReviewStatus,
		})
	}
	return out
}

func mapLearnSources(sources []coaching.SourceRef) []*hubv1.LearnSource {
	out := make([]*hubv1.LearnSource, 0, len(sources))
	for _, source := range sources {
		out = append(out, &hubv1.LearnSource{
			Id:             source.ID,
			Kind:           source.Kind,
			Title:          source.Title,
			Author:         source.Author,
			Url:            source.URL,
			PublishedAtIso: source.PublishedAtISO,
		})
	}
	return out
}
