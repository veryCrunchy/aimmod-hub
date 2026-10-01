// Command aimmod-devseed fills a local development database with synthetic
// players, runs, live sessions and tournaments so every page has data to show.
//
// It only runs against the database named by AIMMOD_DEVSEED_DATABASE_URL (or
// -database-url), never DATABASE_URL, so it cannot write into a deployment by
// accident. All identities are invented.
//
//	AIMMOD_DEVSEED_DATABASE_URL=postgres://postgres:postgres@localhost:5432/aimmod_hub?sslmode=disable \
//	  go run ./api/cmd/aimmod-devseed
//
// Pass -live to keep refreshing the synthetic live sessions until interrupted.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"time"

	hubv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/hub/v1"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	"github.com/veryCrunchy/aimmod-hub/api/internal/tournament"
	"github.com/veryCrunchy/aimmod-hub/api/internal/tournament/bracket"
)

type player struct {
	externalID string
	name       string
	skill      float64 // 0.6 .. 1.15 multiplier on scenario base scores
	activity   int     // runs per active day
	focus      string  // preferred scenario type
}

type scenario struct {
	name     string
	kind     string
	base     float64 // typical score for skill 1.0
	accuracy float64 // typical accuracy for skill 1.0, percent
	duration time.Duration
}

var players = []player{
	{"devseed:kestrel", "Demo Kestrel", 1.12, 9, "Tracking"},
	{"devseed:lumen", "Demo Lumen", 1.05, 7, "OneShotClicking"},
	{"devseed:quartz", "Demo Quartz", 0.98, 6, "TargetSwitching"},
	{"devseed:ember", "Demo Ember", 0.93, 8, "Tracking"},
	{"devseed:nimbus", "Demo Nimbus", 0.88, 4, "OneShotClicking"},
	{"devseed:sable", "Demo Sable", 0.84, 5, "TargetSwitching"},
	{"devseed:vireo", "Demo Vireo", 0.79, 3, "Tracking"},
	{"devseed:onyx", "Demo Onyx", 0.74, 3, "OneShotClicking"},
	{"devseed:pike", "Demo Pike", 0.69, 2, "TargetSwitching"},
	{"devseed:wren", "Demo Wren", 0.64, 2, "Tracking"},
}

var scenarios = []scenario{
	{"Synthetic Smoothbot", "Tracking", 3200, 78, 60 * time.Second},
	{"Synthetic Strafe Track", "Tracking", 2900, 74, 60 * time.Second},
	{"Synthetic Air Track", "Tracking", 2600, 70, 60 * time.Second},
	{"Synthetic Tile Click", "OneShotClicking", 1150, 92, 60 * time.Second},
	{"Synthetic Micro Flick", "OneShotClicking", 980, 88, 60 * time.Second},
	{"Synthetic Reflex Grid", "OneShotClicking", 1050, 86, 60 * time.Second},
	{"Synthetic Switch Duo", "TargetSwitching", 780, 81, 60 * time.Second},
	{"Synthetic Switch Wide", "TargetSwitching", 690, 79, 60 * time.Second},
	{"Synthetic Bounce Switch", "TargetSwitching", 720, 76, 45 * time.Second},
	{"Synthetic Long Track", "Tracking", 4100, 72, 90 * time.Second},
}

