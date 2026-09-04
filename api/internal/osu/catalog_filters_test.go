package osu

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
	"google.golang.org/protobuf/proto"
)

func TestCatalogOfficialFiltersAndLength(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/token":
			_, _ = w.Write([]byte(`{"access_token":"test-token","expires_in":3600}`))
		case "/api/v2/beatmapsets/search":
			want := map[string]string{
				"q": "training stars>=0 stars<=5.75 bpm>=180 length>=100 length<=120 ar<=9.5 cs>=4 od<=8",
				"m": "0", "s": "ranked", "sort": "difficulty_asc",
			}
			for key, value := range want {
				if got := r.URL.Query().Get(key); got != value {
					t.Errorf("%s = %q; want %q", key, got, value)
				}
			}
			_, _ = w.Write([]byte(`{"cursor_string":"next-page","beatmapsets":[{"id":123,"beatmaps":[{"id":1,"mode":"osu","difficulty_rating":5,"hit_length":80,"total_length":110},{"id":2,"mode":"osu","difficulty_rating":7,"hit_length":130,"total_length":150}]}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	server, err := NewServer(Config{
		OfficialBaseURL: upstream.URL, OfficialClientID: "123", OfficialClientSecret: "secret",
		HTTPClient: upstream.Client(), ProviderRequestsPerSecond: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.SearchBeatmapItems(context.Background(), connect.NewRequest(&osuv1.SearchBeatmapItemsRequest{
		Query: " training ", Providers: []osuv1.Provider{osuv1.Provider_PROVIDER_OSU_OFFICIAL}, Sort: "difficulty_asc",
		Filters: &osuv1.BeatmapSearchFilters{
			Ruleset: osuv1.Ruleset_RULESET_OSU, Status: "ranked",
			Stars:             &osuv1.NumberRange{Minimum: proto.Float64(0), Maximum: proto.Float64(5.75)},
			Bpm:               &osuv1.NumberRange{Minimum: proto.Float64(180)},
			LengthSeconds:     &osuv1.NumberRange{Minimum: proto.Float64(100), Maximum: proto.Float64(120)},
			ApproachRate:      &osuv1.NumberRange{Maximum: proto.Float64(9.5)},
			CircleSize:        &osuv1.NumberRange{Minimum: proto.Float64(4)},
			OverallDifficulty: &osuv1.NumberRange{Maximum: proto.Float64(8)},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Msg.Items) != 1 || len(response.Msg.Items[0].Difficulties) != 2 {
		t.Fatalf("search must retain all difficulties in matching sets: %v", response.Msg)
	}
	if got := response.Msg.Items[0].Difficulties[0].LengthSeconds; got != 110 {
		t.Errorf("length = %d; want total length 110, matching official length filter", got)
	}
	if len(response.Msg.NextPageTokens) != 1 || response.Msg.NextPageTokens[0].PageToken != "next-page" {
		t.Errorf("pagination not preserved: %v", response.Msg.NextPageTokens)
	}
}

func TestCatalogRangeValidation(t *testing.T) {
	for _, value := range []*osuv1.NumberRange{
		nil, {}, {Minimum: proto.Float64(0)}, {Maximum: proto.Float64(0)},
		{Minimum: proto.Float64(4.5), Maximum: proto.Float64(4.5)},
	} {
		if err := validateRange("stars", value); err != nil {
			t.Errorf("valid range %v: %v", value, err)
		}
	}
	for _, value := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		for _, bounds := range []*osuv1.NumberRange{{Minimum: proto.Float64(value)}, {Maximum: proto.Float64(value)}} {
			if err := validateRange("stars", bounds); err == nil {
				t.Errorf("accepted invalid range %v", bounds)
			}
		}
	}
	if err := validateRange("stars", &osuv1.NumberRange{Minimum: proto.Float64(6), Maximum: proto.Float64(5)}); err == nil {
		t.Error("accepted reversed range")
	}
}

func TestCatalogOfficialSortMapping(t *testing.T) {
	for _, field := range []string{"artist", "creator", "difficulty", "favourites", "plays", "ranked", "relevance", "title", "updated"} {
		for _, direction := range []string{"asc", "desc"} {
			value := field + "_" + direction
			if got := officialSort(value); got != value {
				t.Errorf("sort %q = %q", value, got)
			}
		}
	}
	if got := officialSort(" DIFFICULTY_ASC "); got != "difficulty_asc" {
		t.Errorf("normalized sort = %q", got)
	}
	for _, value := range []string{"difficulty", "difficulty_up", "stars_asc", "unknown_desc"} {
		if err := validateSearchRequest(&osuv1.SearchBeatmapItemsRequest{Sort: value}); err == nil {
			t.Errorf("accepted unsupported sort %q", value)
		}
	}
}
