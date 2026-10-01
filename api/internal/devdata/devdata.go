// Package devdata describes the synthetic world used for local development:
// invented players, scenarios and benchmarks shared by the database seed and
// the synthetic KovaaK's provider so both tell the same story. Nothing here
// refers to a real person or account.
package devdata

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type Player struct {
	ExternalID string
	Name       string
	Skill      float64 // 0.6 .. 1.15 multiplier on scenario base scores
	Activity   int     // runs per active day
	Focus      string  // preferred scenario type
	// Linked marks players whose synthetic Steam and KovaaK's accounts are
	// linked through their AimMod uploads.
	Linked bool
}

type Scenario struct {
	Name          string
	Kind          string
	Base          float64 // typical score for skill 1.0
	Accuracy      float64 // typical accuracy for skill 1.0, percent
	Duration      time.Duration
	LeaderboardID uint32
}

var Players = []Player{
	{"devseed:kestrel", "Demo Kestrel", 1.12, 9, "Tracking", true},
	{"devseed:lumen", "Demo Lumen", 1.05, 7, "OneShotClicking", true},
	{"devseed:quartz", "Demo Quartz", 0.98, 6, "TargetSwitching", true},
	{"devseed:ember", "Demo Ember", 0.93, 8, "Tracking", true},
	{"devseed:nimbus", "Demo Nimbus", 0.88, 4, "OneShotClicking", true},
	{"devseed:sable", "Demo Sable", 0.84, 5, "TargetSwitching", true},
	{"devseed:vireo", "Demo Vireo", 0.79, 3, "Tracking", false},
	{"devseed:onyx", "Demo Onyx", 0.74, 3, "OneShotClicking", false},
	{"devseed:pike", "Demo Pike", 0.69, 2, "TargetSwitching", false},
	{"devseed:wren", "Demo Wren", 0.64, 2, "Tracking", false},
}

var Scenarios = []Scenario{
	{"Synthetic Smoothbot", "Tracking", 3200, 78, 60 * time.Second, 90001},
	{"Synthetic Strafe Track", "Tracking", 2900, 74, 60 * time.Second, 90002},
	{"Synthetic Air Track", "Tracking", 2600, 70, 60 * time.Second, 90003},
	{"Synthetic Tile Click", "OneShotClicking", 1150, 92, 60 * time.Second, 90004},
	{"Synthetic Micro Flick", "OneShotClicking", 980, 88, 60 * time.Second, 90005},
	{"Synthetic Reflex Grid", "OneShotClicking", 1050, 86, 60 * time.Second, 90006},
	{"Synthetic Switch Duo", "TargetSwitching", 780, 81, 60 * time.Second, 90007},
	{"Synthetic Switch Wide", "TargetSwitching", 690, 79, 60 * time.Second, 90008},
	{"Synthetic Bounce Switch", "TargetSwitching", 720, 76, 45 * time.Second, 90009},
	{"Synthetic Long Track", "Tracking", 4100, 72, 90 * time.Second, 90010},
}

// Extra scenarios that only exist on the synthetic KovaaK's side.
var ProviderOnlyScenarios = []Scenario{
	{"Synthetic Rare Drill", "Clicking", 500, 80, 60 * time.Second, 90099},
	{"Synthetic Warmup Grid", "Clicking", 900, 90, 60 * time.Second, 90011},
	{"Synthetic Flow Track", "Tracking", 3000, 75, 60 * time.Second, 90012},
}

// Handle is the synthetic player's AimMod handle.
func (p Player) Handle() string {
	return strings.ToLower(strings.ReplaceAll(p.Name, " ", "-"))
}

// SteamID is an invented Steam64-shaped id that belongs to no account.
func (p Player) SteamID() string {
	for i, other := range Players {
		if other.ExternalID == p.ExternalID {
			return fmt.Sprintf("765611900000%05d", 1001+i)
		}
	}
	return ""
}

// KovaaksUsername is the synthetic KovaaK's username.
func (p Player) KovaaksUsername() string { return p.Handle() }

// RemotePlayer is a synthetic KovaaK's player who has never used AimMod.
type RemotePlayer struct {
	SteamID  string
	Username string
	Name     string
	Country  string
	Skill    float64
}

var countries = []string{"us", "de", "nl", "se", "jp", "br", "gb", "fr", "pl", "kr", "ca", "au"}

