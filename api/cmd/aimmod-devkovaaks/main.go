// Command aimmod-devkovaaks serves a synthetic stand-in for the public
// KovaaK's web API and Steam profile XML, for local development only. Its
// players, scores and benchmarks are invented (see internal/devdata) and match
// what aimmod-devseed writes, so benchmark sheets, KovaaK's leaderboards and
// KovaaK's-only player pages all have data without contacting KovaaK's.
//
//	go run ./api/cmd/aimmod-devkovaaks -addr 127.0.0.1:18091
//	AIMMOD_KOVAAKS_API_BASE_URL=http://127.0.0.1:18091/webapp-backend \
//	AIMMOD_STEAM_COMMUNITY_BASE_URL=http://127.0.0.1:18091/steam \
//	  go run ./cmd/aimmod-hub
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/devdata"
)

type person struct {
	SteamID  string
	Username string
	Name     string
	Country  string
	Skill    float64
	// Plays is false for players who never touched a scenario.
	Benchmarks bool
}

var people []person
var baseURL string

func main() {
	addr := flag.String("addr", "127.0.0.1:18091", "listen address")
	flag.Parse()
	baseURL = "http://" + *addr
	for _, p := range devdata.Players {
		if p.Linked {
			people = append(people, person{SteamID: p.SteamID(), Username: p.KovaaksUsername(), Name: p.Name, Country: "nl", Skill: p.Skill, Benchmarks: true})
		}
	}
	for _, r := range devdata.RemotePlayers() {
		people = append(people, person{SteamID: r.SteamID, Username: r.Username, Name: r.Name, Country: r.Country, Skill: r.Skill, Benchmarks: true})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/webapp-backend/benchmarks/player-progress-rank", handleBenchmarkList)
	mux.HandleFunc("/webapp-backend/benchmarks/player-progress-rank-benchmark", handleBenchmarkDetail)
	mux.HandleFunc("/webapp-backend/leaderboard/scores/global", handleLeaderboard)
	mux.HandleFunc("/webapp-backend/scenario/popular", handleScenarioSearch)
	mux.HandleFunc("/webapp-backend/user/search", handleUserSearch)
	mux.HandleFunc("/webapp-backend/user/profile/by-username", handleProfile)
	mux.HandleFunc("/webapp-backend/user/scenario/total-play", handleUserScenarios)
	mux.HandleFunc("/steam/profiles/", handleSteamProfile)
	mux.HandleFunc("/steam/id/", handleSteamVanity)
	mux.HandleFunc("/icons/", handleIcon)
	log.Printf("synthetic KovaaK's provider on %s", baseURL)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func intParam(r *http.Request, key string, fallback int) int {
	if v, err := strconv.Atoi(r.URL.Query().Get(key)); err == nil && v >= 0 {
		return v
	}
	return fallback
}

func personBySteam(id string) *person {
	for i := range people {
		if people[i].SteamID == id {
			return &people[i]
		}
	}
	return nil
}

func personByUsername(name string) *person {
	for i := range people {
		if strings.EqualFold(people[i].Username, name) {
			return &people[i]
		}
	}
	return nil
}

func allScenarios() []devdata.Scenario {
	return append(append([]devdata.Scenario(nil), devdata.Scenarios...), devdata.ProviderOnlyScenarios...)
}

func scenarioByLeaderboard(id uint32) (devdata.Scenario, bool) {
	for _, sc := range allScenarios() {
		if sc.LeaderboardID == id {
			return sc, true
		}
	}
	return devdata.Scenario{}, false
}

// plays reports whether a synthetic person has a score on a scenario. The
// rare drill only has three entries so it reads as barely played.
func plays(p person, sc devdata.Scenario, index int) bool {
	if sc.Name == "Synthetic Rare Drill" {
		return index < 3
	}
	return true
}

type scoreRow struct {
	person person
	score  float64
	index  int
}

func board(sc devdata.Scenario) []scoreRow {
	var rows []scoreRow
	for i, p := range people {
		if !plays(p, sc, i) {
			continue
		}
		wobble := 1 + 0.03*math.Sin(float64(i)*1.7+float64(sc.LeaderboardID))
		rows = append(rows, scoreRow{person: p, score: math.Round(devdata.KovaaksScore(sc, p.Skill) * wobble), index: i})
	}
	sort.SliceStable(rows, func(a, b int) bool { return rows[a].score > rows[b].score })
	return rows
}

func rankOf(sc devdata.Scenario, steamID string) (int, float64) {
	for i, row := range board(sc) {
		if row.person.SteamID == steamID {
			return i + 1, row.score
		}
	}
	return 0, 0
}

func rankIcon(index int) string { return fmt.Sprintf("%s/icons/rank-%d.svg", baseURL, index) }

func ranksPayload() []map[string]string {
	out := []map[string]string{}
	for i, rank := range devdata.Ranks {
		out = append(out, map[string]string{"name": rank.Name, "icon": rankIcon(i), "color": rank.Color, "frame": ""})
	}
	return out
}

func benchmarkByID(id uint32) (devdata.Benchmark, bool) {
	for _, b := range devdata.Benchmarks {
		if b.ID == id {
			return b, true
		}
	}
	return devdata.Benchmark{}, false
}

func scenarioRank(score float64, maxes []float64) int {
	rank := 0
	for i, threshold := range maxes {
		if score >= threshold {
			rank = i + 1
		}
	}
	return rank
}

// orderedObject writes JSON objects in a fixed key order, like the real API.
type orderedObject struct {
	keys   []string
	values []any
}

func (o orderedObject) MarshalJSON() ([]byte, error) {
	var b strings.Builder
	b.WriteByte('{')
	for i, key := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(key)
		v, err := json.Marshal(o.values[i])
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return []byte(b.String()), nil
}

func detailFor(b devdata.Benchmark, steamID string) map[string]any {
	categories := orderedObject{}
	overall := math.MaxInt
	progress := 0.0
	for _, category := range b.Categories {
		scenarios := orderedObject{}
		categoryRank := math.MaxInt
		categoryProgress := 0.0
		for _, name := range category.Scenarios {
			sc, _ := devdata.ScenarioByName(name)
			maxes := devdata.Thresholds(sc, b.Difficulty)
			rank, score := rankOf(sc, steamID)
			r := scenarioRank(score, maxes)
			if r < categoryRank {
				categoryRank = r
			}
			categoryProgress += math.Min(100, score/maxes[len(maxes)-1]*100)
			var leaderboardRank any
			if rank > 0 {
				leaderboardRank = rank
			}
			scenarios.keys = append(scenarios.keys, name)
			scenarios.values = append(scenarios.values, map[string]any{
				"score": score * 100, "leaderboard_rank": leaderboardRank, "scenario_rank": r,
				"rank_maxes": maxes, "leaderboard_id": sc.LeaderboardID,
			})
		}
		if categoryRank == math.MaxInt {
			categoryRank = 0
		}
		if categoryRank < overall {
			overall = categoryRank
		}
		progress += categoryProgress
		categories.keys = append(categories.keys, category.Name)
		categories.values = append(categories.values, map[string]any{
			"benchmark_progress": math.Round(categoryProgress), "category_rank": categoryRank, "scenarios": scenarios,
		})
	}
	if overall == math.MaxInt {
		overall = 0
	}
	return map[string]any{
		"benchmark_progress": math.Round(progress),
		"overall_rank":       overall,
		"categories":         categories,
		"ranks":              ranksPayload(),
	}
}

func handleBenchmarkDetail(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("benchmarkId"))
	b, ok := benchmarkByID(uint32(id))
	if !ok {
		http.Error(w, "unknown benchmark", http.StatusNotFound)
		return
	}
	writeJSON(w, detailFor(b, r.URL.Query().Get("steamId")))
}

func handleBenchmarkList(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	page := intParam(r, "page", 0)
	max := intParam(r, "max", 300)
	p := personByUsername(username)
	var data []map[string]any
	for _, b := range devdata.Benchmarks {
		rankName, rankIndex := "No Rank", 0
		if p != nil {
			detail := detailFor(b, p.SteamID)
			rankIndex = detail["overall_rank"].(int)
			rankName = devdata.Ranks[rankIndex].Name
		}
		data = append(data, map[string]any{
			"benchmarkName": b.Name, "benchmarkId": b.ID, "benchmarkIcon": baseURL + "/icons/benchmark.svg",
			"benchmarkAuthor": b.Author, "type": "benchmark", "rankName": rankName, "rankIcon": rankIcon(rankIndex),
			"rankColor": devdata.Ranks[rankIndex].Color,
		})
	}
	start := page * max
	if start > len(data) {
		start = len(data)
	}
	end := start + max
	if end > len(data) {
		end = len(data)
	}
	writeJSON(w, map[string]any{"page": page, "max": max, "total": len(data), "data": data[start:end]})
}

func handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("leaderboardId"))
	sc, ok := scenarioByLeaderboard(uint32(id))
	if !ok {
		writeJSON(w, map[string]any{"total": 0, "page": 0, "max": 0, "data": []any{}})
		return
	}
	page := intParam(r, "page", 0)
	max := intParam(r, "max", 50)
	rows := board(sc)
	var data []map[string]any
	for i := page * max; i < len(rows) && i < (page+1)*max; i++ {
		row := rows[i]
		epoch := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC).Add(-time.Duration(row.index*37) * time.Hour).UnixMilli()
		data = append(data, map[string]any{
			"steamId": row.person.SteamID, "score": row.score, "rank": i + 1, "steamAccountName": row.person.Name,
			"webappUsername": row.person.Username, "country": row.person.Country,
			"attributes": map[string]any{"cm360": 25 + float64(row.index%9)*4.5, "epoch": epoch},
		})
	}
	writeJSON(w, map[string]any{"total": len(rows), "page": page, "max": max, "data": data})
}

func handleScenarioSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("scenarioNameSearch")))
	page := intParam(r, "page", 0)
	max := intParam(r, "max", 20)
	var matches []devdata.Scenario
	for _, sc := range allScenarios() {
		if q == "" || strings.Contains(strings.ToLower(sc.Name), q) {
			matches = append(matches, sc)
		}
	}
	var data []map[string]any
	for i := page * max; i < len(matches) && i < (page+1)*max; i++ {
		sc := matches[i]
		rows := board(sc)
		top := 0.0
		if len(rows) > 0 {
			top = rows[0].score
		}
		data = append(data, map[string]any{
			"rank": i + 1, "leaderboardId": sc.LeaderboardID, "scenarioName": sc.Name,
			"scenario": map[string]any{"aimType": sc.Kind, "authors": []string{"AimMod Dev"}},
			"counts":   map[string]any{"plays": len(rows) * 37, "entries": len(rows)},
			"topScore": map[string]any{"score": top},
		})
	}
	writeJSON(w, map[string]any{"page": page, "max": max, "total": len(matches), "data": data})
}

func handleUserSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("username")))
	max := intParam(r, "max", 10)
	out := []map[string]any{}
	for _, p := range people {
		if q != "" && (strings.Contains(strings.ToLower(p.Username), q) || strings.Contains(strings.ToLower(p.Name), q)) {
			out = append(out, map[string]any{"steamId": p.SteamID, "username": p.Username, "steamAccountName": p.Name, "steamAccountAvatar": "", "country": p.Country})
		}
		if len(out) >= max {
			break
		}
	}
	writeJSON(w, out)
}