func main() {
	databaseURL := flag.String("database-url", os.Getenv("AIMMOD_DEVSEED_DATABASE_URL"), "development database (defaults to AIMMOD_DEVSEED_DATABASE_URL)")
	days := flag.Int("days", 45, "days of history to generate")
	live := flag.Bool("live", false, "keep refreshing synthetic live sessions until interrupted")
	liveOnly := flag.Bool("live-only", false, "skip runs and tournaments; only write live sessions")
	flag.Parse()
	if strings.TrimSpace(*databaseURL) == "" {
		log.Fatal("set AIMMOD_DEVSEED_DATABASE_URL or -database-url to a local development database")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	s, err := store.Open(ctx, *databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	rng := rand.New(rand.NewSource(20260601))
	now := time.Now().UTC().Truncate(time.Minute)
	if !*liveOnly {
		for i, p := range players {
			// A linked account gives each synthetic player a readable handle and name.
			link := store.DiscordLink{UserExternalID: p.externalID, DiscordUserID: fmt.Sprintf("devseed-%02d", i), Username: handleFor(p), GlobalName: p.name}
			if err := s.LinkDiscordAccount(ctx, link); err != nil {
				log.Fatal(err)
			}
		}
		count, err := seedRuns(ctx, s, rng, now, *days)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("stored %d synthetic runs for %d players", count, len(players))
		if err := seedTournaments(ctx, s, now); err != nil {
			log.Fatal(err)
		}
	}
	if err := seedLive(ctx, s, rng); err != nil {
		log.Fatal(err)
	}
	if !*live {
		return
	}
	log.Print("refreshing live sessions every 30s; press Ctrl+C to stop")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := seedLive(ctx, s, rng); err != nil {
				log.Print(err)
			}
		}
	}
}

func number(v float64) *hubv1.SessionSummaryValue {
	return &hubv1.SessionSummaryValue{Kind: &hubv1.SessionSummaryValue_NumberValue{NumberValue: v}}
}

func seedRuns(ctx context.Context, s *store.Store, rng *rand.Rand, now time.Time, days int) (int, error) {
	count := 0
	for pi, p := range players {
		// Each player improves a little over the window, so trends have a shape.
		for d := days; d >= 0; d-- {
			if rng.Float64() > 0.55+0.04*float64(p.activity) {
				continue
			}
			day := now.AddDate(0, 0, -d)
			start := time.Date(day.Year(), day.Month(), day.Day(), 16+rng.Intn(5), rng.Intn(60), 0, 0, time.UTC)
			progress := 1 - float64(d)/float64(days+1)
			runs := 1 + rng.Intn(p.activity)
			for r := 0; r < runs; r++ {
				sc := pickScenario(rng, p)
				played := start.Add(time.Duration(r) * (sc.duration + 40*time.Second))
				if played.After(now) {
					continue
				}
				run, err := buildRun(rng, p, pi, sc, played, progress, count)
				if err != nil {
					return count, err
				}
				if err := s.SaveIngestedRun(ctx, run, nil); err != nil {
					return count, fmt.Errorf("save run for %s: %w", p.name, err)
				}
				count++
			}
		}
	}
	return count, nil
}

func pickScenario(rng *rand.Rand, p player) scenario {
	if rng.Float64() < 0.6 {
		var preferred []scenario
		for _, sc := range scenarios {
			if sc.kind == p.focus {
				preferred = append(preferred, sc)
			}
		}
		return preferred[rng.Intn(len(preferred))]
	}
	return scenarios[rng.Intn(len(scenarios))]
}

