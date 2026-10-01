package tournament

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/tournament/bracket"
)

var t0 = time.Date(2026, 5, 1, 18, 0, 0, 0, time.UTC)

func actor(n int) Actor {
	return Actor{UserRef: UserRef{UserID: int64(n), ExternalID: fmt.Sprintf("ext-%d", n), Handle: fmt.Sprintf("player%d", n), DisplayName: fmt.Sprintf("Player %d", n)}}
}

var organiser = Actor{UserRef: UserRef{UserID: 1000, Handle: "organiser", DisplayName: "Organiser"}}

func baseSpec() Spec {
	return Spec{
		Name:    "Synthetic Cup",
		Format:  bracket.SingleElimination,
		Seeding: SeedManual,
		Ruleset: Ruleset{BestOf: 3, Pool: []PoolScenario{{Name: "Alpha"}, {Name: "Bravo"}, {Name: "Charlie"}}},
	}
}

func seq() func() uint64 {
	n := uint64(0)
	return func() uint64 { n++; return 1000 + n }
}

func newOpen(t *testing.T, spec Spec, players int) *Tournament {
	t.Helper()
	randomSeed = seq()
	tr, err := New("t1", spec, organiser, t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Advance(organiser, OpenRegistration, nil, 0, t0); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= players; i++ {
		if _, err := tr.Register(actor(i), t0.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	return tr
}

func started(t *testing.T, spec Spec, players int) *Tournament {
	t.Helper()
	tr := newOpen(t, spec, players)
	if err := tr.Advance(organiser, Start, nil, 7, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	return tr
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func wantKind(t *testing.T, err error, k Kind) {
	t.Helper()
	if KindOf(err) != k {
		t.Fatalf("got %v (kind %d), want kind %d", err, KindOf(err), k)
	}
}

// player returns the actor playing in a match slot.
func player(tr *Tournament, m *bracket.Match, slot int) Actor {
	e := tr.Entrant(m.Slots[slot].Entrant)
	return Actor{UserRef: e.User}
}

func TestSpecValidation(t *testing.T) {
	bad := []func(*Spec){
		func(s *Spec) { s.Name = "x" },
		func(s *Spec) { s.Format = "ladder" },
		func(s *Spec) { s.MaxEntrants = 1 },
		func(s *Spec) { s.MaxEntrants = 999 },
		func(s *Spec) { s.Ruleset.BestOf = 2 },
		func(s *Spec) { s.Ruleset.Pool = nil },
		func(s *Spec) { s.Ruleset.Pool = append(s.Ruleset.Pool, PoolScenario{Name: "alpha"}) },
		func(s *Spec) { s.Ruleset.Pool[0].TimeLimit = 5 },
		func(s *Spec) { s.Ruleset.Pool[0].Hash = "xyz" },
		func(s *Spec) { s.Ruleset.GameMode = "duel" },
		func(s *Spec) { s.Seeding = SeedScenario },
		func(s *Spec) { s.Scheduling = Scheduled },
		func(s *Spec) { at := t0; s.CheckInOpens = &at },
		func(s *Spec) { a, b := t0, t0.Add(-time.Minute); s.CheckInOpens, s.CheckInCloses = &a, &b },
		func(s *Spec) { s.Ruleset.Veto = []VetoStep{{Ban, HigherSeed}, {Ban, LowerSeed}, {Ban, HigherSeed}} },
		func(s *Spec) { s.Ruleset.Veto = []VetoStep{{Ban, HigherSeed}} }, // bo3 needs 3 left
		func(s *Spec) { s.Ruleset.Veto = []VetoStep{{"swap", HigherSeed}} },
		func(s *Spec) { s.Format = bracket.RoundRobin; s.MaxEntrants = 64 },
	}
	for i, mutate := range bad {
		s := baseSpec()
		s.Ruleset.Pool = append([]PoolScenario(nil), s.Ruleset.Pool...)
		mutate(&s)
		if _, err := s.Normalize(); KindOf(err) != KindInvalid {
			t.Fatalf("case %d accepted: %v", i, err)
		}
	}
	s, err := baseSpec().Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if s.MaxEntrants != 32 || s.Scheduling != ReadyWhenOnline || s.Ruleset.Countdown != 5 || s.Ruleset.ConfirmMinutes != 15 || s.Ruleset.GameMode != "score-race" {
		t.Fatalf("defaults %+v", s)
	}
	if _, err := New("t", baseSpec(), Actor{}, t0); err != ErrSignIn {
		t.Fatal("anonymous create")
	}
}

func TestDefaultVeto(t *testing.T) {
	steps := DefaultVeto(5, 3)
	want := []VetoStep{{Ban, HigherSeed}, {Ban, LowerSeed}, {Pick, HigherSeed}, {Pick, LowerSeed}}
	if fmt.Sprint(steps) != fmt.Sprint(want) {
		t.Fatalf("veto %v", steps)
	}
	if len(DefaultVeto(1, 1)) != 0 {
		t.Fatal("bo1 single pool has a veto")
	}
}

func TestRegistrationRules(t *testing.T) {
	tr, _ := New("t1", baseSpec(), organiser, t0)
	_, err := tr.Register(actor(1), t0)
	wantKind(t, err, KindState) // draft
	must(t, tr.Advance(organiser, OpenRegistration, nil, 0, t0))
	_, err = tr.Register(Actor{}, t0)
	if err != ErrSignIn {
		t.Fatal("anonymous registration")
	}
	e1, err := tr.Register(actor(1), t0)
	must(t, err)
	again, _ := tr.Register(actor(1), t0)
	if again != e1 || len(tr.Entrants) != 1 {
		t.Fatal("double registration")
	}
	wantKind(t, tr.Advance(actor(1), OpenCheckIn, nil, 0, t0), KindForbidden)
	// Caps.
	spec := baseSpec()
	spec.MaxEntrants = 2
	full := newOpen(t, spec, 2)
	_, err = full.Register(actor(9), t0)
	wantKind(t, err, KindState)
	// Withdrawing frees the slot and can re-register.
	must(t, full.Withdraw(actor(1), t0))
	_, err = full.Register(actor(9), t0)
	must(t, err)
	_, err = full.Register(actor(1), t0)
	wantKind(t, err, KindState)
}

func TestInviteOnly(t *testing.T) {
	spec := baseSpec()
	spec.InviteOnly = true
	tr := newOpen(t, spec, 0)
	_, err := tr.Register(actor(1), t0)
	wantKind(t, err, KindForbidden)
	wantKind(t, tr.SetInvites(actor(1), []string{"player1"}, false, t0), KindForbidden)
	must(t, tr.SetInvites(organiser, []string{"Player1", " player2 "}, false, t0))
	if strings.Join(tr.Invites, ",") != "player1,player2" {
		t.Fatalf("invites %v", tr.Invites)
	}
	_, err = tr.Register(actor(1), t0)
	must(t, err)
	must(t, tr.SetInvites(organiser, []string{"player2"}, true, t0))
	_, err = tr.Register(actor(2), t0)
	wantKind(t, err, KindForbidden)
	wantKind(t, tr.SetInvites(organiser, []string{"has space"}, false, t0), KindInvalid)
}

func TestCheckInWindowAndNoShowsAtStart(t *testing.T) {
	spec := baseSpec()
	opens, closes := t0.Add(time.Hour), t0.Add(2*time.Hour)
	spec.CheckInOpens, spec.CheckInCloses = &opens, &closes
	tr := newOpen(t, spec, 4)
	_, err := tr.CheckIn(actor(1), t0)
	wantKind(t, err, KindState)
	if tr.Tick(t0.Add(30*time.Minute)) || tr.Status != Registration {
		t.Fatal("check-in opened early")
	}
	if !tr.Tick(opens) || tr.Status != CheckIn {
		t.Fatal("check-in did not open on time")
	}
	for _, p := range []int{1, 2, 4} {
		_, err := tr.CheckIn(actor(p), opens.Add(time.Minute))
		must(t, err)
	}
	_, err = tr.CheckIn(actor(3), closes)
	wantKind(t, err, KindState)
	_, err = tr.CheckIn(actor(9), opens)
	wantKind(t, err, KindNotFound)
	must(t, tr.Advance(organiser, Start, nil, 0, closes))
	if tr.Entrant("e3").Status != EntrantNoShow || len(tr.Bracket.Entrants) != 3 {
		t.Fatalf("no-show still in the field: %v", tr.Bracket.Entrants)
	}
	// Manual seeding without seeds: registration order.
	if strings.Join(tr.Bracket.Entrants, ",") != "e1,e2,e4" {
		t.Fatalf("seed order %v", tr.Bracket.Entrants)
	}
}

func TestStartNeedsTwo(t *testing.T) {
	tr := newOpen(t, baseSpec(), 1)
	wantKind(t, tr.Advance(organiser, Start, nil, 0, t0), KindState)
}

func TestSeedingMethods(t *testing.T) {
	spec := baseSpec()
	spec.Seeding = SeedRandom
	a := started(t, spec, 8)
	b := started(t, spec, 8)
	if strings.Join(a.Bracket.Entrants, ",") != strings.Join(b.Bracket.Entrants, ",") || a.RandomSeed != 7 {
		t.Fatal("random seeding is not reproducible from the recorded draw")
	}
	if strings.Join(a.Bracket.Entrants, ",") == "e1,e2,e3,e4,e5,e6,e7,e8" {
		t.Fatal("random seeding kept registration order")
	}
	spec.Seeding, spec.SeedingSource = SeedScenario, "Alpha"
	tr := newOpen(t, spec, 4)
	ratings := map[int64]Rating{2: {Value: 900, Label: "900"}, 4: {Value: 1200, Label: "1,200"}, 1: {Value: 300}}
	must(t, tr.Advance(organiser, Start, ratings, 0, t0))
	if strings.Join(tr.Bracket.Entrants, ",") != "e4,e2,e1,e3" {
		t.Fatalf("stat seeding %v", tr.Bracket.Entrants)
	}
	if tr.Entrant("e4").RatingLabel != "1,200" || tr.Entrant("e3").Rating != nil {
		t.Fatal("ratings not recorded")
	}
}

func TestManualSeedsAndReseedRules(t *testing.T) {
	tr := newOpen(t, baseSpec(), 4)
	must(t, tr.SetSeeds(organiser, []string{"e3", "e1"}, t0))
	order := func() string {
		var ids []string
		for _, e := range tr.seedable() {
			ids = append(ids, e.ID)
		}
		return strings.Join(ids, ",")
	}
	if order() != "e3,e1,e2,e4" {
		t.Fatalf("manual seeds %s", order())
	}
	wantKind(t, tr.SetSeeds(organiser, []string{"e1", "e1"}, t0), KindInvalid)
	wantKind(t, tr.SetSeeds(organiser, []string{"e99"}, t0), KindInvalid)
	must(t, tr.Advance(organiser, Start, nil, 0, t0))
	if strings.Join(tr.Bracket.Entrants, ",") != "e3,e1,e2,e4" {
		t.Fatalf("bracket ignores manual seeds: %v", tr.Bracket.Entrants)
	}
	// Reseeding after the start regenerates the bracket until a match starts.
	must(t, tr.SetSeeds(organiser, []string{"e4"}, t0))
	if tr.Bracket.Entrants[0] != "e4" {
		t.Fatal("reseed did not regenerate the bracket")
	}
	m := tr.Bracket.Match("W1-1")
	must(t, tr.MarkReady(player(tr, m, 0), "W1-1", true, true, t0))
	must(t, tr.MarkReady(player(tr, m, 1), "W1-1", true, true, t0))
	wantKind(t, tr.SetSeeds(organiser, []string{"e1"}, t0), KindState)
}

// playMatch readies both players and plays every game with the given scores.
func playMatch(t *testing.T, tr *Tournament, id string, scores ...[2]float64) {
	t.Helper()
	m := tr.Bracket.Match(id)
	must(t, tr.MarkReady(player(tr, m, 0), id, true, true, t0))
	must(t, tr.MarkReady(player(tr, m, 1), id, true, true, t0))
	for turn, _ := tr.VetoTurn(m); turn != ""; turn, _ = tr.VetoTurn(m) {
		e := tr.Entrant(turn)
		must(t, tr.Veto(Actor{UserRef: e.User}, id, tr.available(tr.Series[id])[0].Name, t0))
	}
	s := tr.Series[id]
	host := Actor{UserRef: tr.Entrant(s.Host).User}
	for _, sc := range scores {
		g := s.Current()
		must(t, tr.ReportGame(host, id, GameReport{Index: g.Index, Scores: sc, HostValidated: true, Seed: strconv.FormatUint(g.Seed, 10)}, t0))
	}
}

func confirm(t *testing.T, tr *Tournament, id string) {
	t.Helper()
	m := tr.Bracket.Match(id)
	s := tr.Series[id]
	other := 0
	if m.Slots[0].Entrant == s.ReportedBy {
		other = 1
	}
	must(t, tr.Confirm(player(tr, m, other), id, t0))
}

func TestFullSingleEliminationFlow(t *testing.T) {
	tr := started(t, baseSpec(), 4)
	if tr.Status != InProgress || len(tr.Series) != 2 {
		t.Fatalf("status %s series %d", tr.Status, len(tr.Series))
	}
	m := tr.Bracket.Match("W1-1") // e1 v e4
	if tr.State(m) != StateReady {
		t.Fatal(tr.State(m))
	}
	// Only players act on their match.
	wantKind(t, tr.MarkReady(actor(2), "W1-1", true, true, t0), KindForbidden)
	must(t, tr.MarkReady(actor(4), "W1-1", true, true, t0))
	if tr.Series["W1-1"].StartedAt != nil {
		t.Fatal("started with one player ready")
	}
	must(t, tr.MarkReady(actor(1), "W1-1", true, false, t0))
	s := tr.Series["W1-1"]
	// e1 is the higher seed but can't host; e4 can.
	if s.Host != "e4" || tr.State(m) != StateLive || len(s.Games) != 1 {
		t.Fatalf("host %s state %s", s.Host, tr.State(m))
	}
	if s.Games[0].Scenario != "Alpha" || s.Games[0].Seed == 0 {
		t.Fatalf("game 1 %+v", s.Games[0])
	}
	// Wrong game index.
	wantKind(t, tr.ReportGame(actor(4), "W1-1", GameReport{Index: 1, Scores: [2]float64{1, 2}}, t0), KindInvalid)
	wantKind(t, tr.ReportGame(actor(4), "W1-1", GameReport{Index: 0, Scores: [2]float64{-1, 2}}, t0), KindInvalid)
	must(t, tr.ReportGame(actor(4), "W1-1", GameReport{Index: 0, Scores: [2]float64{900, 1000}, HostValidated: true}, t0))
	if len(s.Games) != 2 || s.Games[1].Scenario != "Bravo" || s.Games[1].Seed == s.Games[0].Seed {
		t.Fatalf("game 2 %+v", s.Games[1])
	}
	if !s.Games[0].HostValidated {
		t.Fatal("host validation not recorded")
	}
	// The non-host can report too, but not validate.
	must(t, tr.ReportGame(actor(1), "W1-1", GameReport{Index: 1, Scores: [2]float64{800, 1100}, HostValidated: true}, t0))
	if s.Games[1].HostValidated {
		t.Fatal("non-host validated a game")
	}
	if tr.State(m) != StateAwaiting || s.ReportedBy != "e1" {
		t.Fatalf("after 2-0: %s by %s", tr.State(m), s.ReportedBy)
	}
	wantKind(t, tr.ReportGame(actor(4), "W1-1", GameReport{Index: 2}, t0), KindState)
	// The reporter can't confirm their own report.
	wantKind(t, tr.Confirm(actor(1), "W1-1", t0), KindState)
	must(t, tr.Confirm(actor(4), "W1-1", t0))
	if m.State != bracket.Complete || m.WinnerID() != "e4" || m.Score != [2]int{0, 2} {
		t.Fatalf("bracket result %+v", m)
	}
	if tr.Bracket.Match("W2-1").Slots[0].Entrant != "e4" {
		t.Fatal("winner did not advance")
	}
	playMatch(t, tr, "W1-2", [2]float64{10, 5}, [2]float64{10, 5})
	confirm(t, tr, "W1-2")
	if tr.Series["W2-1"] == nil || tr.State(tr.Bracket.Match("W2-1")) != StateReady {
		t.Fatal("final not open")
	}
	playMatch(t, tr, "W2-1", [2]float64{1, 2}, [2]float64{3, 2}, [2]float64{5, 4})
	confirm(t, tr, "W2-1")
	tr.Tick(t0)
	if tr.Status != Completed || tr.Bracket.Champion() != "e4" {
		t.Fatalf("status %s champion %s", tr.Status, tr.Bracket.Champion())
	}
	if tr.Entrant("e1").Status != EntrantEliminated || tr.Entrant("e4").Status != EntrantActive {
		t.Fatal("entrant statuses")
	}
}

func TestTiedGameIsReplayedOnTheSameScenario(t *testing.T) {
	tr := started(t, baseSpec(), 2)
	playMatch(t, tr, "W1-1", [2]float64{50, 50})
	s := tr.Series["W1-1"]
	if len(s.Games) != 2 || s.Games[1].Scenario != s.Games[0].Scenario || s.Games[1].Seed == s.Games[0].Seed {
		t.Fatalf("tie replay %+v", s.Games)
	}
	if s.Wins() != [2]int{} {
		t.Fatal("tie counted")
	}
}

func TestResultRecordSetsTheScores(t *testing.T) {
	spec := baseSpec()
	spec.Ruleset.BestOf = 1
	tr := started(t, spec, 2)
	m := tr.Bracket.Match("W1-1")
	must(t, tr.MarkReady(actor(1), "W1-1", true, true, t0))
	must(t, tr.MarkReady(actor(2), "W1-1", true, true, t0))
	g := tr.Series["W1-1"].Games[0]
	seed := strconv.FormatUint(g.Seed, 10)
	bad := &ResultRecord{Format: 1, Mode: "score-race", Players: []ResultPlayer{{Key: "k1", Entrant: "e1", Score: 10}, {Key: "k9", Entrant: "e9", Score: 5}}}
	wantKind(t, tr.ReportGame(actor(1), "W1-1", GameReport{Index: 0, Result: bad}, t0), KindInvalid)
	wantKind(t, tr.ReportGame(actor(1), "W1-1", GameReport{Index: 0, Result: &ResultRecord{Format: 2}}, t0), KindInvalid)
	twice := &ResultRecord{Format: 1, Players: []ResultPlayer{{Entrant: "e1"}, {Entrant: "e1"}}}
	wantKind(t, tr.ReportGame(actor(1), "W1-1", GameReport{Index: 0, Result: twice}, t0), KindInvalid)
	rec := &ResultRecord{Format: 1, Match: "m-synthetic", Mode: "score-race", Seed: seed, StartedAt: 1, EndedAt: 2, Host: "k2",
		Players: []ResultPlayer{{Key: "k2", Entrant: m.Slots[1].Entrant, Place: 2, Score: 800, Replay: "ABC"}, {Key: "k1", Entrant: m.Slots[0].Entrant, Place: 1, Score: 900}}, Winner: "k1"}
	must(t, tr.ReportGame(actor(1), "W1-1", GameReport{Index: 0, Result: rec}, t0))
	if g.Scores != [2]float64{900, 800} || g.Winner != 0 || g.Result == nil || g.Result.Players[0].Replay != "abc" || contains(g.Flags, "seed-mismatch") {
		t.Fatalf("game %+v", g)
	}
}

func TestVetoOrderAndTurns(t *testing.T) {
	spec := baseSpec()
	spec.Ruleset.Pool = []PoolScenario{{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"}, {Name: "E"}}
	spec.Ruleset.Veto = DefaultVeto(5, 3) // ban H, ban L, pick H, pick L, decider
	tr := started(t, spec, 2)
	m := tr.Bracket.Match("W1-1") // e1 (higher) v e2
	must(t, tr.MarkReady(actor(1), "W1-1", true, true, t0))
	if turn, _ := tr.VetoTurn(m); turn != "" {
		t.Fatal("veto before both are ready")
	}
	must(t, tr.MarkReady(actor(2), "W1-1", true, true, t0))
	if tr.State(m) != StateVeto {
		t.Fatal(tr.State(m))
	}
	wantKind(t, tr.Veto(actor(2), "W1-1", "A", t0), KindState) // not their turn
	must(t, tr.Veto(actor(1), "W1-1", "a", t0))                // ban A (case-insensitive)
	wantKind(t, tr.Veto(actor(2), "W1-1", "A", t0), KindInvalid)
	must(t, tr.Veto(actor(2), "W1-1", "B", t0))
	if _, action := tr.VetoTurn(m); action != Pick {
		t.Fatal("expected a pick")
	}
	wantKind(t, tr.MarkReady(actor(1), "W1-1", false, true, t0), KindState)
	must(t, tr.Veto(actor(1), "W1-1", "E", t0))
	must(t, tr.Veto(actor(2), "W1-1", "C", t0))
	s := tr.Series["W1-1"]
	if s.StartedAt == nil || len(s.Veto) != 5 || s.Veto[4].Action != Decider || s.Veto[4].Scenario != "D" {
		t.Fatalf("veto log %+v", s.Veto)
	}
	if got := []string{tr.order(s)[0].Name, tr.order(s)[1].Name, tr.order(s)[2].Name}; strings.Join(got, "") != "ECD" {
		t.Fatalf("game order %v", got)
	}
	if s.Games[0].Scenario != "E" {
		t.Fatal("game 1 is not the first pick")
	}
}

func TestRoundBestOfOverridesFinal(t *testing.T) {
	spec := baseSpec()
	spec.Ruleset.BestOf = 1
	spec.Ruleset.RoundBestOf = []RoundBestOf{{Side: bracket.Winners, FromRound: 0, BestOf: 3}}
	tr := started(t, spec, 4)
	if tr.BestOf(tr.Bracket.Match("W1-1")) != 1 || tr.BestOf(tr.Bracket.Match("W2-1")) != 3 {
		t.Fatal("final best-of")
	}
}

func TestNoShowForfeitAndBothAbsent(t *testing.T) {
	tr := started(t, baseSpec(), 4)
	must(t, tr.MarkReady(actor(1), "W1-1", true, true, t0))
	deadline := *tr.Series["W1-1"].Deadline
	if tr.Tick(deadline.Add(-time.Second)) && tr.Bracket.Match("W1-1").State == bracket.Complete {
		t.Fatal("forfeit before the deadline")
	}
	tr.Tick(deadline)
	m := tr.Bracket.Match("W1-1")
	if m.State != bracket.Complete || m.WinnerID() != "e1" || m.Resolution != bracket.Forfeit || tr.Series["W1-1"].Resolution != "no_show" {
		t.Fatalf("no-show: %+v %+v", m, tr.Series["W1-1"])
	}
	tr.Tick(deadline)
	s2 := tr.Series["W1-2"]
	if tr.Bracket.Match("W1-2").State != bracket.Ready || !contains(s2.Flags, "no-show-both") {
		t.Fatalf("nobody showed: %+v", s2)
	}
}

func TestScheduledDeadline(t *testing.T) {
	spec := baseSpec()
	spec.Scheduling = Scheduled
	at := t0.Add(3 * time.Hour)
	spec.StartsAt = &at
	tr := started(t, spec, 2)
	s := tr.Series["W1-1"]
	if s.ScheduledAt == nil || !s.ScheduledAt.Equal(at) || !s.Deadline.Equal(at.Add(10*time.Minute)) {
		t.Fatalf("schedule %+v", s)
	}
}

func TestAutoConfirmAndBlockedResults(t *testing.T) {
	tr := started(t, baseSpec(), 4)
	playMatch(t, tr, "W1-1", [2]float64{2, 1}, [2]float64{2, 1})
	s := tr.Series["W1-1"]
	tr.Tick(s.Deadline.Add(-time.Second))
	if tr.Bracket.Match("W1-1").State == bracket.Complete {
		t.Fatal("accepted before the confirm window ended")
	}
	tr.Tick(*s.Deadline)
	if tr.Bracket.Match("W1-1").State != bracket.Complete || !contains(s.Flags, "auto-confirmed") {
		t.Fatal("not auto-confirmed")
	}
	// A seed mismatch blocks auto-confirmation.
	m := tr.Bracket.Match("W1-2")
	must(t, tr.MarkReady(player(tr, m, 0), "W1-2", true, true, t0))
	must(t, tr.MarkReady(player(tr, m, 1), "W1-2", true, true, t0))
	host := Actor{UserRef: tr.Entrant(tr.Series["W1-2"].Host).User}
	must(t, tr.ReportGame(host, "W1-2", GameReport{Index: 0, Scores: [2]float64{2, 1}, Seed: "12345"}, t0))
	must(t, tr.ReportGame(host, "W1-2", GameReport{Index: 1, Scores: [2]float64{2, 1}}, t0))
	if !contains(tr.Series["W1-2"].Games[0].Flags, "seed-mismatch") {
		t.Fatal("seed mismatch not flagged")
	}
	tr.Tick(t0.Add(time.Hour))
	if tr.Bracket.Match("W1-2").State == bracket.Complete || !contains(tr.Series["W1-2"].Flags, "needs-review") {
		t.Fatal("blocked result was auto-confirmed")
	}
	// The opponent can still accept it explicitly.
	confirm(t, tr, "W1-2")
	if tr.Bracket.Match("W1-2").State != bracket.Complete {
		t.Fatal("explicit confirm")
	}
}

func TestRequireReplaysBlocksAutoConfirm(t *testing.T) {
	spec := baseSpec()
	spec.Ruleset.BestOf = 1
	spec.Ruleset.RequireReplays = true
	tr := started(t, spec, 2)
	playMatch(t, tr, "W1-1", [2]float64{2, 1})
	tr.Tick(t0.Add(time.Hour))
	if tr.Bracket.Match("W1-1").State == bracket.Complete {
		t.Fatal("accepted without replays")
	}
}

func TestDisputes(t *testing.T) {
	for _, decision := range []DisputeDecision{DecisionUphold, DecisionOverturn, DecisionReplay} {
		tr := started(t, baseSpec(), 2)
		playMatch(t, tr, "W1-1", [2]float64{2, 1}, [2]float64{2, 1})
		m := tr.Bracket.Match("W1-1")
		_, err := tr.OpenDispute(actor(5), "W1-1", "not my match", t0)
		wantKind(t, err, KindForbidden)
		_, err = tr.OpenDispute(actor(2), "W1-1", "no", t0)
		wantKind(t, err, KindInvalid)
		d, err := tr.OpenDispute(actor(2), "W1-1", "Their score jumped at the end.", t0)
		must(t, err)
		if tr.State(m) != StateDisputed {
			t.Fatal(tr.State(m))
		}
		wantKind(t, tr.Confirm(actor(2), "W1-1", t0), KindState)
		tr.Tick(t0.Add(48 * time.Hour))
		if m.State == bracket.Complete {
			t.Fatal("disputed match auto-confirmed")
		}
		_, err = tr.ResolveDispute(actor(2), d.ID, Decision{Decision: decision}, t0)
		wantKind(t, err, KindForbidden)
		dec := Decision{Decision: decision, Winner: "e2", Wins: [2]int{1, 2}, Note: "Replay showed a desync."}
		_, err = tr.ResolveDispute(organiser, d.ID, dec, t0)
		must(t, err)
		if d.Status != DisputeResolved || d.ResolvedBy == nil {
			t.Fatal("dispute not resolved")
		}
		switch decision {
		case DecisionUphold:
			if m.WinnerID() != "e1" {
				t.Fatal("uphold")
			}
		case DecisionOverturn:
			if m.WinnerID() != "e2" || m.Resolution != bracket.Override {
				t.Fatal("overturn")
			}
		case DecisionReplay:
			s := tr.Series["W1-1"]
			if m.State != bracket.Ready || s.StartedAt != nil || len(s.Games) != 0 || !contains(s.Flags, "replayed") {
				t.Fatalf("replay %+v", s)
			}
		}
		_, err = tr.ResolveDispute(organiser, d.ID, dec, t0)
		wantKind(t, err, KindState)
	}
}

func TestOrganiserOverrideResetsLaterMatches(t *testing.T) {
	tr := started(t, baseSpec(), 4)
	playMatch(t, tr, "W1-1", [2]float64{2, 1}, [2]float64{2, 1})
	confirm(t, tr, "W1-1")
	playMatch(t, tr, "W1-2", [2]float64{2, 1}, [2]float64{2, 1})
	confirm(t, tr, "W1-2")
	m := tr.Bracket.Match("W2-1")
	must(t, tr.MarkReady(player(tr, m, 0), "W2-1", true, true, t0))
	_, err := tr.SetResult(actor(1), "W1-1", "e4", [2]int{0, 2}, "override", "", t0)
	wantKind(t, err, KindForbidden)
	_, err = tr.SetResult(organiser, "W1-1", "e2", [2]int{0, 2}, "override", "", t0)
	wantKind(t, err, KindInvalid) // e2 isn't in W1-1
	reset, err := tr.SetResult(organiser, "W1-1", "e4", [2]int{0, 2}, "override", "Replay check", t0)
	must(t, err)
	if fmt.Sprint(reset) != "[]" && fmt.Sprint(reset) != "[W2-1]" {
		t.Fatalf("reset %v", reset)
	}
	final := tr.Bracket.Match("W2-1")
	if final.Slots[0].Entrant != "e4" || tr.Series["W2-1"] == nil || len(tr.Series["W2-1"].ReadyAt) != 0 {
		t.Fatalf("final after override: %+v %+v", final, tr.Series["W2-1"])
	}
	// Forfeit a ready match.
	_, err = tr.SetResult(organiser, "W2-1", "e2", [2]int{}, "forfeit", "e4 left", t0)
	must(t, err)
	tr.Tick(t0)
	if tr.Status != Completed || tr.Bracket.Champion() != "e2" {
		t.Fatalf("champion %s", tr.Bracket.Champion())
	}
	// Overriding the final reopens nothing but changes the champion.
	_, err = tr.SetResult(organiser, "W2-1", "e4", [2]int{2, 0}, "override", "", t0)
	must(t, err)
	tr.Tick(t0)
	if tr.Bracket.Champion() != "e4" || tr.Status != Completed {
		t.Fatal("final override")
	}
}

func TestDisqualifyAndWithdrawDuringPlay(t *testing.T) {
	tr := started(t, baseSpec(), 4)
	_, err := tr.Disqualify(actor(2), "e1", "", t0)
	wantKind(t, err, KindForbidden)
	e, err := tr.Disqualify(organiser, "e1", "Cheating", t0)
	must(t, err)
	if e.Status != EntrantDisqualified || tr.Bracket.Match("W1-1").WinnerID() != "e4" {
		t.Fatal("DQ did not forfeit")
	}
	must(t, tr.Withdraw(actor(2), t0))
	if tr.Bracket.Match("W1-2").WinnerID() != "e3" {
		t.Fatal("withdraw did not forfeit")
	}
	if _, err := tr.Register(actor(1), t0); err == nil {
		t.Fatal("re-registered after DQ")
	}
}

func TestStaffRoles(t *testing.T) {
	tr := newOpen(t, baseSpec(), 2)
	admin, caster := actor(50), actor(51)
	wantKind(t, tr.SetStaff(admin, admin.UserRef, RoleAdmin, t0), KindForbidden)
	must(t, tr.SetStaff(organiser, admin.UserRef, RoleAdmin, t0))
	must(t, tr.SetStaff(organiser, caster.UserRef, RoleCaster, t0))
	if !tr.CanManage(admin) || tr.CanManage(caster) || !tr.CanSeePrivate(caster) {
		t.Fatal("roles")
	}
	// Admins manage matches but don't change the staff.
	wantKind(t, tr.SetStaff(admin, caster.UserRef, RoleNone, t0), KindForbidden)
	must(t, tr.SetStaff(organiser, admin.UserRef, RoleNone, t0))
	if tr.CanManage(admin) {
		t.Fatal("removed admin still manages")
	}
	hub := actor(77)
	hub.HubAdmin = true
	if !tr.CanManage(hub) {
		t.Fatal("hub admin")
	}
	wantKind(t, tr.SetStaff(organiser, organiser.UserRef, RoleAdmin, t0), KindInvalid)
}

func TestDoubleEliminationThroughTheAggregate(t *testing.T) {
	spec := baseSpec()
	spec.Format = bracket.DoubleElimination
	spec.Options.GrandFinalReset = true
	spec.Ruleset.BestOf = 1
	tr := started(t, spec, 4)
	for guard := 0; tr.Status == InProgress && guard < 50; guard++ {
		ready := tr.Bracket.Playable()
		if len(ready) == 0 {
			t.Fatalf("stuck")
		}
		id := ready[0].ID
		// The losers-bracket champion takes the first grand final to force the reset.
		sc := [2]float64{2, 1}
		if id == "GF1" {
			sc = [2]float64{1, 2}
		}
		playMatch(t, tr, id, sc)
		confirm(t, tr, id)
		tr.Tick(t0)
	}
	if tr.Status != Completed || tr.Bracket.Match("GF2").State != bracket.Complete {
		t.Fatalf("status %s", tr.Status)
	}
}

func TestAggregateJSONRoundTrip(t *testing.T) {
	tr := started(t, baseSpec(), 4)
	playMatch(t, tr, "W1-1", [2]float64{2, 1})
	data, err := json.Marshal(tr)
	must(t, err)
	var back Tournament
	must(t, json.Unmarshal(data, &back))
	s := back.Series["W1-1"]
	host := Actor{UserRef: back.Entrant(s.Host).User}
	must(t, back.ReportGame(host, "W1-1", GameReport{Index: 1, Scores: [2]float64{2, 1}}, t0))
	if back.State(back.Bracket.Match("W1-1")) != StateAwaiting {
		t.Fatal("round trip lost the series")
	}
	d, err := back.OpenDispute(Actor{UserRef: back.Entrant("e4").User}, "W1-1", "Checking the replay please", t0)
	must(t, err)
	if d.ID != "d5" {
		t.Fatalf("ids continue after loading: %s", d.ID)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{"Synthetic Cup #3!": "synthetic-cup-3", "  ": "tournament", "Ünïcode Ópen": "n-code-pen"}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Fatalf("%q -> %q, want %q", in, got, want)
		}
	}
}

// replayFile builds a synthetic format 2 replay header with a stub body.
func replayFile(header map[string]any) []byte {
	h, _ := json.Marshal(header)
	out := []byte("AMRPLAY2")
	out = binary.LittleEndian.AppendUint32(out, 2)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(h)))
	out = append(out, h...)
	out = binary.LittleEndian.AppendUint32(out, 5)
	out = binary.LittleEndian.AppendUint32(out, 64)
	return append(out, make([]byte, 32)...)
}

func goodHeader() map[string]any {
	return map[string]any{"kind": "header", "version": 2, "id": "synthetic-replay", "coordinates": "unreal-centimeters",
		"recordedAt": t0.Format(time.RFC3339), "scenario": "Alpha", "reason": "completed", "frames": 3600, "inputEvents": 5000,
		"score": 1000.0, "duration": 60.0}
}

func TestDecodeReplayHeader(t *testing.T) {
	info, err := DecodeReplayHeader(replayFile(goodHeader()))
	must(t, err)
	if info.Scenario != "Alpha" || info.Score != 1000 || info.Frames != 3600 || info.Algorithm != 5 || info.BodySize != 64 {
		t.Fatalf("info %+v", info)
	}
	if _, err := DecodeReplayHeader([]byte("not a replay at all, really")); err != ErrNotReplay {
		t.Fatal(err)
	}
	bad := goodHeader()
	bad["coordinates"] = "meters"
	if _, err := DecodeReplayHeader(replayFile(bad)); err == nil {
		t.Fatal("wrong coordinates accepted")
	}
	short := replayFile(goodHeader())
	if _, err := DecodeReplayHeader(short[:30]); err == nil {
		t.Fatal("truncated header accepted")
	}
	seeded := goodHeader()
	seeded["seed"] = 42
	info, err = DecodeReplayHeader(replayFile(seeded))
	if err != nil || info.Seed != "42" {
		t.Fatalf("seed %q %v", info.Seed, err)
	}
}

func TestCheckReplay(t *testing.T) {
	g := &Game{Scenario: "Alpha", Seed: 42, TimeLimit: 60}
	start := t0.Add(-time.Minute)
	reported := t0.Add(time.Minute)
	info, _ := DecodeReplayHeader(replayFile(goodHeader()))
	status, checks := CheckReplay(info, g, 1000, &start, &reported)
	if status != ReplayVerified {
		t.Fatalf("good replay: %s %v", status, checks)
	}
	if !strings.Contains(strings.Join(checks, "|"), "seed: not in this replay") {
		t.Fatalf("missing-seed note %v", checks)
	}
	cases := []struct {
		mutate func(map[string]any)
		score  float64
		want   string
	}{
		{func(h map[string]any) { h["scenario"] = "Other" }, 1000, ReplayRejected},
		{func(h map[string]any) { h["inputEvents"] = 0 }, 1000, ReplayRejected},
		{func(h map[string]any) {}, 1200, ReplaySuspicious},
		{func(h map[string]any) { h["duration"] = 30.0 }, 1000, ReplaySuspicious},
		{func(h map[string]any) { h["reason"] = "aborted" }, 1000, ReplaySuspicious},
		{func(h map[string]any) { h["frames"] = 100 }, 1000, ReplaySuspicious},
		{func(h map[string]any) { h["recordedAt"] = t0.Add(-time.Hour).Format(time.RFC3339) }, 1000, ReplaySuspicious},
		{func(h map[string]any) { h["seed"] = "7" }, 1000, ReplaySuspicious},
		{func(h map[string]any) { h["seed"] = "42" }, 1000, ReplayVerified},
	}
	for i, c := range cases {
		h := goodHeader()
		c.mutate(h)
		info, err := DecodeReplayHeader(replayFile(h))
		must(t, err)
		if status, checks := CheckReplay(info, g, c.score, &start, &reported); status != c.want {
			t.Fatalf("case %d: %s %v", i, status, checks)
		}
	}
}

func TestUploadsAttachAndFlagGames(t *testing.T) {
	spec := baseSpec()
	spec.Ruleset.BestOf = 1
	tr := started(t, spec, 2)
	m := tr.Bracket.Match("W1-1")
	must(t, tr.MarkReady(actor(1), "W1-1", true, true, t0))
	must(t, tr.MarkReady(actor(2), "W1-1", true, true, t0))
	info, _ := DecodeReplayHeader(replayFile(goodHeader()))
	_, _, err := tr.AddUpload(actor(3), "W1-1", 0, "aa", 10, "k", info, t0)
	wantKind(t, err, KindForbidden)
	_, _, err = tr.AddUpload(actor(1), "W1-1", 4, "aa", 10, "k", info, t0)
	wantKind(t, err, KindInvalid)
	u1, _, err := tr.AddUpload(actor(1), "W1-1", 0, "aa", 10, "k1", info, t0)
	must(t, err)
	// Replacing an upload returns the old storage key for deletion.
	u1b, replaced, err := tr.AddUpload(actor(1), "W1-1", 0, "bb", 10, "k1b", info, t0)
	must(t, err)
	if replaced != "k1" || tr.Uploads[u1.ID] != nil {
		t.Fatal("upload not replaced")
	}
	bad := goodHeader()
	bad["score"] = 10.0
	badInfo, _ := DecodeReplayHeader(replayFile(bad))
	u2, _, err := tr.AddUpload(actor(2), "W1-1", 0, "cc", 10, "k2", badInfo, t0)
	must(t, err)
	host := player(tr, m, 0)
	must(t, tr.ReportGame(host, "W1-1", GameReport{Index: 0, Scores: [2]float64{1000, 900}}, t0))
	g := tr.Series["W1-1"].Games[0]
	if len(g.Replays) != 2 || tr.Uploads[u1b.ID].Status != ReplayVerified || tr.Uploads[u2.ID].Status != ReplaySuspicious || !contains(g.Flags, "replay-suspicious") {
		t.Fatalf("game %+v uploads %+v %+v", g, tr.Uploads[u1b.ID], tr.Uploads[u2.ID])
	}
	if !tr.Series["W1-1"].blocked(tr) {
		t.Fatal("suspicious replay should block auto-confirm")
	}
}
