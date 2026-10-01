package bracket

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

func players(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("p%02d", i+1)
	}
	return out
}

// favourite: the better (lower) seed wins.
func favourite(b *Bracket, m *Match) int {
	if b.seedOf(m.Slots[0].Entrant) < b.seedOf(m.Slots[1].Entrant) {
		return 0
	}
	return 1
}

// play reports every ready match with pick until the bracket is done.
func play(t *testing.T, b *Bracket, pick func(*Bracket, *Match) int) int {
	t.Helper()
	played := 0
	for guard := 0; !b.Done(); guard++ {
		if guard > 4*len(b.Entrants)*len(b.Entrants)+100 {
			t.Fatalf("bracket never finished")
		}
		ready := b.Playable()
		if len(ready) == 0 {
			t.Fatalf("not done but nothing playable: %s", dump(b))
		}
		busy := map[string]string{}
		for _, m := range ready {
			for _, s := range m.Slots {
				key := s.Entrant
				if b.Format == RoundRobin {
					// Every round robin match is open at once; one per entrant per round.
					key = fmt.Sprintf("%s#%d", s.Entrant, m.Round)
				}
				if other, ok := busy[key]; ok {
					t.Fatalf("%s is in two ready matches (%s, %s)", s.Entrant, other, m.ID)
				}
				busy[key] = m.ID
			}
		}
		m := ready[0]
		w := pick(b, m)
		score := [2]int{}
		score[w] = 2
		score[1-w] = 1
		if err := b.Report(m.ID, w, score); err != nil {
			t.Fatalf("report %s: %v", m.ID, err)
		}
		played++
	}
	return played
}

func dump(b *Bracket) string {
	out := ""
	for _, m := range b.Matches {
		out += fmt.Sprintf("\n%s %s [%s:%s %s:%s] w=%d", m.ID, m.State, m.Slots[0].Entrant, m.Slots[0].State, m.Slots[1].Entrant, m.Slots[1].State, m.Winner)
	}
	return out
}

func losses(b *Bracket) map[string]int {
	out := map[string]int{}
	for _, m := range b.Matches {
		if m.State == Complete && m.Resolution != Walkover {
			out[m.LoserID()]++
		}
	}
	return out
}

func TestSeedOrderKeepsTopSeedsApart(t *testing.T) {
	if got, want := SeedOrder(8), []int{1, 8, 4, 5, 2, 7, 3, 6}; !reflect.DeepEqual(got, want) {
		t.Fatalf("seed order 8 = %v, want %v", got, want)
	}
	for _, size := range []int{2, 4, 16, 64, 512} {
		order := SeedOrder(size)
		seen := map[int]bool{}
		for i := 0; i < size; i += 2 {
			if order[i]+order[i+1] != size+1 {
				t.Fatalf("size %d pairs %d with %d", size, order[i], order[i+1])
			}
		}
		for _, s := range order {
			seen[s] = true
		}
		if len(seen) != size {
			t.Fatalf("size %d repeats seeds", size)
		}
		// Seeds 1 and 2 sit in different halves.
		half := map[int]int{}
		for i, s := range order {
			half[s] = i / (size / 2)
		}
		if size > 1 && half[1] == half[2] {
			t.Fatalf("size %d puts seeds 1 and 2 in one half", size)
		}
	}
}

func TestTooFewOrDuplicateEntrants(t *testing.T) {
	for _, f := range []Format{SingleElimination, DoubleElimination, RoundRobin, Swiss} {
		if _, err := Generate(f, []string{"a"}, Options{}); err == nil {
			t.Fatalf("%s accepted one entrant", f)
		}
		if _, err := Generate(f, []string{"a", "a"}, Options{}); err == nil {
			t.Fatalf("%s accepted duplicate entrants", f)
		}
		if _, err := Generate(f, []string{"a", ""}, Options{}); err == nil {
			t.Fatalf("%s accepted an empty id", f)
		}
	}
	if _, err := Generate("ladder", players(4), Options{}); err == nil {
		t.Fatalf("unknown format accepted")
	}
}

