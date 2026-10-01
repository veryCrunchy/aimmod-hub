package store

import (
	"encoding/json"
	"math"
	"testing"
)

func TestSanitizeLiveActivityKeepsCompanionPayloadsCompatible(t *testing.T) {
	// A payload exactly as the companion app sends it: no client fields.
	var payload LiveActivityPayload
	if err := json.Unmarshal([]byte(`{"gameStateCode":3,"gameState":"Challenge","paused":false,"scenarioName":"Synthetic Scenario","runtimeLoaded":true,"bridgeConnected":false}`), &payload); err != nil {
		t.Fatal(err)
	}
	clean := sanitizeLiveActivityPayload(payload)
	if clean.Client != LiveClientCompanion || clean.Activity != "" || clean.SessionRunCount != nil || clean.SteamConnected != nil {
		t.Fatalf("companion payload gained fields: %+v", clean)
	}
	record := LiveActivityRecord{LiveActivityPayload: clean}
	record.finish()
	if !record.Active || record.Healthy {
		t.Fatalf("a reconnecting bridge must not count as healthy: %+v", record)
	}
}

func TestSanitizeLiveActivityInGameFields(t *testing.T) {
	elapsed, negative, runs := 125.0, -1.0, uint32(4)
	steam := true
	clean := sanitizeLiveActivityPayload(LiveActivityPayload{
		Client: " In-Game ", ClientVersion: "1.2.3\n", Activity: "Challenge",
		SessionRunCount: &runs, SessionElapsedSecs: &elapsed, SteamConnected: &steam,
		RuntimeLoaded: true, BridgeConnected: true,
	})
	if clean.Client != LiveClientInGame || clean.ClientVersion != "1.2.3" || clean.Activity != "challenge" || *clean.SessionElapsedSecs != 125 {
		t.Fatalf("unexpected sanitized payload: %+v", clean)
	}
	record := LiveActivityRecord{LiveActivityPayload: clean}
	record.finish()
	if !record.Healthy {
		t.Fatal("both checks passing is healthy")
	}

	odd := sanitizeLiveActivityPayload(LiveActivityPayload{Client: "<script>", Activity: "hacking", SessionElapsedSecs: &negative})
	if odd.Client != LiveClientCompanion || odd.Activity != "" || odd.SessionElapsedSecs != nil {
		t.Fatalf("invalid values were kept: %+v", odd)
	}
	nan := math.NaN()
	if sanitizeLiveActivityPayload(LiveActivityPayload{SessionElapsedSecs: &nan}).SessionElapsedSecs != nil {
		t.Fatal("NaN session time was kept")
	}
}

func TestLiveActivityRecordFromStoredCompanionJSON(t *testing.T) {
	// Rows stored before the client field existed read back as companion.
	var record LiveActivityRecord
	if err := json.Unmarshal([]byte(`{"gameState":"Idling","runtimeLoaded":true,"bridgeConnected":true}`), &record.LiveActivityPayload); err != nil {
		t.Fatal(err)
	}
	record.finish()
	if record.Client != LiveClientCompanion || !record.Healthy {
		t.Fatalf("stored companion row: %+v", record)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"runtimeLoaded", "bridgeConnected", "healthy", "client", "active"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("response is missing %q: %s", key, encoded)
		}
	}
}