func buildRun(rng *rand.Rand, p player, pi int, sc scenario, played time.Time, progress float64, ordinal int) (store.IngestedRun, error) {
	skill := p.skill * (0.92 + 0.1*progress) * (0.94 + 0.12*rng.Float64())
	score := math.Round(sc.base * skill)
	accuracy := math.Min(99.5, sc.accuracy*(0.9+0.12*skill/1.1)+rng.NormFloat64()*1.5)
	seconds := int(sc.duration / time.Second)
	shotsPerSecond := 2.2 + rng.Float64()
	if sc.kind == "Tracking" {
		shotsPerSecond = 15
	}
	shots := uint32(float64(seconds) * shotsPerSecond)
	hits := uint32(float64(shots) * accuracy / 100)
	kills := uint32(0)
	if sc.kind != "Tracking" {
		kills = hits
	}

	summary := map[string]*hubv1.SessionSummaryValue{
		"shotsFired":         number(float64(shots)),
		"shotsHit":           number(float64(hits)),
		"kills":              number(float64(kills)),
		"accuracyPct":        number(accuracy),
		"scorePerMinute":     number(score / (float64(seconds) / 60)),
		"avgScorePerMinute":  number(score / (float64(seconds) / 60)),
		"peakScorePerMinute": number(score / (float64(seconds) / 60) * 1.18),
		"killsPerSecond":     number(float64(kills) / float64(seconds)),
		"avgKillsPerSecond":  number(float64(kills) / float64(seconds)),
		"peakKillsPerSecond": number(float64(kills) / float64(seconds) * 1.3),
	}
	if sc.kind == "Tracking" {
		possible := float64(seconds) * 100
		done := possible * accuracy / 100
		summary["damageDone"] = number(done)
		summary["damagePossible"] = number(possible)
		summary["damageEfficiency"] = number(accuracy)
		summary["smoothnessComposite"] = number(55 + 30*skill/1.15 + rng.Float64()*5)
		summary["smoothnessJitter"] = number(0.2 + rng.Float64()*0.3)
		summary["smoothnessPathEfficiency"] = number(0.7 + 0.2*skill/1.15)
	} else {
		summary["avgFireToHitMs"] = number(260 - 60*skill + rng.Float64()*30)
		summary["avgShotsToHit"] = number(1 + (100-accuracy)/100)
		summary["panelAvgTtkMs"] = number(420 - 120*skill + rng.Float64()*40)
		summary["panelBestTtkMs"] = number(220 - 60*skill)
	}

	var timeline []store.TimelineSecond
	if ordinal%3 == 0 {
		for t := 1; t <= seconds; t++ {
			frac := float64(t) / float64(seconds)
			fatigue := 1 - 0.06*frac*frac
			wobble := 1 + rng.NormFloat64()*0.04
			timeline = append(timeline, store.TimelineSecond{
				Second:    uint32(t),
				Score:     math.Round(score * frac),
				Accuracy:  math.Max(0, math.Min(100, accuracy*fatigue*wobble)),
				DamageEff: math.Max(0, math.Min(100, accuracy*fatigue*wobble)),
				SPM:       score / (float64(seconds) / 60) * fatigue * wobble,
				Shots:     uint32(float64(shots) * frac),
				Hits:      uint32(float64(hits) * frac),
				Kills:     uint32(float64(kills) * frac),
			})
		}
	}

	summaryJSON, err := store.SummaryMapToJSON(summary)
	if err != nil {
		return store.IngestedRun{}, err
	}
	featureJSON, err := store.SummaryMapToJSON(map[string]*hubv1.SessionSummaryValue{})
	if err != nil {
		return store.IngestedRun{}, err
	}
	return store.IngestedRun{
		AppVersion:      "devseed",
		SchemaVersion:   1,
		UserExternalID:  p.externalID,
		UserDisplayName: p.name,
		SessionID:       fmt.Sprintf("devseed-%02d-%s", pi, played.Format("20060102T150405")),
		ScenarioName:    sc.name,
		ScenarioType:    sc.kind,
		Score:           score,
		Accuracy:        accuracy,
		DurationMS:      uint64(sc.duration / time.Millisecond),
		PlayedAt:        played,
		SummaryJSON:     summaryJSON,
		FeatureJSON:     featureJSON,
		Timeline:        timeline,
	}, nil
}

func userRef(ctx context.Context, s *store.Store, p player) (tournament.UserRef, error) {
	handle := handleFor(p)
	u, err := s.TournamentUserByHandle(ctx, handle)
	if err != nil {
		return tournament.UserRef{}, fmt.Errorf("look up %s: %w", handle, err)
	}
	return tournament.UserRef{UserID: u.UserID, ExternalID: u.ExternalID, Handle: u.Handle, DisplayName: u.DisplayName}, nil
}