func TestSingleEliminationByesGoToTopSeeds(t *testing.T) {
	b, err := Generate(SingleElimination, players(5), Options{})
	if err != nil {
		t.Fatal(err)
	}
	// 8 slots, 3 byes: seeds 1, 2 and 3 advance without playing.
	walkovers := map[string]bool{}
	for _, m := range b.Matches {
		if m.Round == 1 && m.Resolution == Walkover {
			walkovers[m.WinnerID()] = true
		}
	}
	if !reflect.DeepEqual(walkovers, map[string]bool{"p01": true, "p02": true, "p03": true}) {
		t.Fatalf("byes went to %v", walkovers)
	}
	// 4 v 5 plays first; 2 v 3 meet straight away in round 2.
	ready := b.Playable()
	if len(ready) != 2 || ready[0].ID != "W1-2" || ready[1].ID != "W2-2" {
		t.Fatalf("playable: %s", dump(b))
	}
}

func TestSingleEliminationEveryFieldSize(t *testing.T) {
	for n := 2; n <= 70; n++ {
		for _, third := range []bool{false, true} {
			b, err := Generate(SingleElimination, players(n), Options{ThirdPlace: third})
			if err != nil {
				t.Fatal(err)
			}
			rng := rand.New(rand.NewSource(int64(n)))
			played := play(t, b, func(*Bracket, *Match) int { return rng.Intn(2) })
			want := n - 1
			if third && n >= 4 {
				want++
			}
			if played != want {
				t.Fatalf("n=%d third=%v played %d matches, want %d", n, third, played, want)
			}
			champ := b.Champion()
			if champ == "" {
				t.Fatalf("n=%d no champion", n)
			}
			l := losses(b)
			if l[champ] != 0 {
				t.Fatalf("n=%d champion lost", n)
			}
			for _, e := range b.Entrants {
				extra := 0
				if third && n >= 4 && b.Match("3P").Has(e) {
					extra = 1
				}
				if e != champ && (l[e] < 1 || l[e] > 1+extra) {
					t.Fatalf("n=%d %s has %d losses", n, e, l[e])
				}
			}
			s := b.Standings()
			if s[0].Entrant != champ || s[0].Place != 1 || s[1].Place != 2 {
				t.Fatalf("n=%d standings top: %+v", n, s[:2])
			}
			if third && n >= 4 && (s[2].Place != 3 || s[3].Place != 4) {
				t.Fatalf("n=%d bronze match should separate 3rd and 4th: %+v", n, s[:4])
			}
			if !third && n >= 4 && (s[2].Place != 3 || s[3].Place != 3) {
				t.Fatalf("n=%d semi-final losers share 3rd: %+v", n, s[:4])
			}
			for _, st := range s[1:] {
				if !st.Eliminated {
					t.Fatalf("n=%d %s not marked eliminated", n, st.Entrant)
				}
			}
		}
	}
}

func TestSingleEliminationFavouritesMeetInFinal(t *testing.T) {
	b, _ := Generate(SingleElimination, players(16), Options{})
	play(t, b, favourite)
	final := b.Match("W4-1")
	a, z := final.Entrants()
	if a != "p01" || z != "p02" {
		t.Fatalf("final was %s v %s", a, z)
	}
	semis := []*Match{b.Match("W3-1"), b.Match("W3-2")}
	if x, y := semis[0].Entrants(); x != "p01" || y != "p04" {
		t.Fatalf("semi 1 was %s v %s", x, y)
	}
	if x, y := semis[1].Entrants(); x != "p02" || y != "p03" {
		t.Fatalf("semi 2 was %s v %s", x, y)
	}
	s := b.Standings()
	want := []int{1, 2, 3, 3, 5, 5, 5, 5, 9, 9, 9, 9, 9, 9, 9, 9}
	for i, st := range s {
		if st.Place != want[i] || st.Entrant != fmt.Sprintf("p%02d", i+1) {
			t.Fatalf("standing %d = %+v", i, st)
		}
	}
}

