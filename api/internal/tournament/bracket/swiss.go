package bracket

import (
	"fmt"
	"sort"
)

// Swiss: every round pairs entrants with the same (or nearest) record who
// have not met yet. Round 1 pairs the top half of the seeds against the
// bottom half. An odd field gives one bye per round to the lowest-ranked
// entrant without one; a bye counts as a match win with no games.

func (b *Bracket) swissRoundDone() bool {
	r := b.Rounds(SwissSide)
	if r == 0 {
		return false
	}
	for _, m := range b.Matches {
		if m.Side == SwissSide && m.Round == r && (m.State == Pending || m.State == Ready) {
			return false
		}
	}
	return true
}

func (b *Bracket) pairSwissRound(round int) error {
	if round > b.Options.SwissRounds {
		return ErrNoMoreRounds
	}
	var order []string
	if round == 1 {
		order = append(order, b.Entrants...)
	} else {
		for _, s := range b.tableStandings() {
			order = append(order, s.Entrant)
		}
	}
	active := order[:0:0]
	for _, e := range order {
		if !b.Withdrawn[e] {
			active = append(active, e)
		}
	}
	if len(active) < 2 {
		// Nobody left to pair: close the event at this round.
		b.Options.SwissRounds = round - 1
		return ErrNoMoreRounds
	}
	if len(active)%2 == 1 {
		bye := -1
		for i := len(active) - 1; i >= 0; i-- {
			if b.Byes[active[i]] == 0 {
				bye = i
				break
			}
		}
		if bye < 0 {
			bye = len(active) - 1
		}
		if b.Byes == nil {
			b.Byes = map[string]int{}
		}
		b.Byes[active[bye]]++
		active = append(active[:bye:bye], active[bye+1:]...)
	}
	var pairs [][2]string
	if round == 1 {
		half := len(active) / 2
		for i := 0; i < half; i++ {
			pairs = append(pairs, [2]string{active[i], active[i+half]})
		}
	} else {
		played := map[[2]string]bool{}
		for _, m := range b.Matches {
			a, z := m.Slots[0].Entrant, m.Slots[1].Entrant
			if a != "" && z != "" {
				played[[2]string{a, z}] = true
				played[[2]string{z, a}] = true
			}
		}
		budget := 200000
		pairs = pairAvoiding(active, played, &budget)
		if pairs == nil {
			// Rematches can't be avoided (small fields late on): pair in order.
			for i := 0; i+1 < len(active); i += 2 {
				pairs = append(pairs, [2]string{active[i], active[i+1]})
			}
		}
	}
	for i, p := range pairs {
		m := b.add(&Match{ID: fmt.Sprintf("S%d-%d", round, i+1), Side: SwissSide, Round: round, Position: i + 1})
		m.Slots[0] = Slot{Entrant: p[0], State: SlotFilled, From: Source{Kind: FromSeed, Seed: b.seedOf(p[0])}}
		m.Slots[1] = Slot{Entrant: p[1], State: SlotFilled, From: Source{Kind: FromSeed, Seed: b.seedOf(p[1])}}
	}
	return nil
}

// pairAvoiding pairs the ranked list top-down, each entrant with the
// nearest-ranked opponent they have not played, backtracking when a choice
// leaves the rest unpairable. nil when no rematch-free pairing exists (or the
// search budget runs out).
func pairAvoiding(ranked []string, played map[[2]string]bool, budget *int) [][2]string {
	if len(ranked) == 0 {
		return [][2]string{}
	}
	*budget--
	if *budget < 0 {
		return nil
	}
	first := ranked[0]
	for j := 1; j < len(ranked); j++ {
		if played[[2]string{first, ranked[j]}] {
			continue
		}
		rest := make([]string, 0, len(ranked)-2)
		rest = append(rest, ranked[1:j]...)
		rest = append(rest, ranked[j+1:]...)
		if tail := pairAvoiding(rest, played, budget); tail != nil {
			return append([][2]string{{first, ranked[j]}}, tail...)
		}
		if *budget < 0 {
			return nil
		}
	}
	return nil
}

// SwissOpponents lists who an entrant has met, for tie-break display.
func (b *Bracket) SwissOpponents(entrant string) []string {
	var out []string
	for _, m := range b.Matches {
		if m.Side != SwissSide || !m.Has(entrant) {
			continue
		}
		if m.Slots[0].Entrant == entrant {
			out = append(out, m.Slots[1].Entrant)
		} else {
			out = append(out, m.Slots[0].Entrant)
		}
	}
	sort.Strings(out)
	return out
}
