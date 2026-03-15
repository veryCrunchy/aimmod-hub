package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/kovaaksbenchmarks"
	"github.com/veryCrunchy/aimmod-hub/api/internal/service"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
)

// externalProfileResponse is returned by GET /api/lookup?q=<any>
type externalProfileResponse struct {
	// ResolvedSteamId is the canonical Steam64 ID (may be empty for KovaaK's-only lookups).
	// The frontend should redirect to /u/:resolvedSteamId after a successful lookup.
	ResolvedSteamId   string                     `json:"resolvedSteamId"`
	KovaaksUsername   string                     `json:"kovaaksUsername"`
	IsAimmodUser      bool                       `json:"isAimmodUser"`
	AimmodHandle      string                     `json:"aimmodHandle,omitempty"`
	AimmodDisplayName string                     `json:"aimmodDisplayName,omitempty"`
	Benchmarks        []externalBenchmarkSummary `json:"benchmarks"`
}

type externalBenchmarkSummary struct {
	BenchmarkID      uint32 `json:"benchmarkId"`
	BenchmarkName    string `json:"benchmarkName"`
	BenchmarkIconURL string `json:"benchmarkIconUrl"`
	BenchmarkAuthor  string `json:"benchmarkAuthor"`
	BenchmarkType    string `json:"benchmarkType"`
	OverallRankName  string `json:"overallRankName"`
	OverallRankIcon  string `json:"overallRankIcon"`
	OverallRankColor string `json:"overallRankColor"`
}

// externalBenchmarkPageResponse is returned by GET /api/lookup/benchmark?steamId=<id>&benchmarkId=<id>
type externalBenchmarkPageResponse struct {
	SteamID           string                       `json:"steamId"`
	KovaaksUsername   string                       `json:"kovaaksUsername"`
	IsAimmodUser      bool                         `json:"isAimmodUser"`
	AimmodHandle      string                       `json:"aimmodHandle,omitempty"`
	BenchmarkID       uint32                       `json:"benchmarkId"`
	BenchmarkName     string                       `json:"benchmarkName"`
	BenchmarkIconURL  string                       `json:"benchmarkIconUrl"`
	OverallRankIndex  uint32                       `json:"overallRankIndex"`
	OverallRankName   string                       `json:"overallRankName"`
	OverallRankIcon   string                       `json:"overallRankIcon"`
	OverallRankColor  string                       `json:"overallRankColor"`
	Ranks             []externalRankVisual         `json:"ranks"`
	Categories        []externalCategoryPage       `json:"categories"`
}

type externalRankVisual struct {
	RankIndex uint32 `json:"rankIndex"`
	RankName  string `json:"rankName"`
	IconURL   string `json:"iconUrl"`
	Color     string `json:"color"`
	FrameURL  string `json:"frameUrl"`
}

type externalCategoryPage struct {
	CategoryName string                  `json:"categoryName"`
	CategoryRank uint32                  `json:"categoryRank"`
	Scenarios    []externalScenarioPage  `json:"scenarios"`
}

type externalScenarioPage struct {
	ScenarioName    string               `json:"scenarioName"`
	Score           float64              `json:"score"`
	LeaderboardRank uint32               `json:"leaderboardRank"`
	LeaderboardID   uint32               `json:"leaderboardId"`
	RankIndex       uint32               `json:"rankIndex"`
	RankName        string               `json:"rankName"`
	RankIconURL     string               `json:"rankIconUrl"`
	RankColor       string               `json:"rankColor"`
	Thresholds      []externalThreshold  `json:"thresholds"`
}

type externalThreshold struct {
	RankIndex uint32  `json:"rankIndex"`
	RankName  string  `json:"rankName"`
	IconURL   string  `json:"iconUrl"`
	Color     string  `json:"color"`
	Score     float64 `json:"score"`
}

type playerSearchResult struct {
	Type        string `json:"type"` // "aimmod" | "kovaaks"
	SteamID     string `json:"steamId"`
	Handle      string `json:"handle,omitempty"`
	DisplayName string `json:"displayName"`
	Username    string `json:"username"`
	AvatarURL   string `json:"avatarUrl"`
	Country     string `json:"country,omitempty"`
	RunCount    int    `json:"runCount,omitempty"`
}

type playerSearchResponse struct {
	Players []playerSearchResult `json:"players"`
}