func TestReportValidation(t *testing.T) {
	b, _ := Generate(SingleElimination, players(4), Options{})
	if err := b.Report("nope", 0, [2]int{1, 0}); err != ErrUnknownMatch {
		t.Fatal(err)
	}
	if err := b.Report("W2-1", 0, [2]int{1, 0}); err != ErrNotReady {
		t.Fatalf("final reported before semis: %v", err)
	}
	if err := b.Report("W1-1", 0, [2]int{0, 1}); err != ErrBadResult {
		t.Fatalf("winner with lower score: %v", err)
	}
	if err := b.Report("W1-1", 2, [2]int{1, 0}); err != ErrBadResult {
		t.Fatalf("slot 2: %v", err)
	}
	if err := b.Report("W1-1", 0, [2]int{-1, -2}); err != ErrBadResult {
		t.Fatalf("negative score: %v", err)
	}
	if err := b.Report("W1-1", 0, [2]int{1, 0}); err != nil {
		t.Fatal(err)
	}
	if err := b.Report("W1-1", 1, [2]int{0, 1}); err != ErrAlreadyPlayed {
		t.Fatalf("double report: %v", err)
	}
}

func TestDoubleEliminationEveryFieldSize(t *testing.T) {
	for n := 2; n <= 70; n++ {
		for _, reset := range []bool{true, false} {
			b, err := Generate(DoubleElimination, players(n), Options{GrandFinalReset: reset})
			if err != nil {
				t.Fatal(err)
			}
			rng := rand.New(rand.NewSource(int64(n * 7)))
			played := play(t, b, func(*Bracket, *Match) int { return rng.Intn(2) })
			champ := b.Champion()
			if champ == "" {
				t.Fatalf("n=%d reset=%v no champion: %s", n, reset, dump(b))
			}
			l := losses(b)
			if l[champ] > 1 {
				t.Fatalf("n=%d champion has %d losses", n, l[champ])
			}
			for _, e := range b.Entrants {
				if e == champ {
					continue
				}
				// Without a reset the winners-bracket champion goes out on one loss.
				if l[e] != 2 && !(l[e] == 1 && !reset) {
					t.Fatalf("n=%d reset=%v %s has %d losses: %s", n, reset, e, l[e], dump(b))
				}
			}
			gf2 := b.Match("GF2")
			usedReset := gf2 != nil && gf2.State == Complete
			want := 2*n - 2
			if usedReset {
				want++
			}
			if played != want {
				t.Fatalf("n=%d reset=%v played %d, want %d", n, reset, played, want)
			}
			s := b.Standings()
			if s[0].Entrant != champ || s[0].Place != 1 || s[1].Place != 2 || s[1].Eliminated == false {
				t.Fatalf("n=%d standings: %+v", n, s[:2])
			}
			if n >= 3 && s[2].Place != 3 {
				t.Fatalf("n=%d third place: %+v", n, s[:3])
			}
		}
	}
}

func TestDoubleEliminationStructure(t *testing.T) {
	b, _ := Generate(DoubleElimination, players(8), Options{GrandFinalReset: true})
	count := map[Side]int{}
	for _, m := range b.Matches {
		count[m.Side]++
	}
	if count[Winners] != 7 || count[Losers] != 6 || count[GrandFinal] != 2 {
		t.Fatalf("8-player double elimination sides: %v", count)
	}
	if b.Rounds(Losers) != 4 {
		t.Fatalf("losers rounds = %d", b.Rounds(Losers))
	}
	// Every winners match except the final feeds the losers bracket.
	for _, m := range b.Matches {
		if m.Side == Winners && m.LoserTo == nil {
			t.Fatalf("%s loser goes nowhere", m.ID)
		}
		if m.Side == Losers && m.LoserTo != nil {
			t.Fatalf("losers match %s has a second life", m.ID)
		}
	}
	// Drop-ins in losers round 2 are reversed, so W1 losers don't replay W2 pairings at once.
	l2 := b.Match("L2-1")
	if l2.Slots[1].From.Match != "W2-2" {
		t.Fatalf("L2-1 drop-in from %s", l2.Slots[1].From.Match)
	}
}

