// Package bracket generates and advances tournament brackets.
//
// It is pure: no clock, storage or randomness of its own. Callers pass
// entrants in seed order (index 0 is the top seed) and report results by
// match id; the engine fills downstream slots, resolves walkovers for byes
// and forfeits, and computes standings.
package bracket

import (
	"errors"
	"fmt"
	"math/bits"
	"sort"
)

type Format string

const (
	SingleElimination Format = "single_elimination"
	DoubleElimination Format = "double_elimination"
	RoundRobin        Format = "round_robin"
	Swiss             Format = "swiss"
)

// Side groups matches into the parts of a bracket.
type Side string

const (
	Winners    Side = "winners"
	Losers     Side = "losers"
	GrandFinal Side = "grand_final"
	ThirdPlace Side = "third_place"
	Pool       Side = "round_robin"
	SwissSide  Side = "swiss"
)

// SlotState: pending until its source resolves; filled with an entrant, or
// empty when the source produced nobody (a bye, or a void match).
type SlotState string

const (
	SlotPending SlotState = "pending"
	SlotFilled  SlotState = "filled"
	SlotEmpty   SlotState = "empty"
)

// SourceKind says where a slot gets its entrant from.
type SourceKind string

const (
	FromSeed   SourceKind = "seed"
	FromWinner SourceKind = "winner"
	FromLoser  SourceKind = "loser"
	// FromSlot copies an entrant from a slot of another match (the grand
	// final reset plays the same two entrants as the first grand final).
	FromSlot SourceKind = "slot"
	FromBye  SourceKind = "bye"
)

type Source struct {
	Kind  SourceKind `json:"kind"`
	Match string     `json:"match,omitempty"`
	Seed  int        `json:"seed,omitempty"` // 1-based
	Slot  int        `json:"slot,omitempty"`
}

type Slot struct {
	Entrant string    `json:"entrant,omitempty"`
	State   SlotState `json:"state"`
	From    Source    `json:"from"`
}

type MatchState string

const (
	// Pending: at least one entrant is not known yet.
	Pending MatchState = "pending"
	// Ready: both entrants are known and the match can be played.
	Ready MatchState = "ready"
	// Complete: decided by play, walkover, forfeit or override.
	Complete MatchState = "complete"
	// Skipped: the match is not needed (an unused grand final reset, or a
	// void match between two byes).
	Skipped MatchState = "skipped"
)

// Resolution says how a complete match was decided.
type Resolution string

const (
	Played   Resolution = "played"
	Walkover Resolution = "walkover" // the opponent slot was empty (a bye)
	Forfeit  Resolution = "forfeit"  // an entrant withdrew, no-showed or was disqualified
	Override Resolution = "override" // an organiser set the result
)

type Link struct {
	Match string `json:"match"`
	Slot  int    `json:"slot"`
}

type Match struct {
	ID       string     `json:"id"`
	Side     Side       `json:"side"`
	Round    int        `json:"round"`
	Position int        `json:"position"`
	Slots    [2]Slot    `json:"slots"`
	State    MatchState `json:"state"`
	// Winner is the winning slot (0 or 1), or -1.
	Winner     int        `json:"winner"`
	Score      [2]int     `json:"score"`
	Resolution Resolution `json:"resolution,omitempty"`
	WinnerTo   *Link      `json:"winnerTo,omitempty"`
	LoserTo    *Link      `json:"loserTo,omitempty"`
	// Stage orders eliminations for standings: the loser of this match is
	// out at this stage (0 when the loser carries on, e.g. into the losers bracket).
	Stage int `json:"stage,omitempty"`
}

func (m *Match) Entrants() (string, string) { return m.Slots[0].Entrant, m.Slots[1].Entrant }

// WinnerID and LoserID are empty until the match is complete.
func (m *Match) WinnerID() string {
	if m.State != Complete || m.Winner < 0 {
		return ""
	}
	return m.Slots[m.Winner].Entrant
}

func (m *Match) LoserID() string {
	if m.State != Complete || m.Winner < 0 {
		return ""
	}
	return m.Slots[1-m.Winner].Entrant
}

func (m *Match) Has(entrant string) bool {
	return entrant != "" && (m.Slots[0].Entrant == entrant || m.Slots[1].Entrant == entrant)
}

