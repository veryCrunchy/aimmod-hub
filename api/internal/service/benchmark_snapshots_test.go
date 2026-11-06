package service

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	hubv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1"
	"testing"
	"time"
)

func TestSnapshotPagesFreshnessAndPartial(t *testing.T) {
	data := &hubv1.GetBenchmarkLeaderboardResponse{}
	for i := 0; i < 30; i++ {
		data.Entries = append(data.Entries, &hubv1.BenchmarkLeaderboardEntry{UserHandle: fmt.Sprintf("synthetic%d", i)})
	}
	stamp := time.Now().UTC().Truncate(time.Second)
	req := connect.NewRequest(&hubv1.GetBenchmarkLeaderboardRequest{})
	req.Header().Set("X-Page", "2")
	req.Header().Set("X-Page-Size", "10")
	result := pageBenchmarkSnapshot(benchmarkSnapshot{data: data, updated: stamp, partial: true}, req)
	if len(result.Msg.Entries) != 10 || result.Msg.Entries[0].UserHandle != "synthetic10" || result.Header().Get("X-Total-Count") != "30" || result.Header().Get("X-Snapshot-Partial") != "true" || result.Header().Get("X-Snapshot-Updated") != stamp.Format(time.RFC3339) {
		t.Fatal(result)
	}
	if len(data.Entries) != 30 {
		t.Fatal("snapshot mutated")
	}
}
func TestIngestRequiresAuthenticatedIdentity(t *testing.T) {
	_, err := (&HubServer{}).IngestAuthorized(context.Background(), "", &hubv1.IngestSessionRequest{SteamId: "00000000000000001"})
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal(err)
	}
}
func TestNestedCollectionLimits(t *testing.T) {
	if ValidateIngestCollections(&hubv1.IngestSessionRequest{TimelineSeconds: make([]*hubv1.TimelineSecond, 7201)}) == nil {
		t.Fatal("oversized timeline accepted")
	}
}