func TestDoubleEliminationFavouritesNoReset(t *testing.T) {
	b, _ := Generate(DoubleElimination, players(8), Options{GrandFinalReset: true})
	play(t, b, favourite)
	gf := b.Match("GF1")
	if a, z := gf.Entrants(); a != "p01" || z != "p02" {
		t.Fatalf("grand final %s v %s", a, z)
	}
	if b.Match("GF2").State != Skipped {
		t.Fatalf("reset played after the winners champion won")
	}
	if b.Champion() != "p01" {
		t.Fatalf("champion %s", b.Champion())
	}
	s := b.Standings()
	wantPlaces := []int{1, 2, 3, 4, 5, 5, 7, 7}
	for i := range s {
		if s[i].Place != wantPlaces[i] {
			t.Fatalf("places %+v", s)
		}
	}
}

func TestGrandFinalReset(t *testing.T) {
	b, _ := Generate(DoubleElimination, players(4), Options{GrandFinalReset: true})
	// Everyone but the grand final goes to the favourite.
	for !b.Done() {
		m := b.Playable()[0]
		w := favourite(b, m)
		if m.ID == "GF1" {
			w = 1 // the losers-bracket champion takes the first final
		}
		if m.ID == "GF2" {
			if m.State != Ready {
				t.Fatal("reset not ready")
			}
			if a, z := m.Entrants(); a != "p01" || z != "p02" {
				t.Fatalf("reset %s v %s", a, z)
			}
			if b.Champion() != "" {
				t.Fatal("champion before the reset")
			}
			w = 1
		}
		score := [2]int{}
		score[w] = 1
		if err := b.Report(m.ID, w, score); err != nil {
			t.Fatal(err)
		}
	}
	if b.Champion() != "p02" {
		t.Fatalf("champion %q after winning the reset", b.Champion())
	}
	if l := losses(b); l["p02"] != 1 || l["p01"] != 2 {
		t.Fatalf("losses %v", l)
	}
	s := b.Standings()
	if s[0].Entrant != "p02" || s[1].Entrant != "p01" || s[1].Place != 2 {
		t.Fatalf("standings %+v", s)
	}
}

func TestGrandFinalWithoutResetEndsOnFirstFinal(t *testing.T) {
	b, _ := Generate(DoubleElimination, players(4), Options{})
	if b.Match("GF2") != nil {
		t.Fatal("reset created though disabled")
	}
	for !b.Done() {
		m := b.Playable()[0]
		w := favourite(b, m)
		if m.ID == "GF1" {
			w = 1
		}
		score := [2]int{}
		score[w] = 1
		_ = b.Report(m.ID, w, score)
	}
	if b.Champion() != "p02" {
		t.Fatalf("champion %s", b.Champion())
	}
}

func TestDoubleEliminationByesNeverCreateEmptyFinals(t *testing.T) {
	for _, n := range []int{3, 5, 6, 9, 12, 17, 33} {
		b, _ := Generate(DoubleElimination, players(n), Options{GrandFinalReset: true})
		play(t, b, favourite)
		gf := b.Match("GF1")
		if gf.Slots[0].State != SlotFilled || gf.Slots[1].State != SlotFilled || gf.Resolution != Played {
			t.Fatalf("n=%d grand final not played by two entrants: %s", n, dump(b))
		}
	}
}

func TestRoundRobin(t *testing.T) {
	for n := 2; n <= 17; n++ {
		for _, cycles := range []int{1, 2} {
			b, err := Generate(RoundRobin, players(n), Options{Cycles: cycles})
			if err != nil {
				t.Fatal(err)
			}
			pairs := map[[2]string]int{}
			perRound := map[int]map[string]bool{}
			for _, m := range b.Matches {
				if m.State != Ready {
					t.Fatalf("round robin match %s not ready", m.ID)
				}
				a, z := m.Entrants()
				if a > z {
					a, z = z, a
				}
				pairs[[2]string{a, z}]++
				if perRound[m.Round] == nil {
					perRound[m.Round] = map[string]bool{}
				}
				for _, e := range []string{a, z} {
					if perRound[m.Round][e] {
						t.Fatalf("n=%d %s plays twice in round %d", n, e, m.Round)
					}
					perRound[m.Round][e] = true
				}
			}
			if len(pairs) != n*(n-1)/2 {
				t.Fatalf("n=%d %d distinct pairs", n, len(pairs))
			}
			for p, c := range pairs {
				if c != cycles {
					t.Fatalf("n=%d pair %v meets %d times", n, p, c)
				}
			}
			rounds := n - 1
			if n%2 == 1 {
				rounds = n
			}
			if b.Rounds(Pool) != rounds*cycles {
				t.Fatalf("n=%d rounds %d", n, b.Rounds(Pool))
			}
			play(t, b, favourite)
			s := b.Standings()
			for i, st := range s {
				if st.Entrant != fmt.Sprintf("p%02d", i+1) || st.Place != i+1 || st.Wins != (n-1-i)*cycles {
					t.Fatalf("n=%d standings %+v", n, s)
				}
			}
		}
	}
}