type Options struct {
	// ThirdPlace adds a bronze match between the semi-final losers (single elimination).
	ThirdPlace bool `json:"thirdPlace,omitempty"`
	// GrandFinalReset plays a second grand final when the losers-bracket
	// champion wins the first (double elimination). On by default in Generate callers.
	GrandFinalReset bool `json:"grandFinalReset,omitempty"`
	// Cycles: how many times each pair meets in a round robin (1 or 2).
	Cycles int `json:"cycles,omitempty"`
	// SwissRounds: rounds to play; 0 picks ceil(log2(n)).
	SwissRounds int `json:"swissRounds,omitempty"`
}

type Bracket struct {
	Format   Format   `json:"format"`
	Options  Options  `json:"options"`
	Entrants []string `json:"entrants"` // seed order
	Matches  []*Match `json:"matches"`
	// Withdrawn entrants lose every match they are (or later get) placed in.
	Withdrawn map[string]bool `json:"withdrawn,omitempty"`
	// Byes records Swiss byes per entrant (one at most each).
	Byes  map[string]int `json:"byes,omitempty"`
	index map[string]*Match
}

var (
	ErrUnknownMatch  = errors.New("unknown match")
	ErrNotReady      = errors.New("match is not ready to be played")
	ErrAlreadyPlayed = errors.New("match already has a result")
	ErrBadResult     = errors.New("invalid result")
	ErrRoundOpen     = errors.New("the current round is not finished")
	ErrNoMoreRounds  = errors.New("all rounds have been played")
	ErrTooFew        = errors.New("at least two entrants are needed")
)

const MaxEntrants = 512

// Generate builds the bracket for entrants given in seed order.
func Generate(format Format, entrants []string, opts Options) (*Bracket, error) {
	if len(entrants) < 2 {
		return nil, ErrTooFew
	}
	if len(entrants) > MaxEntrants {
		return nil, fmt.Errorf("at most %d entrants", MaxEntrants)
	}
	seen := map[string]bool{}
	for _, e := range entrants {
		if e == "" || seen[e] {
			return nil, errors.New("entrant ids must be unique and non-empty")
		}
		seen[e] = true
	}
	b := &Bracket{Format: format, Options: opts, Entrants: append([]string(nil), entrants...)}
	switch format {
	case SingleElimination:
		b.buildSingle()
	case DoubleElimination:
		b.buildDouble()
	case RoundRobin:
		if b.Options.Cycles < 1 || b.Options.Cycles > 2 {
			b.Options.Cycles = 1
		}
		b.buildRoundRobin()
	case Swiss:
		if b.Options.SwissRounds <= 0 {
			b.Options.SwissRounds = DefaultSwissRounds(len(entrants))
		}
		if b.Options.SwissRounds > len(entrants)-1 {
			b.Options.SwissRounds = len(entrants) - 1
		}
		b.Byes = map[string]int{}
		b.reindex()
		if err := b.pairSwissRound(1); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown format %q", format)
	}
	b.reindex()
	b.settle()
	return b, nil
}

// DefaultSwissRounds is ceil(log2(n)), enough to find a single unbeaten entrant.
func DefaultSwissRounds(n int) int {
	if n <= 1 {
		return 0
	}
	return bits.Len(uint(n - 1))
}

func (b *Bracket) reindex() {
	b.index = make(map[string]*Match, len(b.Matches))
	for _, m := range b.Matches {
		b.index[m.ID] = m
	}
}

// Match returns a match by id. Brackets decoded from JSON index lazily.
func (b *Bracket) Match(id string) *Match {
	if b.index == nil || len(b.index) != len(b.Matches) {
		b.reindex()
	}
	return b.index[id]
}

func (b *Bracket) add(m *Match) *Match {
	m.Winner = -1
	if m.State == "" {
		m.State = Pending
	}
	b.Matches = append(b.Matches, m)
	return m
}