// handleFor is the synthetic player's linked username, which the Hub uses as the handle.
func handleFor(p player) string {
	return strings.ToLower(strings.ReplaceAll(p.name, " ", "-"))
}

func seedLive(ctx context.Context, s *store.Store, rng *rand.Rand) error {
	states := []struct {
		player   int
		scenario int
		state    string
		elapsed  float64
	}{
		{0, 0, "Playing", 22},
		{1, 3, "Playing", 41},
		{3, 1, "Playing", 9},
		{5, 6, "In menus", 0},
	}
	for _, st := range states {
		ref, err := userRef(ctx, s, players[st.player])
		if err != nil {
			return err
		}
		sc := scenarios[st.scenario]
		payload := store.LiveActivityPayload{GameStateCode: 1, GameState: st.state, RuntimeLoaded: true, BridgeConnected: true}
		if st.state == "Playing" {
			frac := st.elapsed / sc.duration.Seconds()
			score := math.Round(sc.base * players[st.player].skill * frac)
			acc := sc.accuracy * (0.95 + rng.Float64()*0.06)
			remaining := sc.duration.Seconds() - st.elapsed
			kills := uint32(score / 100)
			payload.ScenarioName = sc.name
			payload.ScenarioType = sc.kind
			payload.Score = &score
			payload.AccuracyPct = &acc
			payload.Kills = &kills
			payload.TimeRemainingSecs = &remaining
		}
		if err := s.UpsertLiveActivity(ctx, ref.UserID, payload); err != nil {
			return err
		}
	}
	return nil
}

func seedTournaments(ctx context.Context, s *store.Store, now time.Time) error {
	refs := make([]tournament.UserRef, len(players))
	for i, p := range players {
		ref, err := userRef(ctx, s, p)
		if err != nil {
			return err
		}
		refs[i] = ref
	}
	org := tournament.Actor{UserRef: refs[0], Verified: true}
	pool := []tournament.PoolScenario{{Name: scenarios[0].name}, {Name: scenarios[3].name}, {Name: scenarios[6].name}}
	starts := now.Add(72 * time.Hour)

	specs := []struct {
		id      string
		spec    tournament.Spec
		players int
		start   bool
	}{
		{"t_devseed_open", tournament.Spec{
			Name: "Synthetic Weekly Cup", Description: "Best of three across tracking, clicking and switching.",
			Format: bracket.SingleElimination, Seeding: tournament.SeedRandom, Scheduling: tournament.Scheduled, StartsAt: &starts,
			MaxEntrants: 16, Ruleset: tournament.Ruleset{BestOf: 3, Pool: pool},
		}, 5, false},
		{"t_devseed_live", tournament.Spec{
			Name: "Synthetic Tracking Ladder", Description: "Single scenario, best of one.",
			Format: bracket.SingleElimination, Seeding: tournament.SeedManual,
			MaxEntrants: 8, Ruleset: tournament.Ruleset{BestOf: 1, Pool: pool[:1]},
		}, 8, true},
	}
	for _, item := range specs {
		if existing, err := s.LoadTournament(ctx, item.id); err == nil && existing != nil {
			continue
		}
		t, err := tournament.New(item.id, item.spec, org, now)
		if err != nil {
			return err
		}
		if err := s.CreateTournament(ctx, t); err != nil {
			return err
		}
		if err := t.Advance(org, tournament.OpenRegistration, nil, 0, now); err != nil {
			return err
		}
		for _, ref := range refs[:item.players] {
			if _, err := t.Register(tournament.Actor{UserRef: ref, Verified: true}, now); err != nil {
				return err
			}
		}
		if item.start {
			if err := t.Advance(org, tournament.Start, nil, 1, now); err != nil {
				return err
			}
		}
		if err := s.SaveTournament(ctx, t, now); err != nil {
			return err
		}
	}
	log.Printf("stored %d synthetic tournaments", len(specs))
	return nil
}