func TestRoundRobinTieBreaks(t *testing.T) {
	// A three-way cycle: everyone 1-1. Game difference decides.
	b, _ := Generate(RoundRobin, []string{"a", "b", "c"}, Options{})
	results := map[[2]string][2]int{{"a", "b"}: {2, 0}, {"b", "c"}: {2, 1}, {"c", "a"}: {2, 1}}
	for _, m := range b.Matches {
		x, y := m.Entrants()
		if r, ok := results[[2]string{x, y}]; ok {
			_ = b.Report(m.ID, 0, r)
		} else {
			r := results[[2]string{y, x}]
			_ = b.Report(m.ID, 1, [2]int{r[1], r[0]})
		}
	}
	s := b.Standings()
	// a: +2 -1 = +1 (3-2); b: 2-2 = 0; c: 3-3 = 0, then games won 3 > 2.
	if s[0].Entrant != "a" || s[1].Entrant != "c" || s[2].Entrant != "b" {
		t.Fatalf("tie-breaks %+v", s)
	}
	// Head to head separates a two-way tie on points, ahead of game difference.
	b2, _ := Generate(RoundRobin, []string{"a", "b", "c", "d"}, Options{})
	wins := map[[2]string][2]int{ // winner, loser -> winner's games, loser's games
		{"b", "a"}: {1, 0}, {"a", "c"}: {2, 0}, {"a", "d"}: {2, 0},
		{"b", "c"}: {1, 0}, {"d", "b"}: {1, 0}, {"c", "d"}: {1, 0},
	}
	for _, m := range b2.Matches {
		x, y := m.Entrants()
		if g, ok := wins[[2]string{x, y}]; ok {
			_ = b2.Report(m.ID, 0, g)
		} else {
			g := wins[[2]string{y, x}]
			_ = b2.Report(m.ID, 1, [2]int{g[1], g[0]})
		}
	}
	s2 := b2.Standings()
	order := []string{s2[0].Entrant, s2[1].Entrant, s2[2].Entrant, s2[3].Entrant}
	if !reflect.DeepEqual(order, []string{"b", "a", "c", "d"}) {
		t.Fatalf("head to head %+v", s2)
	}
	for i, st := range s2 {
		if st.Place != i+1 {
			t.Fatalf("head-to-head ties should not share places: %+v", s2)
		}
	}
}

func TestSwissPairsWithoutRematches(t *testing.T) {
	for _, n := range []int{4, 6, 7, 8, 9, 16, 21, 32} {
		b, err := Generate(Swiss, players(n), Options{})
		if err != nil {
			t.Fatal(err)
		}
		if b.Options.SwissRounds != DefaultSwissRounds(n) {
			t.Fatalf("n=%d rounds %d", n, b.Options.SwissRounds)
		}
		rng := rand.New(rand.NewSource(int64(n)))
		play(t, b, func(*Bracket, *Match) int { return rng.Intn(2) })
		if b.Rounds(SwissSide) != b.Options.SwissRounds {
			t.Fatalf("n=%d played %d rounds", n, b.Rounds(SwissSide))
		}
		met := map[[2]string]bool{}
		for _, m := range b.Matches {
			a, z := m.Entrants()
			if a > z {
				a, z = z, a
			}
			if met[[2]string{a, z}] {
				t.Fatalf("n=%d rematch %s v %s", n, a, z)
			}
			met[[2]string{a, z}] = true
		}
		byes := 0
		for e, c := range b.Byes {
			if c > 1 {
				t.Fatalf("n=%d %s had %d byes", n, e, c)
			}
			byes += c
		}
		if n%2 == 1 && byes != b.Options.SwissRounds {
			t.Fatalf("n=%d byes %d", n, byes)
		}
		s := b.Standings()
		if s[0].Place != 1 {
			t.Fatalf("n=%d standings %+v", n, s)
		}
	}
}