func handleProfile(w http.ResponseWriter, r *http.Request) {
	p := personByUsername(r.URL.Query().Get("username"))
	if p == nil {
		writeJSON(w, map[string]any{})
		return
	}
	writeJSON(w, map[string]any{
		"steamId": p.SteamID, "steamAccountName": p.Name, "steamAccountAvatar": "", "country": p.Country,
		"created": "2023-03-14T10:00:00.000Z", "lastAccess": "2026-09-30T18:00:00.000Z",
		"scenariosPlayed": strconv.Itoa(len(allScenarios()) * 41), "webapp": map[string]any{"username": p.Username},
	})
}

func handleUserScenarios(w http.ResponseWriter, r *http.Request) {
	p := personByUsername(r.URL.Query().Get("username"))
	page := intParam(r, "page", 0)
	max := intParam(r, "max", 20)
	var data []map[string]any
	total := 0
	if p != nil {
		for i, sc := range allScenarios() {
			rank, score := rankOf(sc, p.SteamID)
			if rank == 0 {
				continue
			}
			total++
			if total <= page*max || total > (page+1)*max {
				continue
			}
			data = append(data, map[string]any{
				"leaderboardId": strconv.Itoa(int(sc.LeaderboardID)), "scenarioName": sc.Name,
				"counts": map[string]any{"plays": 12 + (i*7)%60}, "rank": rank, "score": score,
				"attributes": map[string]any{"cm360": 34, "epoch": time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC).UnixMilli()},
			})
		}
	}
	writeJSON(w, map[string]any{"page": page, "max": max, "total": total, "data": data})
}

func steamXML(w http.ResponseWriter, p *person) {
	w.Header().Set("Content-Type", "text/xml")
	if p == nil {
		fmt.Fprint(w, `<?xml version="1.0"?><response><error>The specified profile could not be found.</error></response>`)
		return
	}
	fmt.Fprintf(w, `<?xml version="1.0"?><profile><steamID64>%s</steamID64><steamID><![CDATA[%s]]></steamID></profile>`, p.SteamID, p.Name)
}

func handleSteamProfile(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/steam/profiles/"), "/")
	steamXML(w, personBySteam(id))
}

func handleSteamVanity(w http.ResponseWriter, r *http.Request) {
	steamXML(w, personByUsername(strings.Trim(strings.TrimPrefix(r.URL.Path, "/steam/id/"), "/")))
}

// handleIcon draws simple rank badges so no external images are needed.
func handleIcon(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/icons/"), ".svg")
	color := "#5ec8d8"
	label := "B"
	if strings.HasPrefix(name, "rank-") {
		index, _ := strconv.Atoi(strings.TrimPrefix(name, "rank-"))
		if index >= 0 && index < len(devdata.Ranks) {
			color = devdata.Ranks[index].Color
			if color == "" {
				color = "#555b66"
			}
			label = string([]rune(devdata.Ranks[index].Name)[0])
		}
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><polygon points="32,3 59,18 59,46 32,61 5,46 5,18" fill="%s" stroke="rgba(0,0,0,.35)" stroke-width="3"/><text x="32" y="41" font-family="sans-serif" font-size="24" font-weight="700" text-anchor="middle" fill="rgba(0,0,0,.65)">%s</text></svg>`, color, label)
}