func newExternalHandler(hub *service.HubServer) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/lookup/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if q == "" {
			writeJSON(w, http.StatusOK, playerSearchResponse{Players: []playerSearchResult{}})
			return
		}
		handlePlayerSearch(w, r, hub, q)
	})
	mux.HandleFunc("/api/lookup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if q == "" {
			q = strings.TrimSpace(r.URL.Query().Get("steamId")) // backward compat
		}
		if q == "" {
			writeExternalJSONError(w, "q is required", http.StatusBadRequest)
			return
		}
		handleLookup(w, r, hub, q)
	})
	mux.HandleFunc("/api/lookup/benchmark", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if q == "" {
			q = strings.TrimSpace(r.URL.Query().Get("steamId"))
		}
		benchmarkIDStr := strings.TrimSpace(r.URL.Query().Get("benchmarkId"))
		if q == "" || benchmarkIDStr == "" {
			writeExternalJSONError(w, "q and benchmarkId are required", http.StatusBadRequest)
			return
		}
		benchmarkID64, err := strconv.ParseUint(benchmarkIDStr, 10, 32)
		if err != nil {
			writeExternalJSONError(w, "invalid benchmarkId", http.StatusBadRequest)
			return
		}
		handleLookupBenchmark(w, r, hub, q, uint32(benchmarkID64))
	})
	return mux
}

func handlePlayerSearch(w http.ResponseWriter, r *http.Request, hub *service.HubServer, q string) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	// Run KovaaK's API search and AimMod DB search in parallel.
	type kovaaksResult struct {
		users []kovaaksbenchmarks.KovaaksUserResult
		err   error
	}
	type aimmodResult struct {
		profiles []store.SearchProfileRecord
		err      error
	}

	kovaaksCh := make(chan kovaaksResult, 1)
	aimmodCh := make(chan aimmodResult, 1)

	go func() {
		users, err := hub.Benchmarks().SearchUsers(ctx, q, 10)
		kovaaksCh <- kovaaksResult{users, err}
	}()
	go func() {
		rec, err := hub.Store().Search(ctx, q)
		aimmodCh <- aimmodResult{rec.Profiles, err}
	}()

	kr := <-kovaaksCh
	ar := <-aimmodCh

	// Upsert KovaaK's results into the persistent cache (best-effort).
	if kr.err == nil && len(kr.users) > 0 {
		entries := make([]store.KovaaksUserCacheEntry, 0, len(kr.users))
		for _, u := range kr.users {
			entries = append(entries, store.KovaaksUserCacheEntry{
				SteamID:     u.SteamID,
				Username:    u.Username,
				DisplayName: u.DisplayName,
				AvatarURL:   u.AvatarURL,
				Country:     u.Country,
			})
		}
		_ = hub.Store().UpsertKovaaksUsers(ctx, entries)
	}

	// Also search the DB cache for users not returned by the live API
	// (covers previously-seen players whose username matches the query).
	cachedUsers, _ := hub.Store().SearchKovaaksUserCache(ctx, q, 10)

	// Build result list: AimMod users first, then KovaaK's-only players.
	// Use steamId to deduplicate — AimMod entries take precedence.
	seen := make(map[string]bool)
	players := make([]playerSearchResult, 0, 12)

	// AimMod users — enrich with Steam ID from linked accounts where possible.
	if ar.err == nil {
		for _, p := range ar.profiles {
			players = append(players, playerSearchResult{
				Type:        "aimmod",
				Handle:      p.UserHandle,
				DisplayName: p.UserDisplayName,
				Username:    p.UserHandle,
				AvatarURL:   p.AvatarURL,
				RunCount:    int(p.RunCount),
			})
			// Mark by handle so KovaaK's dupes are suppressed below.
			seen["handle:"+strings.ToLower(p.UserHandle)] = true
		}
	}

	// Merge KovaaK's live results and DB-cached results, deduplicating by Steam ID.
	type kovaaksEntry struct {
		steamID  string
		username string
		display  string
		avatar   string
		country  string
	}
	var kovaaksEntries []kovaaksEntry
	for _, u := range kr.users {
		kovaaksEntries = append(kovaaksEntries, kovaaksEntry{u.SteamID, u.Username, u.DisplayName, u.AvatarURL, u.Country})
	}
	for _, u := range cachedUsers {
		if !seen[u.SteamID] {
			kovaaksEntries = append(kovaaksEntries, kovaaksEntry{u.SteamID, u.Username, u.DisplayName, u.AvatarURL, u.Country})
		}
	}

	for _, e := range kovaaksEntries {
		if seen[e.steamID] {
			continue
		}
		// Skip if this is actually an AimMod user (matched by username).
		if seen["handle:"+strings.ToLower(e.username)] {
			continue
		}
		seen[e.steamID] = true
		players = append(players, playerSearchResult{
			Type:        "kovaaks",
			SteamID:     e.steamID,
			DisplayName: e.display,
			Username:    e.username,
			AvatarURL:   e.avatar,
			Country:     e.country,
		})
	}

	if players == nil {
		players = []playerSearchResult{}
	}
	writeJSON(w, http.StatusOK, playerSearchResponse{Players: players})
}

