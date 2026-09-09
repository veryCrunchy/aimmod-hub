package osu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

type metadataIndex struct {
	memoryPlayerIndex
	item json.RawMessage
}

func (m *metadataIndex) GetIndexedOsuScoreMetadata(context.Context, int64) (json.RawMessage, error) {
	return m.item, nil
}

func TestScoreMetadataCrawlsNeverFetchUpstreamOrWriteIndex(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "unexpected upstream call", 500)
	}))
	defer upstream.Close()
	index := &metadataIndex{item: json.RawMessage(`{"source":"official","officialScoreId":"42","onlineScoreId":42,"osuUserId":7,"beatmapId":9,"visibility":"public"}`)}
	s, err := NewServer(Config{OfficialBaseURL: upstream.URL, OfficialClientID: "1", OfficialClientSecret: "example", PlayerIndex: index})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := s.GetPublicScoreMetadata(context.Background(), 42)
	if err != nil || detail.Status != "available" {
		t.Fatalf("stored metadata: %+v %v", detail, err)
	}
	for id := int64(100); id < 1100; id++ {
		detail, err = s.GetPublicScoreMetadata(context.Background(), id)
		if err != nil || detail.Status == "available" {
			t.Fatalf("unknown score: %+v %v", detail, err)
		}
	}
	index.item = json.RawMessage(`{"source":"official","officialScoreId":"42","onlineScoreId":42,"osuUserId":7,"beatmapId":9,"visibility":"private"}`)
	if detail, _ = s.GetPublicScoreMetadata(context.Background(), 42); detail.Status == "available" {
		t.Fatal("private data used in metadata")
	}
	key := "GET score-v20220705 " + s.official.client.resolve("/api/v2/scores/42", nil)
	s.official.client.cache.set(key, []byte(`{"id":42,"user_id":7,"ruleset_id":0,"beatmap_id":9,"pp":321.5}`))
	if detail, err = s.GetPublicScoreMetadata(context.Background(), 42); err != nil || detail.Status != "available" {
		t.Fatalf("cached metadata: %+v %v", detail, err)
	}
	if calls.Load() != 0 || len(index.scores) != 0 || len(index.players) != 0 {
		t.Fatal("metadata caused upstream requests or index writes")
	}
}