func TestSwissRoundOnePairsHalves(t *testing.T) {
	b, _ := Generate(Swiss, players(8), Options{SwissRounds: 3})
	want := [][2]string{{"p01", "p05"}, {"p02", "p06"}, {"p03", "p07"}, {"p04", "p08"}}
	for i, m := range b.Matches {
		a, z := m.Entrants()
		if [2]string{a, z} != want[i] {
			t.Fatalf("round 1 match %d = %s v %s", i, a, z)
		}
	}
	// The next round only appears when this one is done.
	for _, m := range b.Matches[:3] {
		_ = b.Report(m.ID, 0, [2]int{1, 0})
	}
	if b.Rounds(SwissSide) != 1 {
		t.Fatal("round 2 paired early")
	}
	_ = b.Report(b.Matches[3].ID, 0, [2]int{1, 0})
	if b.Rounds(SwissSide) != 2 {
		t.Fatal("round 2 not paired")
	}
	// Winners meet winners.
	for _, m := range b.Matches[4:] {
		a, z := m.Entrants()
		wa, wz := b.records()[a].Wins, b.records()[z].Wins
		if wa != wz {
			t.Fatalf("round 2 pairs %s (%d) with %s (%d)", a, wa, z, wz)
		}
	}
}

func TestWithdrawForfeitsCurrentAndFutureMatches(t *testing.T) {
	b, _ := Generate(SingleElimination, players(8), Options{})
	b.Withdraw("p08")
	m := b.Match("W1-1")
	if m.State != Complete || m.Resolution != Forfeit || m.WinnerID() != "p01" {
		t.Fatalf("withdrawn entrant's match: %s", dump(b))
	}
	// p04 withdraws before reaching round 2.
	b.Withdraw("p04")
	_ = b.Report("W1-2", 0, [2]int{1, 0}) // p04 beats p05? no: p04 already forfeited
	if b.Match("W1-2").WinnerID() != "p05" {
		t.Fatalf("p04's first match: %s", dump(b))
	}
	if r2 := b.Match("W2-1"); r2.State != Ready {
		t.Fatalf("W2-1 should be ready: %s", dump(b))
	}
	s := b.Standings()
	for _, st := range s {
		if (st.Entrant == "p08" || st.Entrant == "p04") && !st.Eliminated {
			t.Fatalf("withdrawn entrant not eliminated: %+v", st)
		}
	}
}

func TestWithdrawInDoubleEliminationSkipsLosersBracket(t *testing.T) {
	b, _ := Generate(DoubleElimination, players(4), Options{GrandFinalReset: true})
	b.Withdraw("p04")
	// p04 forfeits W1-1 and drops to L1-1, where it forfeits again once its opponent arrives.
	if l := b.Match("L1-1"); l.Slots[0].Entrant != "p04" || l.State != Pending {
		t.Fatalf("L1-1: %s", dump(b))
	}
	play(t, b, favourite)
	if b.Champion() != "p01" {
		t.Fatalf("champion %s", b.Champion())
	}
	for _, m := range b.Matches {
		if m.Has("p04") && m.Resolution != Forfeit {
			t.Fatalf("p04 played %s", m.ID)
		}
	}
}

func TestForfeit(t *testing.T) {
	b, _ := Generate(SingleElimination, players(4), Options{})
	if err := b.Forfeit("W1-1", "p02"); err != ErrNotReady {
		t.Fatalf("forfeit by someone not in the match: %v", err)
	}
	if err := b.Forfeit("W1-1", "p04"); err != nil {
		t.Fatal(err)
	}
	if b.Match("W1-1").WinnerID() != "p01" || b.Match("W2-1").Slots[0].Entrant != "p01" {
		t.Fatalf("forfeit did not advance: %s", dump(b))
	}
	if err := b.Forfeit("W1-1", "p01"); err != ErrAlreadyPlayed {
		t.Fatalf("second forfeit: %v", err)
	}
}