func nextPow2(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

// SeedOrder returns the standard bracket line-up for size slots (a power of
// two): 1 v size, then each half mirrored so the top two seeds can only meet
// in the final. Values are 1-based seeds.
func SeedOrder(size int) []int {
	order := []int{1}
	for len(order) < size {
		n := len(order) * 2
		next := make([]int, 0, n)
		for _, s := range order {
			next = append(next, s, n+1-s)
		}
		order = next
	}
	return order
}

func (b *Bracket) seedSlot(seed int) Slot {
	if seed > len(b.Entrants) {
		return Slot{State: SlotEmpty, From: Source{Kind: FromBye, Seed: seed}}
	}
	return Slot{Entrant: b.Entrants[seed-1], State: SlotFilled, From: Source{Kind: FromSeed, Seed: seed}}
}

// buildWinners creates the winners bracket and returns its rounds.
func (b *Bracket) buildWinners(prefix string, side Side) [][]*Match {
	size := nextPow2(len(b.Entrants))
	order := SeedOrder(size)
	rounds := [][]*Match{}
	first := []*Match{}
	for i := 0; i < size/2; i++ {
		m := b.add(&Match{ID: fmt.Sprintf("%s1-%d", prefix, i+1), Side: side, Round: 1, Position: i + 1})
		m.Slots[0] = b.seedSlot(order[2*i])
		m.Slots[1] = b.seedSlot(order[2*i+1])
		first = append(first, m)
	}
	rounds = append(rounds, first)
	for r := 2; len(rounds[len(rounds)-1]) > 1; r++ {
		prev := rounds[len(rounds)-1]
		cur := []*Match{}
		for i := 0; i < len(prev)/2; i++ {
			m := b.add(&Match{ID: fmt.Sprintf("%s%d-%d", prefix, r, i+1), Side: side, Round: r, Position: i + 1})
			for s := 0; s < 2; s++ {
				src := prev[2*i+s]
				m.Slots[s] = Slot{State: SlotPending, From: Source{Kind: FromWinner, Match: src.ID}}
				src.WinnerTo = &Link{Match: m.ID, Slot: s}
			}
			cur = append(cur, m)
		}
		rounds = append(rounds, cur)
	}
	return rounds
}

func (b *Bracket) buildSingle() {
	rounds := b.buildWinners("W", Winners)
	// Stages are doubled so a bronze match winner can sit between stages.
	for r, round := range rounds {
		for _, m := range round {
			m.Stage = 2 * (r + 1)
		}
	}
	if b.Options.ThirdPlace && len(rounds) >= 2 {
		semis := rounds[len(rounds)-2]
		third := b.add(&Match{ID: "3P", Side: ThirdPlace, Round: len(rounds), Position: 1, Stage: 2 * (len(rounds) - 1)})
		for s := 0; s < 2; s++ {
			third.Slots[s] = Slot{State: SlotPending, From: Source{Kind: FromLoser, Match: semis[s].ID}}
			semis[s].LoserTo = &Link{Match: third.ID, Slot: s}
			semis[s].Stage = 0 // decided by the bronze match instead
		}
	} else {
		b.Options.ThirdPlace = false
	}
}

// buildDouble: winners bracket, a losers bracket fed by winners-bracket
// losers, and a grand final with an optional reset.
//
// For a winners bracket of k rounds the losers bracket has 2(k-1) rounds:
// odd rounds pair the survivors (round 1 pairs winners-round-1 losers), even
// rounds drop winners-round losers in against them. Drop-ins are reversed on
// alternate rounds so first-round opponents don't meet again straight away.
func (b *Bracket) buildDouble() {
	wb := b.buildWinners("W", Winners)
	k := len(wb)
	var lbChampion func(slot int, gf *Match)
	if k == 1 {
		final := wb[0][0]
		lbChampion = func(slot int, gf *Match) {
			gf.Slots[slot] = Slot{State: SlotPending, From: Source{Kind: FromLoser, Match: final.ID}}
			final.LoserTo = &Link{Match: gf.ID, Slot: slot}
		}
	} else {
		stage := 0
		var prev []*Match
		// Losers round 1: pairs of winners-round-1 losers.
		{
			stage += 2
			src := wb[0]
			cur := []*Match{}
			for i := 0; i < len(src)/2; i++ {
				m := b.add(&Match{ID: fmt.Sprintf("L1-%d", i+1), Side: Losers, Round: 1, Position: i + 1, Stage: stage})
				for s := 0; s < 2; s++ {
					w := src[2*i+s]
					m.Slots[s] = Slot{State: SlotPending, From: Source{Kind: FromLoser, Match: w.ID}}
					w.LoserTo = &Link{Match: m.ID, Slot: s}
				}
				cur = append(cur, m)
			}
			prev = cur
		}
		for wr := 2; wr <= k; wr++ {
			// Even round: survivors against winners-round wr losers.
			stage += 2
			lr := 2 * (wr - 1)
			drops := wb[wr-1]
			cur := []*Match{}
			for i := range prev {
				m := b.add(&Match{ID: fmt.Sprintf("L%d-%d", lr, i+1), Side: Losers, Round: lr, Position: i + 1, Stage: stage})
				survivor := prev[i]
				m.Slots[0] = Slot{State: SlotPending, From: Source{Kind: FromWinner, Match: survivor.ID}}
				survivor.WinnerTo = &Link{Match: m.ID, Slot: 0}
				d := i
				if wr%2 == 0 {
					d = len(drops) - 1 - i
				}
				drop := drops[d]
				m.Slots[1] = Slot{State: SlotPending, From: Source{Kind: FromLoser, Match: drop.ID}}
				drop.LoserTo = &Link{Match: m.ID, Slot: 1}
				cur = append(cur, m)
			}
			prev = cur
			if wr == k {
				break
			}
			// Odd round: pair the survivors.
			stage += 2
			lr++
			cur = []*Match{}
			for i := 0; i < len(prev)/2; i++ {
				m := b.add(&Match{ID: fmt.Sprintf("L%d-%d", lr, i+1), Side: Losers, Round: lr, Position: i + 1, Stage: stage})
				for s := 0; s < 2; s++ {
					src := prev[2*i+s]
					m.Slots[s] = Slot{State: SlotPending, From: Source{Kind: FromWinner, Match: src.ID}}
					src.WinnerTo = &Link{Match: m.ID, Slot: s}
				}
				cur = append(cur, m)
			}
			prev = cur
		}
		lbFinal := prev[0]
		lbChampion = func(slot int, gf *Match) {
			gf.Slots[slot] = Slot{State: SlotPending, From: Source{Kind: FromWinner, Match: lbFinal.ID}}
			lbFinal.WinnerTo = &Link{Match: gf.ID, Slot: slot}
		}
	}
	finalStage := 2 * (2*(k-1) + 1)
	wbFinal := wb[k-1][0]
	gf := b.add(&Match{ID: "GF1", Side: GrandFinal, Round: 1, Position: 1, Stage: finalStage})
	gf.Slots[0] = Slot{State: SlotPending, From: Source{Kind: FromWinner, Match: wbFinal.ID}}
	wbFinal.WinnerTo = &Link{Match: gf.ID, Slot: 0}
	lbChampion(1, gf)
	if b.Options.GrandFinalReset {
		reset := b.add(&Match{ID: "GF2", Side: GrandFinal, Round: 2, Position: 1, Stage: finalStage})
		reset.Slots[0] = Slot{State: SlotPending, From: Source{Kind: FromSlot, Match: gf.ID, Slot: 0}}
		reset.Slots[1] = Slot{State: SlotPending, From: Source{Kind: FromSlot, Match: gf.ID, Slot: 1}}
	}
}

// buildRoundRobin uses the circle method: one entrant stays fixed while the
// others rotate, so every pair meets exactly once per cycle and nobody plays
// twice in a round. An odd field gets a phantom; its pairings are rest rounds.
func (b *Bracket) buildRoundRobin() {
	players := append([]string(nil), b.Entrants...)
	if len(players)%2 == 1 {
		players = append(players, "")
	}
	n := len(players)
	rounds := n - 1
	round := 0
	for c := 0; c < b.Options.Cycles; c++ {
		ring := append([]string(nil), players...)
		for r := 0; r < rounds; r++ {
			round++
			pos := 0
			for i := 0; i < n/2; i++ {
				a, z := ring[i], ring[n-1-i]
				if a == "" || z == "" {
					continue
				}
				// Alternate sides so the fixed entrant isn't always first, and
				// swap on the second cycle.
				if (r%2 == 1 && i == 0) != (c == 1) {
					a, z = z, a
				}
				pos++
				m := b.add(&Match{ID: fmt.Sprintf("R%d-%d", round, pos), Side: Pool, Round: round, Position: pos})
				m.Slots[0] = Slot{Entrant: a, State: SlotFilled, From: Source{Kind: FromSeed, Seed: b.seedOf(a)}}
				m.Slots[1] = Slot{Entrant: z, State: SlotFilled, From: Source{Kind: FromSeed, Seed: b.seedOf(z)}}
			}
			// Rotate everyone but the first.
			last := ring[n-1]
			copy(ring[2:], ring[1:n-1])
			ring[1] = last
		}
	}
}

func (b *Bracket) seedOf(entrant string) int {
	for i, e := range b.Entrants {
		if e == entrant {
			return i + 1
		}
	}
	return 0
}

// Rounds returns the highest round number on a side.
func (b *Bracket) Rounds(side Side) int {
	n := 0
	for _, m := range b.Matches {
		if m.Side == side && m.Round > n {
			n = m.Round
		}
	}
	return n
}

// Report records a played result. winner is the winning slot.
func (b *Bracket) Report(matchID string, winner int, score [2]int) error {
	m := b.Match(matchID)
	if m == nil {
		return ErrUnknownMatch
	}
	if m.State == Complete || m.State == Skipped {
		return ErrAlreadyPlayed
	}
	if m.State != Ready {
		return ErrNotReady
	}
	if winner != 0 && winner != 1 || score[0] < 0 || score[1] < 0 || score[winner] <= score[1-winner] {
		return ErrBadResult
	}
	b.complete(m, winner, score, Played)
	b.settle()
	return nil
}

// Forfeit awards a ready match to the other entrant.
func (b *Bracket) Forfeit(matchID, loser string) error {
	m := b.Match(matchID)
	if m == nil {
		return ErrUnknownMatch
	}
	if m.State == Complete || m.State == Skipped {
		return ErrAlreadyPlayed
	}
	if m.State != Ready || !m.Has(loser) {
		return ErrNotReady
	}
	w := 0
	if m.Slots[0].Entrant == loser {
		w = 1
	}
	b.complete(m, w, [2]int{}, Forfeit)
	b.settle()
	return nil
}

// Withdraw removes an entrant: their ready matches are forfeited now and any
// match they reach later is forfeited as soon as it becomes ready. Results
// already played stand.
func (b *Bracket) Withdraw(entrant string) {
	if b.Withdrawn == nil {
		b.Withdrawn = map[string]bool{}
	}
	b.Withdrawn[entrant] = true
	b.settle()
}

// SetResult is the organiser override: it sets (or changes) a match result.
// When a completed match changes winner, every match that depended on it is
// reset and returned so callers can tell the affected players. Matches with
// both entrants still in place keep their results.
func (b *Bracket) SetResult(matchID string, winner int, score [2]int) ([]string, error) {
	m := b.Match(matchID)
	if m == nil {
		return nil, ErrUnknownMatch
	}
	if winner != 0 && winner != 1 || score[0] < 0 || score[1] < 0 || score[winner] < score[1-winner] {
		return nil, ErrBadResult
	}
	if m.State != Ready && m.State != Complete {
		return nil, ErrNotReady
	}
	if m.Slots[winner].State != SlotFilled {
		return nil, ErrBadResult
	}
	var reset []string
	if m.State == Complete && m.Winner != winner {
		reset = b.clearDownstream(m)
	}
	m.State = Ready
	b.complete(m, winner, score, Override)
	b.settle()
	return reset, nil
}

// clearDownstream un-fills the slots this match fed and recursively resets
// matches that can no longer stand.
func (b *Bracket) clearDownstream(m *Match) []string {
	var reset []string
	for _, l := range b.links(m) {
		target := b.Match(l.Match)
		if target == nil {
			continue
		}
		if target.State == Complete || target.State == Skipped {
			reset = append(reset, b.resetMatch(target)...)
		}
		target.Slots[l.Slot].Entrant = ""
		target.Slots[l.Slot].State = SlotPending
		if target.State != Pending {
			target.State = Pending
		}
	}
	return reset
}

func (b *Bracket) resetMatch(m *Match) []string {
	reset := []string{m.ID}
	reset = append(reset, b.clearDownstream(m)...)
	m.State = Pending
	m.Winner = -1
	m.Score = [2]int{}
	m.Resolution = ""
	return reset
}

// links returns every slot fed by this match, including the grand final reset.
func (b *Bracket) links(m *Match) []Link {
	var out []Link
	if m.WinnerTo != nil {
		out = append(out, *m.WinnerTo)
	}
	if m.LoserTo != nil {
		out = append(out, *m.LoserTo)
	}
	for _, other := range b.Matches {
		for s, slot := range other.Slots {
			if slot.From.Kind == FromSlot && slot.From.Match == m.ID {
				out = append(out, Link{Match: other.ID, Slot: s})
			}
		}
	}
	return out
}

func (b *Bracket) complete(m *Match, winner int, score [2]int, how Resolution) {
	m.State = Complete
	m.Winner = winner
	m.Score = score
	m.Resolution = how
}

func (b *Bracket) fill(l *Link, entrant string) {
	if l == nil {
		return
	}
	t := b.Match(l.Match)
	if t == nil || t.Slots[l.Slot].State != SlotPending {
		return
	}
	if entrant == "" {
		t.Slots[l.Slot].State = SlotEmpty
	} else {
		t.Slots[l.Slot].Entrant = entrant
		t.Slots[l.Slot].State = SlotFilled
	}
}

// settle propagates results until nothing changes: fills downstream slots,
// opens matches whose entrants are known, resolves byes, forfeits for
// withdrawn entrants, and the grand final reset.
func (b *Bracket) settle() {
	for {
		for changed := true; changed; {
			changed = false
			for _, m := range b.Matches {
				if b.step(m) {
					changed = true
				}
			}
		}
		// Swiss pairs the next round as soon as the current one is decided.
		if b.Format == Swiss && b.swissRoundDone() && b.Rounds(SwissSide) < b.Options.SwissRounds {
			if b.pairSwissRound(b.Rounds(SwissSide)+1) == nil {
				b.reindex()
				continue
			}
		}
		return
	}
}

func (b *Bracket) step(m *Match) bool {
	switch m.State {
	case Pending:
		if m.ID == "GF2" {
			return b.stepReset(m)
		}
		a, z := m.Slots[0].State, m.Slots[1].State
		if a == SlotPending || z == SlotPending {
			return false
		}
		switch {
		case a == SlotFilled && z == SlotFilled:
			m.State = Ready
		case a == SlotFilled:
			b.complete(m, 0, [2]int{}, Walkover)
		case z == SlotFilled:
			b.complete(m, 1, [2]int{}, Walkover)
		default:
			// Two byes: nobody advances.
			m.State = Skipped
			b.fill(m.WinnerTo, "")
			b.fill(m.LoserTo, "")
		}
		return true
	case Ready:
		wa, wz := b.Withdrawn[m.Slots[0].Entrant], b.Withdrawn[m.Slots[1].Entrant]
		if !wa && !wz {
			return false
		}
		if wa && wz {
			// Both out: the higher seed is recorded as advancing so the
			// bracket can continue; it forfeits its next match in turn.
			w := 0
			if b.seedOf(m.Slots[1].Entrant) < b.seedOf(m.Slots[0].Entrant) {
				w = 1
			}
			b.complete(m, w, [2]int{}, Forfeit)
		} else if wa {
			b.complete(m, 1, [2]int{}, Forfeit)
		} else {
			b.complete(m, 0, [2]int{}, Forfeit)
		}
		return true
	case Complete:
		changed := false
		if m.WinnerTo != nil && b.slotPending(m.WinnerTo) {
			b.fill(m.WinnerTo, m.WinnerID())
			changed = true
		}
		if m.LoserTo != nil && b.slotPending(m.LoserTo) {
			// A walkover has no loser to pass on.
			loser := ""
			if m.Resolution != Walkover {
				loser = m.LoserID()
			}
			b.fill(m.LoserTo, loser)
			changed = true
		}
		return changed
	}
	return false
}

func (b *Bracket) slotPending(l *Link) bool {
	t := b.Match(l.Match)
	return t != nil && t.Slots[l.Slot].State == SlotPending
}

// stepReset opens GF2 only when the losers-bracket champion (slot 1) won GF1.
func (b *Bracket) stepReset(reset *Match) bool {
	gf := b.Match(reset.Slots[0].From.Match)
	if gf == nil {
		return false
	}
	switch gf.State {
	case Complete:
		if gf.Winner == 0 || gf.Resolution == Walkover || gf.Resolution == Forfeit {
			reset.State = Skipped
			return true
		}
		for s := 0; s < 2; s++ {
			reset.Slots[s].Entrant = gf.Slots[s].Entrant
			reset.Slots[s].State = SlotFilled
		}
		reset.State = Ready
		return true
	case Skipped:
		reset.State = Skipped
		return true
	}
	return false
}

// Done reports whether the bracket has a final result.
func (b *Bracket) Done() bool {
	if b.Format == Swiss {
		if b.Rounds(SwissSide) < b.Options.SwissRounds {
			return false
		}
	}
	for _, m := range b.Matches {
		if m.State == Pending || m.State == Ready {
			return false
		}
	}
	return true
}

// Champion is the overall winner once the bracket is done, else "".
func (b *Bracket) Champion() string {
	if !b.Done() {
		return ""
	}
	s := b.Standings()
	if len(s) == 0 || s[0].Place != 1 {
		return ""
	}
	// A shared first place has no champion.
	if len(s) > 1 && s[1].Place == 1 {
		return ""
	}
	return s[0].Entrant
}

// Playable lists the matches that can be played now.
func (b *Bracket) Playable() []*Match {
	var out []*Match
	for _, m := range b.Matches {
		if m.State == Ready {
			out = append(out, m)
		}
	}
	return out
}

// Standing is one entrant's place and record.
type Standing struct {
	Entrant    string  `json:"entrant"`
	Place      int     `json:"place"`
	Wins       int     `json:"wins"`
	Losses     int     `json:"losses"`
	GamesWon   int     `json:"gamesWon"`
	GamesLost  int     `json:"gamesLost"`
	Points     int     `json:"points"`
	Buchholz   int     `json:"buchholz,omitempty"`
	Eliminated bool    `json:"eliminated"`
	Seed       int     `json:"seed"`
	Score      float64 `json:"-"`
}

func (b *Bracket) records() map[string]*Standing {
	rec := map[string]*Standing{}
	for i, e := range b.Entrants {
		rec[e] = &Standing{Entrant: e, Seed: i + 1}
	}
	for _, m := range b.Matches {
		if m.State != Complete || m.Resolution == Walkover {
			continue
		}
		w, l := rec[m.WinnerID()], rec[m.LoserID()]
		if w == nil || l == nil {
			continue
		}
		w.Wins++
		l.Losses++
		w.GamesWon += m.Score[m.Winner]
		w.GamesLost += m.Score[1-m.Winner]
		l.GamesWon += m.Score[1-m.Winner]
		l.GamesLost += m.Score[m.Winner]
	}
	return rec
}

// Standings: places for every entrant. Elimination brackets place by the
// stage an entrant went out (ties share a place, e.g. both semi-final losers
// are 3rd without a bronze match); round robin and Swiss rank by match
// points, then tie-breaks.
func (b *Bracket) Standings() []Standing {
	switch b.Format {
	case RoundRobin, Swiss:
		return b.tableStandings()
	}
	rec := b.records()
	const alive = 1 << 30
	out := map[string]int{}
	for _, e := range b.Entrants {
		out[e] = alive
	}
	finalStage := 0
	for _, m := range b.Matches {
		if m.Stage > finalStage {
			finalStage = m.Stage
		}
	}
	for _, m := range b.Matches {
		if m.State != Complete || m.Stage == 0 {
			continue
		}
		if m.ID == "GF1" && b.Match("GF2") != nil && m.Winner == 1 && m.Resolution != Walkover && m.Resolution != Forfeit {
			continue // the WB champion's first loss: the reset decides
		}
		if l := m.LoserID(); l != "" && m.Resolution != Walkover {
			out[l] = m.Stage
		}
	}
	// The bronze match orders the two semi-final losers.
	if bronze := b.Match("3P"); bronze != nil {
		if bronze.State == Complete {
			if w := bronze.WinnerID(); w != "" {
				out[w] = bronze.Stage + 1
			}
			if l := bronze.LoserID(); l != "" && bronze.Resolution != Walkover {
				out[l] = bronze.Stage
			}
		}
	}
	// The final's winner sits above the runner-up.
	for _, m := range b.Matches {
		if m.State == Complete && m.Stage == finalStage && m.Side != ThirdPlace {
			if w := m.WinnerID(); w != "" && (m.ID != "GF1" || b.Match("GF2") == nil || m.Winner == 0 || b.Match("GF2").State == Skipped) {
				out[w] = finalStage + 1
			}
		}
	}
	list := make([]Standing, 0, len(b.Entrants))
	for _, e := range b.Entrants {
		s := *rec[e]
		s.Eliminated = out[e] != alive && out[e] <= finalStage
		if b.Withdrawn[e] {
			s.Eliminated = true
		}
		list = append(list, s)
	}
	stage := func(e string) int {
		v := out[e]
		if b.Withdrawn[e] && v == alive {
			return 0
		}
		return v
	}
	sort.SliceStable(list, func(i, j int) bool {
		si, sj := stage(list[i].Entrant), stage(list[j].Entrant)
		if si != sj {
			return si > sj
		}
		return list[i].Seed < list[j].Seed
	})
	for i := range list {
		if i > 0 && stage(list[i].Entrant) == stage(list[i-1].Entrant) {
			list[i].Place = list[i-1].Place
		} else {
			list[i].Place = i + 1
		}
	}
	return list
}

func (b *Bracket) tableStandings() []Standing {
	rec := b.records()
	// Swiss byes count as a win (but no games).
	for e, n := range b.Byes {
		if r := rec[e]; r != nil {
			r.Wins += n
		}
	}
	for _, r := range rec {
		r.Points = r.Wins
	}
	if b.Format == Swiss {
		for _, m := range b.Matches {
			if m.State != Complete || m.Resolution == Walkover {
				continue
			}
			a, z := rec[m.Slots[0].Entrant], rec[m.Slots[1].Entrant]
			if a != nil && z != nil {
				a.Buchholz += z.Points
				z.Buchholz += a.Points
			}
		}
	}
	list := make([]Standing, 0, len(rec))
	for _, e := range b.Entrants {
		s := *rec[e]
		s.Eliminated = b.Withdrawn[e]
		list = append(list, s)
	}
	// Points, Buchholz (Swiss), game difference, games won, then seed. A tie
	// of exactly two entrants on points is settled by their head-to-head
	// first; head-to-head is not used for bigger ties, where it can be cyclic.
	cmp := func(x, y Standing) int {
		switch {
		case x.Points != y.Points:
			return y.Points - x.Points
		case x.Buchholz != y.Buchholz:
			return y.Buchholz - x.Buchholz
		}
		if dx, dy := x.GamesWon-x.GamesLost, y.GamesWon-y.GamesLost; dx != dy {
			return dy - dx
		}
		return y.GamesWon - x.GamesWon
	}
	sort.SliceStable(list, func(i, j int) bool {
		if c := cmp(list[i], list[j]); c != 0 {
			return c < 0
		}
		return list[i].Seed < list[j].Seed
	})
	h2h := b.headToHead()
	samePoints := func(x, y Standing) bool { return x.Points == y.Points && x.Buchholz == y.Buchholz }
	decided := map[int]bool{} // index i: list[i] and list[i+1] were separated by head-to-head
	for i := 0; i < len(list); {
		j := i
		for j+1 < len(list) && samePoints(list[i], list[j+1]) {
			j++
		}
		if j == i+1 {
			x, y := list[i].Entrant, list[j].Entrant
			if h2h[[2]string{y, x}] > h2h[[2]string{x, y}] {
				list[i], list[j] = list[j], list[i]
			}
			if h2h[[2]string{y, x}] != h2h[[2]string{x, y}] {
				decided[i] = true
			}
		}
		i = j + 1
	}
	for i := range list {
		if i > 0 && !decided[i-1] && cmp(list[i], list[i-1]) == 0 {
			list[i].Place = list[i-1].Place
		} else {
			list[i].Place = i + 1
		}
	}
	return list
}

// headToHead counts match wins of a over b.
func (b *Bracket) headToHead() map[[2]string]int {
	out := map[[2]string]int{}
	for _, m := range b.Matches {
		if m.State == Complete && m.Resolution != Walkover {
			out[[2]string{m.WinnerID(), m.LoserID()}]++
		}
	}
	return out
}