// RemotePlayers lists synthetic KovaaK's-only players with spread-out skill.
func RemotePlayers() []RemotePlayer {
	out := make([]RemotePlayer, 0, 48)
	for i := 0; i < 48; i++ {
		out = append(out, RemotePlayer{
			SteamID:  fmt.Sprintf("765611900000%05d", 2001+i),
			Username: fmt.Sprintf("synthetic-remote-%02d", i+1),
			Name:     fmt.Sprintf("Synthetic Remote %02d", i+1),
			Country:  countries[i%len(countries)],
			// Deterministic spread from about 0.5 to 1.3.
			Skill: 0.5 + 0.8*math.Mod(float64(i)*0.6180339887, 1),
		})
	}
	return out
}

type Rank struct {
	Name  string
	Color string
}

// Ranks is the synthetic rank ladder; index 0 is the unranked entry.
var Ranks = []Rank{
	{"No Rank", ""},
	{"Iron", "#8a8f98"},
	{"Bronze", "#b07840"},
	{"Silver", "#b8c0c8"},
	{"Gold", "#e0b840"},
	{"Platinum", "#5ec8d8"},
	{"Diamond", "#7aa8ff"},
	{"Master", "#c070f0"},
}

type BenchmarkCategory struct {
	Name      string
	Scenarios []string
}

type Benchmark struct {
	ID         uint32
	Name       string
	Author     string
	Difficulty float64 // threshold multiplier
	Categories []BenchmarkCategory
}

var standardCategories = []BenchmarkCategory{
	{"Clicking Static", []string{"Synthetic Tile Click", "Synthetic Micro Flick"}},
	{"Clicking Dynamic", []string{"Synthetic Reflex Grid"}},
	{"Tracking Smooth", []string{"Synthetic Smoothbot", "Synthetic Long Track"}},
	{"Tracking Reactive", []string{"Synthetic Strafe Track", "Synthetic Air Track"}},
	{"Switching Speed", []string{"Synthetic Switch Duo", "Synthetic Switch Wide"}},
	{"Switching Evasive", []string{"Synthetic Bounce Switch"}},
}

// Benchmarks are the synthetic benchmark sheets. The last four are the
// kinds of entries the catalog filter should hide.
var Benchmarks = []Benchmark{
	{9101, "Synthetic Benchmarks S1 Novice", "AimMod Dev", 0.8, standardCategories},
	{9102, "Synthetic Benchmarks S1 Intermediate", "AimMod Dev", 1.0, standardCategories},
	{9103, "Synthetic Benchmarks S1 Advanced", "AimMod Dev", 1.2, standardCategories},
	{9108, "Synthetic Tracking Focus", "AimMod Dev", 0.95, []BenchmarkCategory{
		{"Tracking Smooth", []string{"Synthetic Smoothbot", "Synthetic Flow Track"}},
		{"Tracking Reactive", []string{"Synthetic Strafe Track"}},
	}},
	{9104, "Bench 1212311231231", "someone", 1.0, []BenchmarkCategory{{"Mixed", []string{"Synthetic Warmup Grid"}}}},
	{9105, "test", "someone", 1.0, []BenchmarkCategory{{"Mixed", []string{"Synthetic Warmup Grid"}}}},
	{9106, "Synthetic Empty Sheet", "someone", 1.0, nil},
	{9107, "Synthetic Tiny Sheet", "someone", 1.0, []BenchmarkCategory{{"Mixed", []string{"Synthetic Rare Drill"}}}},
}

// ScenarioByName finds a seeded or provider-only scenario.
func ScenarioByName(name string) (Scenario, bool) {
	for _, list := range [][]Scenario{Scenarios, ProviderOnlyScenarios} {
		for _, sc := range list {
			if strings.EqualFold(sc.Name, name) {
				return sc, true
			}
		}
	}
	return Scenario{}, false
}

// Thresholds returns rank_maxes for a scenario on a benchmark: the minimum
// score for ranks 1..len(Ranks)-1.
func Thresholds(sc Scenario, difficulty float64) []float64 {
	out := make([]float64, 0, len(Ranks)-1)
	for r := 1; r < len(Ranks); r++ {
		out = append(out, math.Round(sc.Base*difficulty*(0.5+0.085*float64(r))))
	}
	return out
}

// KovaaksScore is the synthetic KovaaK's best score for a skill level. It is
// a little below what the same player uploads through AimMod, so pages show
// both score sources.
func KovaaksScore(sc Scenario, skill float64) float64 {
	return math.Round(sc.Base * skill * 0.97)
}