func TestSetResultResetsDependentMatches(t *testing.T) {
	b, _ := Generate(DoubleElimination, players(4), Options{GrandFinalReset: true})
	// W1-1: p01 v p04, W1-2: p02 v p03.
	_ = b.Report("W1-1", 0, [2]int{2, 0})
	_ = b.Report("W1-2", 0, [2]int{2, 1})
	_ = b.Report("W2-1", 0, [2]int{2, 1}) // p01 beats p02
	_ = b.Report("L1-1", 0, [2]int{2, 0}) // p04 v p03 -> p04
	if b.Match("L2-1").State != Ready {
		t.Fatalf("L2-1: %s", dump(b))
	}
	_ = b.Report("L2-1", 0, [2]int{2, 0}) // p04 beats p02
	// Overturn W1-1: p04 actually won.
	reset, err := b.SetResult("W1-1", 1, [2]int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"W2-1": true, "L1-1": true, "L2-1": true}
	got := map[string]bool{}
	for _, id := range reset {
		got[id] = true
	}
	for id := range want {
		if !got[id] {
			t.Fatalf("reset %v, missing %s", reset, id)
		}
	}
	if b.Match("W1-2").State != Complete {
		t.Fatal("an unrelated result was cleared")
	}
	w2 := b.Match("W2-1")
	if w2.State != Ready || w2.Slots[0].Entrant != "p04" || w2.Slots[1].Entrant != "p02" {
		t.Fatalf("W2-1 after override: %s", dump(b))
	}
	l1 := b.Match("L1-1")
	if l1.State != Ready || l1.Slots[0].Entrant != "p01" {
		t.Fatalf("L1-1 after override: %s", dump(b))
	}
	if b.Match("W1-1").Resolution != Override {
		t.Fatal("override not recorded")
	}
	// Same winner, new score: nothing downstream changes.
	if reset, _ := b.SetResult("W1-2", 0, [2]int{2, 0}); len(reset) != 0 {
		t.Fatalf("score-only override reset %v", reset)
	}
	if _, err := b.SetResult("GF1", 0, [2]int{1, 0}); err != ErrNotReady {
		t.Fatalf("override of a pending match: %v", err)
	}
	if _, err := b.SetResult("W1-2", 0, [2]int{0, 2}); err != ErrBadResult {
		t.Fatalf("override with the loser ahead: %v", err)
	}
}

func TestSetResultOnWalkoverAndAfterGrandFinal(t *testing.T) {
	b, _ := Generate(SingleElimination, players(3), Options{})
	// W1-1 is p01's bye; it can't be handed to the empty slot.
	if _, err := b.SetResult("W1-1", 1, [2]int{0, 1}); err != ErrBadResult {
		t.Fatalf("bye awarded to nobody: %v", err)
	}
	play(t, b, favourite)
	if b.Champion() != "p01" {
		t.Fatal(b.Champion())
	}
	if _, err := b.SetResult("W2-1", 1, [2]int{0, 1}); err != nil {
		t.Fatal(err)
	}
	if b.Champion() != "p02" {
		t.Fatalf("champion after final override %s", b.Champion())
	}
}

func TestJSONRoundTripKeepsPlaying(t *testing.T) {
	for _, f := range []Format{SingleElimination, DoubleElimination, RoundRobin, Swiss} {
		b, _ := Generate(f, players(7), Options{GrandFinalReset: true, ThirdPlace: true})
		rng := rand.New(rand.NewSource(3))
		for i := 0; !b.Done(); i++ {
			data, err := json.Marshal(b)
			if err != nil {
				t.Fatal(err)
			}
			var next Bracket
			if err := json.Unmarshal(data, &next); err != nil {
				t.Fatal(err)
			}
			b = &next
			m := b.Playable()[0]
			w := rng.Intn(2)
			score := [2]int{}
			score[w] = 1
			if err := b.Report(m.ID, w, score); err != nil {
				t.Fatalf("%s: %v", f, err)
			}
		}
		if f != RoundRobin && f != Swiss && b.Champion() == "" {
			t.Fatalf("%s: no champion after round trips", f)
		}
	}
}