func handleLookup(w http.ResponseWriter, r *http.Request, hub *service.HubServer, q string) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	steam64, kovaaksUsername, isAimmodUser, aimmodHandle, aimmodDisplayName, err := resolveFullIdentity(ctx, hub, q)
	if err != nil {
		writeExternalJSONError(w, fmt.Sprintf("could not resolve user: %v", err), http.StatusBadGateway)
		return
	}
	if kovaaksUsername == "" {
		writeExternalJSONError(w, "could not find a KovaaK's player for that query", http.StatusNotFound)
		return
	}

	benchmarkList, err := hub.Benchmarks().ListPlayerBenchmarks(ctx, kovaaksUsername)
	if err != nil {
		writeExternalJSONError(w, fmt.Sprintf("could not fetch benchmarks: %v", err), http.StatusBadGateway)
		return
	}

	summaries := make([]externalBenchmarkSummary, 0, len(benchmarkList))
	for _, b := range benchmarkList {
		summaries = append(summaries, externalBenchmarkSummary{
			BenchmarkID:      b.BenchmarkID,
			BenchmarkName:    b.BenchmarkName,
			BenchmarkIconURL: b.BenchmarkIconURL,
			BenchmarkAuthor:  b.BenchmarkAuthor,
			BenchmarkType:    b.BenchmarkType,
			OverallRankName:  b.OverallRankName,
			OverallRankIcon:  b.OverallRankIcon,
			OverallRankColor: b.OverallRankColor,
		})
	}

	writeJSON(w, http.StatusOK, externalProfileResponse{
		ResolvedSteamId:   steam64,
		KovaaksUsername:   kovaaksUsername,
		IsAimmodUser:      isAimmodUser,
		AimmodHandle:      aimmodHandle,
		AimmodDisplayName: aimmodDisplayName,
		Benchmarks:        summaries,
	})
}

func handleLookupBenchmark(w http.ResponseWriter, r *http.Request, hub *service.HubServer, q string, benchmarkID uint32) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	steam64, kovaaksUsername, isAimmodUser, aimmodHandle, _, err := resolveFullIdentity(ctx, hub, q)
	if err != nil {
		writeExternalJSONError(w, fmt.Sprintf("could not resolve user: %v", err), http.StatusBadGateway)
		return
	}
	if steam64 == "" {
		writeExternalJSONError(w, "benchmark detail requires a Steam ID — provide a Steam64 ID or vanity URL", http.StatusUnprocessableEntity)
		return
	}

	benchmarkList, err := hub.Benchmarks().ListPlayerBenchmarks(ctx, kovaaksUsername)
	if err != nil {
		writeExternalJSONError(w, fmt.Sprintf("could not fetch benchmark list: %v", err), http.StatusBadGateway)
		return
	}
	var summary *kovaaksbenchmarks.ProfileBenchmarkSummary
	for i := range benchmarkList {
		if benchmarkList[i].BenchmarkID == benchmarkID {
			summary = &benchmarkList[i]
			break
		}
	}
	if summary == nil {
		summary = &kovaaksbenchmarks.ProfileBenchmarkSummary{BenchmarkID: benchmarkID}
	}

	detail, categories, err := hub.Benchmarks().BuildFullBenchmarkPage(ctx, *summary, steam64)
	if err != nil {
		writeExternalJSONError(w, fmt.Sprintf("could not fetch benchmark detail: %v", err), http.StatusBadGateway)
		return
	}
	if detail == nil {
		writeExternalJSONError(w, "benchmark not found", http.StatusNotFound)
		return
	}

	ranks := make([]externalRankVisual, 0, len(detail.Ranks))
	for _, rv := range detail.Ranks {
		ranks = append(ranks, externalRankVisual{
			RankIndex: rv.RankIndex,
			RankName:  rv.RankName,
			IconURL:   rv.IconURL,
			Color:     rv.Color,
			FrameURL:  rv.FrameURL,
		})
	}

	cats := make([]externalCategoryPage, 0, len(categories))
	for _, cat := range categories {
		scenarios := make([]externalScenarioPage, 0, len(cat.Scenarios))
		for _, sc := range cat.Scenarios {
			thresholds := make([]externalThreshold, 0, len(sc.Thresholds))
			for _, t := range sc.Thresholds {
				thresholds = append(thresholds, externalThreshold{
					RankIndex: t.RankIndex,
					RankName:  t.RankName,
					IconURL:   t.IconURL,
					Color:     t.Color,
					Score:     t.Score,
				})
			}
			scenarios = append(scenarios, externalScenarioPage{
				ScenarioName:    sc.ScenarioName,
				Score:           sc.Score,
				LeaderboardRank: sc.LeaderboardRank,
				LeaderboardID:   sc.LeaderboardID,
				RankIndex:       sc.ScenarioRank.RankIndex,
				RankName:        sc.ScenarioRank.RankName,
				RankIconURL:     sc.ScenarioRank.IconURL,
				RankColor:       sc.ScenarioRank.Color,
				Thresholds:      thresholds,
			})
		}
		cats = append(cats, externalCategoryPage{
			CategoryName: cat.CategoryName,
			CategoryRank: cat.CategoryRank,
			Scenarios:    scenarios,
		})
	}

	overallRank := kovaaksbenchmarks.RankVisualFromDetail(detail, detail.OverallRank)
	writeJSON(w, http.StatusOK, externalBenchmarkPageResponse{
		SteamID:          steam64,
		KovaaksUsername:  kovaaksUsername,
		IsAimmodUser:     isAimmodUser,
		AimmodHandle:     aimmodHandle,
		BenchmarkID:      benchmarkID,
		BenchmarkName:    summary.BenchmarkName,
		BenchmarkIconURL: summary.BenchmarkIconURL,
		OverallRankIndex: detail.OverallRank,
		OverallRankName:  overallRank.RankName,
		OverallRankIcon:  overallRank.IconURL,
		OverallRankColor: overallRank.Color,
		Ranks:            ranks,
		Categories:       cats,
	})
}

// resolveFullIdentity resolves any user query (Steam64, vanity, Steam URL,
// KovaaK's username) to a canonical identity. It first attempts Steam
// resolution via the benchmarks client (which caches results), then checks
// the AimMod DB by Steam64 and by KovaaK's username.
//
// steam64 may be empty when only a KovaaK's username could be determined
// (e.g. the input is a KovaaK's username with no Steam vanity match).
func resolveFullIdentity(
	ctx context.Context,
	hub *service.HubServer,
	q string,
) (steam64, kovaaksUsername string, isAimmodUser bool, aimmodHandle, aimmodDisplayName string, err error) {
	// Step 1 — Steam resolution (handles Steam64, vanity, profile URLs)
	steamResolved, _ := hub.Benchmarks().ResolveSteamInput(ctx, q)
	steam64 = steamResolved.Steam64
	kovaaksUsername = steamResolved.KovaaksUsername

	// Step 2 — If we have a Steam64 but the KovaaK's username came from the
	// Steam XML persona name (which may differ from the real KovaaK's username),
	// prefer the verified username from the kovaaks_user_cache table.
	if steam64 != "" {
		if cached, cErr := hub.Store().GetKovaaksUserBySteamId(ctx, steam64); cErr == nil && cached != nil && cached.Username != "" {
			kovaaksUsername = cached.Username
		}
	}

	// Step 3 — AimMod DB lookups
	var dbIdentity *store.BenchmarkUserIdentity

	if steam64 != "" {
		dbIdentity, err = hub.Store().GetBenchmarkIdentityBySteamId(ctx, steam64)
		if err != nil {
			return
		}
	}

	// Try by KovaaK's username candidates if DB not yet matched
	if dbIdentity == nil {
		for _, candidate := range unique(kovaaksUsername, q) {
			if candidate == "" {
				continue
			}
			dbIdentity, err = hub.Store().GetBenchmarkIdentityByKovaaksUsername(ctx, candidate)
			if err != nil {
				return
			}
			if dbIdentity != nil {
				if steam64 == "" {
					steam64 = dbIdentity.SteamID
				}
				kovaaksUsername = dbIdentity.KovaaksUsername
				break
			}
		}
	}

	if dbIdentity != nil {
		if kovaaksUsername == "" {
			kovaaksUsername = dbIdentity.KovaaksUsername
		}
		if steam64 == "" {
			steam64 = dbIdentity.SteamID
		}
		return steam64, kovaaksUsername, true, dbIdentity.UserHandle, dbIdentity.DisplayName, nil
	}

	// Not an AimMod user — fall back to raw query as KovaaK's username
	if kovaaksUsername == "" {
		kovaaksUsername = q
	}
	return steam64, kovaaksUsername, false, "", "", nil
}

// unique returns the unique non-empty strings from the arguments, in order.
func unique(vals ...string) []string {
	seen := make(map[string]bool, len(vals))
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func writeExternalJSONError(w http.ResponseWriter, msg string, code int) {
	writeJSON(w, code, map[string]string{"error": msg})
}
