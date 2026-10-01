package tournament

import (
	"crypto/rand"
	"encoding/binary"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/tournament/bracket"
)

// Series is the play of one bracket match: readiness, the veto, the games
// with their seeds and replays, and the report awaiting confirmation.
type Series struct {
	Match string `json:"match"`
	// Entrants the series was opened for; an override that changes them opens a fresh series.
	Entrants    [2]string            `json:"entrants"`
	BestOf      int                  `json:"bestOf"`
	OpenedAt    time.Time            `json:"openedAt"`
	ScheduledAt *time.Time           `json:"scheduledAt,omitempty"`
	ReadyAt     map[string]time.Time `json:"readyAt,omitempty"`
	CanHost     map[string]bool      `json:"canHost,omitempty"`
	Host        string               `json:"host,omitempty"`
	Veto        []VetoEntry          `json:"veto,omitempty"`
	Games       []*Game              `json:"games,omitempty"`
	StartedAt   *time.Time           `json:"startedAt,omitempty"`
	ReportedBy  string               `json:"reportedBy,omitempty"`
	ReportedAt  *time.Time           `json:"reportedAt,omitempty"`
	// Deadline: the no-show deadline while waiting, the confirm deadline after a report.
	Deadline    *time.Time `json:"deadline,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	// Resolution beyond the bracket's: "no_show" for no-show forfeits.
	Resolution string   `json:"resolution,omitempty"`
	Flags      []string `json:"flags,omitempty"`
}

type VetoEntry struct {
	Step     int        `json:"step"`
	Action   VetoAction `json:"action"`
	Entrant  string     `json:"entrant,omitempty"`
	Scenario string     `json:"scenario"`
	At       time.Time  `json:"at"`
}

type Game struct {
	Index     int    `json:"index"`
	Scenario  string `json:"scenario"`
	Hash      string `json:"hash,omitempty"`
	TimeLimit int    `json:"timeLimit,omitempty"`
	// Seed: both players play with the same target randomness.
	Seed           uint64     `json:"seed"`
	HasScore       bool       `json:"hasScore"`
	Scores         [2]float64 `json:"scores"`
	Winner         int        `json:"winner"`
	Replays        []string   `json:"replays,omitempty"`
	ReportedBy     string     `json:"reportedBy,omitempty"`
	ReportedAt     *time.Time `json:"reportedAt,omitempty"`
	HostValidated  bool       `json:"hostValidated,omitempty"`
	PlayedScenario string     `json:"playedScenario,omitempty"`
	Flags          []string   `json:"flags,omitempty"`
	// Result is the lobby host's full record of the game (game-modes.md 8.4).
	Result *ResultRecord `json:"result,omitempty"`
}

// ResultRecord is the per-game result format shared with the in-game
// service: the record proposed in in-game/docs/game-modes.md section 8.4,
// with entrant ids, the seed, and Unix millisecond times.
type ResultRecord struct {
	Format       int            `json:"format"`
	Match        string         `json:"match"`
	Mode         string         `json:"mode"`
	SettingsKey  string         `json:"settingsKey,omitempty"`
	ScenarioHash string         `json:"scenarioHash,omitempty"`
	StartedAt    int64          `json:"startedAt"`
	EndedAt      int64          `json:"endedAt"`
	Host         string         `json:"host,omitempty"`
	Players      []ResultPlayer `json:"players"`
	Winner       string         `json:"winner,omitempty"`
	Seed         string         `json:"seed,omitempty"`
}

type ResultPlayer struct {
	Key          string   `json:"key"`
	Entrant      string   `json:"entrant"`
	Team         int      `json:"team"`
	Place        int      `json:"place"`
	Score        float64  `json:"score"`
	Frags        int      `json:"frags,omitempty"`
	Deaths       int      `json:"deaths,omitempty"`
	Claims       int      `json:"claims,omitempty"`
	Rejected     int      `json:"rejected,omitempty"`
	TrackPercent *float64 `json:"trackPercent,omitempty"`
	Disputed     bool     `json:"disputed,omitempty"`
	Replay       string   `json:"replay,omitempty"`
	Accuracy     float64  `json:"accuracy,omitempty"`
}

// SeriesState is what the API reports for a match.
type SeriesState string

const (
	StatePending  SeriesState = "pending"
	StateReady    SeriesState = "ready"
	StateVeto     SeriesState = "veto"
	StateLive     SeriesState = "live"
	StateAwaiting SeriesState = "awaiting_confirmation"
	StateDisputed SeriesState = "disputed"
	StateComplete SeriesState = "complete"
	StateSkipped  SeriesState = "skipped"
)

// randomSeed is replaceable in tests. Seeds are 32-bit: AimModCore's
// start-scenario seed is 0..4294967295 (in-game/native-mod/DESIGN.md).
var randomSeed = func() uint64 {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return uint64(binary.LittleEndian.Uint32(b[:]))
}

func (s *Series) addFlag(flag string) bool {
	for _, f := range s.Flags {
		if f == flag {
			return false
		}
	}
	s.Flags = append(s.Flags, flag)
	return true
}

func (g *Game) addFlag(flag string) {
	for _, f := range g.Flags {
		if f == flag {
			return
		}
	}
	g.Flags = append(g.Flags, flag)
}

// State of a bracket match with its series.
func (t *Tournament) State(m *bracket.Match) SeriesState {
	switch m.State {
	case bracket.Pending:
		return StatePending
	case bracket.Skipped:
		return StateSkipped
	case bracket.Complete:
		return StateComplete
	}
	s := t.Series[m.ID]
	switch {
	case s == nil:
		return StateReady
	case t.openDispute(m.ID) != nil:
		return StateDisputed
	case s.ReportedAt != nil:
		return StateAwaiting
	case s.StartedAt != nil:
		return StateLive
	case t.vetoSteps(m, s) != nil && len(s.Veto) < len(t.vetoSteps(m, s)) && len(s.ReadyAt) == 2:
		return StateVeto
	}
	return StateReady
}

// BestOf for a match: the ruleset default, or a round override.
func (t *Tournament) BestOf(m *bracket.Match) int {
	best := t.Spec.Ruleset.BestOf
	last := t.Bracket.Rounds(m.Side)
	for _, rb := range t.Spec.Ruleset.RoundBestOf {
		if rb.Side != m.Side {
			continue
		}
		if rb.FromRound == 0 && m.Round == last || rb.FromRound > 0 && m.Round >= rb.FromRound {
			best = rb.BestOf
		}
	}
	return best
}

// syncSeries opens a series for every newly ready match and drops series of
// matches an override sent back to pending.
func (t *Tournament) syncSeries(now time.Time) bool {
	if t.Bracket == nil {
		return false
	}
	if t.Series == nil {
		t.Series = map[string]*Series{}
	}
	changed := false
	for _, m := range t.Bracket.Matches {
		s := t.Series[m.ID]
		switch m.State {
		case bracket.Ready:
			a, z := m.Entrants()
			if s != nil && s.Entrants != [2]string{a, z} {
				s = nil
			}
			if s == nil {
				s = &Series{Match: m.ID, Entrants: [2]string{a, z}, BestOf: t.BestOf(m), OpenedAt: now, ReadyAt: map[string]time.Time{}, CanHost: map[string]bool{}}
				deadline := now.Add(time.Duration(t.Spec.ReadyWindow) * time.Minute)
				if t.Spec.Scheduling == Scheduled && t.Spec.StartsAt != nil && now.Before(*t.Spec.StartsAt) {
					at := *t.Spec.StartsAt
					s.ScheduledAt = &at
					deadline = at.Add(time.Duration(t.Spec.NoShow) * time.Minute)
				}
				s.Deadline = &deadline
				t.Series[m.ID] = s
				changed = true
			}
		case bracket.Pending:
			if s != nil {
				delete(t.Series, m.ID)
				changed = true
			}
		case bracket.Complete:
			if s != nil && s.CompletedAt == nil {
				at := now
				s.CompletedAt = &at
				s.Deadline = nil
				changed = true
			}
		}
	}
	return changed
}

func (t *Tournament) match(id string) (*bracket.Match, *Series, error) {
	// Completed stays reachable so organisers can still correct results.
	if t.Status != InProgress && t.Status != Completed || t.Bracket == nil {
		return nil, nil, errState("The tournament isn't running.")
	}
	m := t.Bracket.Match(id)
	if m == nil {
		return nil, nil, errNotFound("Unknown match.")
	}
	return m, t.Series[id], nil
}

// slotOf: 0 or 1 when the actor plays in the match, else -1.
func (t *Tournament) slotOf(m *bracket.Match, a Actor) int {
	e := t.EntrantOf(a.UserID)
	if e == nil {
		return -1
	}
	for i, s := range m.Slots {
		if s.Entrant == e.ID {
			return i
		}
	}
	return -1
}

func (t *Tournament) seedOf(entrant string) int {
	if e := t.Entrant(entrant); e != nil && e.Seed > 0 {
		return e.Seed
	}
	return math.MaxInt32
}

// higher: the slot of the better (lower-numbered) seed.
func (t *Tournament) higher(m *bracket.Match) int {
	if t.seedOf(m.Slots[1].Entrant) < t.seedOf(m.Slots[0].Entrant) {
		return 1
	}
	return 0
}

// MarkReady records that a player is online and ready. When both are, the
// host is chosen (the higher seed, unless only the other can host) and the
// veto or the first game begins.
func (t *Tournament) MarkReady(a Actor, matchID string, ready, canHost bool, now time.Time) error {
	if !a.SignedIn() {
		return ErrSignIn
	}
	m, s, err := t.match(matchID)
	if err != nil {
		return err
	}
	slot := t.slotOf(m, a)
	if slot < 0 {
		return errForbidden("You aren't playing in this match.")
	}
	if m.State != bracket.Ready || s == nil {
		return errState("This match isn't open.")
	}
	if s.StartedAt != nil {
		return errState("The match has already started.")
	}
	id := m.Slots[slot].Entrant
	if s.ReadyAt == nil {
		s.ReadyAt = map[string]time.Time{}
	}
	if s.CanHost == nil {
		s.CanHost = map[string]bool{}
	}
	if !ready {
		if len(s.Veto) > 0 {
			return errState("The veto has begun.")
		}
		delete(s.ReadyAt, id)
		s.Host = ""
		return nil
	}
	if _, ok := s.ReadyAt[id]; !ok {
		s.ReadyAt[id] = now
	}
	s.CanHost[id] = canHost
	if len(s.ReadyAt) < 2 {
		return nil
	}
	if s.Host == "" {
		h := t.higher(m)
		other := 1 - h
		if !s.CanHost[m.Slots[h].Entrant] && s.CanHost[m.Slots[other].Entrant] {
			h = other
		}
		s.Host = m.Slots[h].Entrant
	}
	if steps := t.vetoSteps(m, s); len(s.Veto) >= len(steps) {
		t.begin(m, s, now)
	}
	return nil
}

// vetoSteps for this match; nil when the games simply follow the pool.
// A custom veto is written for the ruleset's best-of; other best-ofs (a
// longer final) use the standard alternating veto.
func (t *Tournament) vetoSteps(m *bracket.Match, s *Series) []VetoStep {
	r := t.Spec.Ruleset
	if len(r.Veto) == 0 || len(r.Pool) < 2 {
		return nil
	}
	if s != nil && s.BestOf != r.BestOf {
		return DefaultVeto(len(r.Pool), s.BestOf)
	}
	return r.Veto
}

// available: pool scenarios not yet banned or picked.
func (t *Tournament) available(s *Series) []PoolScenario {
	used := map[string]bool{}
	for _, v := range s.Veto {
		used[strings.ToLower(v.Scenario)] = true
	}
	var out []PoolScenario
	for _, p := range t.Spec.Ruleset.Pool {
		if !used[strings.ToLower(p.Name)] {
			out = append(out, p)
		}
	}
	return out
}

// VetoTurn: who acts next in the veto and what they do; "" when the veto is over.
func (t *Tournament) VetoTurn(m *bracket.Match) (string, VetoAction) {
	s := t.Series[m.ID]
	if s == nil || len(s.ReadyAt) < 2 || s.StartedAt != nil {
		return "", ""
	}
	steps := t.vetoSteps(m, s)
	if len(s.Veto) >= len(steps) {
		return "", ""
	}
	step := steps[len(s.Veto)]
	slot := t.higher(m)
	if step.Actor == LowerSeed {
		slot = 1 - slot
	}
	return m.Slots[slot].Entrant, step.Action
}

// Veto bans or picks a scenario on the actor's turn.
func (t *Tournament) Veto(a Actor, matchID, scenario string, now time.Time) error {
	if !a.SignedIn() {
		return ErrSignIn
	}
	m, s, err := t.match(matchID)
	if err != nil {
		return err
	}
	if t.slotOf(m, a) < 0 {
		return errForbidden("You aren't playing in this match.")
	}
	turn, action := t.VetoTurn(m)
	if turn == "" {
		return errState("There's no veto to make now.")
	}
	if turn != m.Slots[t.slotOf(m, a)].Entrant {
		return errState("It's your opponent's turn.")
	}
	var chosen *PoolScenario
	for _, p := range t.available(s) {
		if strings.EqualFold(p.Name, strings.TrimSpace(scenario)) {
			p := p
			chosen = &p
		}
	}
	if chosen == nil {
		return errInvalid("That scenario isn't available.")
	}
	s.Veto = append(s.Veto, VetoEntry{Step: len(s.Veto) + 1, Action: action, Entrant: turn, Scenario: chosen.Name, At: now})
	if len(s.Veto) == len(t.vetoSteps(m, s)) {
		t.begin(m, s, now)
	}
	return nil
}

// order: the scenarios in game order. Picks first, then the decider(s): what
// the veto left, in pool order. Without a veto, the pool in order.
func (t *Tournament) order(s *Series) []PoolScenario {
	pool := t.Spec.Ruleset.Pool
	if len(s.Veto) == 0 {
		return pool
	}
	byName := map[string]PoolScenario{}
	for _, p := range pool {
		byName[strings.ToLower(p.Name)] = p
	}
	var out []PoolScenario
	for _, action := range []VetoAction{Pick, Decider} {
		for _, v := range s.Veto {
			if v.Action == action {
				out = append(out, byName[strings.ToLower(v.Scenario)])
			}
		}
	}
	return append(out, t.available(s)...)
}

func (t *Tournament) begin(m *bracket.Match, s *Series, now time.Time) {
	if s.StartedAt != nil {
		return
	}
	at := now
	s.StartedAt = &at
	s.Deadline = nil
	// Record the decider in the veto log.
	if len(s.Veto) > 0 {
		if left := t.available(s); len(left) > 0 {
			s.Veto = append(s.Veto, VetoEntry{Step: len(s.Veto) + 1, Action: Decider, Scenario: left[0].Name, At: now})
		}
	}
	t.nextGame(s)
}

// Wins counts games won by each slot.
func (s *Series) Wins() [2]int {
	var w [2]int
	for _, g := range s.Games {
		if g.HasScore && g.Winner >= 0 {
			w[g.Winner]++
		}
	}
	return w
}

func (s *Series) needed() int { return s.BestOf/2 + 1 }

// Current is the game being played, or nil.
func (s *Series) Current() *Game {
	if len(s.Games) == 0 {
		return nil
	}
	if g := s.Games[len(s.Games)-1]; !g.HasScore {
		return g
	}
	return nil
}

// nextGame adds the next game with a fresh seed. A tied game is replayed on
// the same scenario.
func (t *Tournament) nextGame(s *Series) {
	order := t.order(s)
	decided := 0
	var replay *Game
	for _, g := range s.Games {
		if g.Winner >= 0 {
			decided++
		}
	}
	if n := len(s.Games); n > 0 && s.Games[n-1].Winner < 0 {
		replay = s.Games[n-1]
	}
	g := &Game{Index: len(s.Games), Seed: randomSeed(), Winner: -1}
	if replay != nil {
		g.Scenario, g.Hash, g.TimeLimit = replay.Scenario, replay.Hash, replay.TimeLimit
	} else {
		p := order[decided%len(order)]
		g.Scenario, g.Hash, g.TimeLimit = p.Name, p.Hash, p.TimeLimit
	}
	s.Games = append(s.Games, g)
}

// GameReport is one game's result as the lobby host (or a player) reports it.
type GameReport struct {
	Index          int
	Scores         [2]float64
	Replays        []string
	HostValidated  bool
	PlayedScenario string
	Seed           string
	Result         *ResultRecord
}

const maxScore = 1e9

// ReportGame records the current game. When a player reaches the wins
// needed, the match waits for the opponent to confirm.
func (t *Tournament) ReportGame(a Actor, matchID string, r GameReport, now time.Time) error {
	if !a.SignedIn() {
		return ErrSignIn
	}
	m, s, err := t.match(matchID)
	if err != nil {
		return err
	}
	slot := t.slotOf(m, a)
	manager := t.CanManage(a)
	if slot < 0 && !manager {
		return errForbidden("You aren't playing in this match.")
	}
	if m.State != bracket.Ready || s == nil || s.StartedAt == nil {
		return errState("The match hasn't started.")
	}
	if t.openDispute(matchID) != nil {
		return errState("The match is disputed; an organiser will decide.")
	}
	if s.ReportedAt != nil {
		return errState("The result is waiting for confirmation.")
	}
	g := s.Current()
	if g == nil || g.Index != r.Index {
		return errInvalid("Report game %d.", len(s.Games))
	}
	if r.Result != nil {
		scores, err := r.Result.scores(m)
		if err != nil {
			return err
		}
		r.Scores = scores
		if r.Seed == "" {
			r.Seed = r.Result.Seed
		}
	}
	for _, v := range r.Scores {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > maxScore {
			return errInvalid("Scores must be between 0 and a billion.")
		}
	}
	reporter := ""
	if slot >= 0 {
		reporter = m.Slots[slot].Entrant
	}
	g.Scores = r.Scores
	g.HasScore = true
	g.ReportedBy = reporter
	at := now
	g.ReportedAt = &at
	g.HostValidated = r.HostValidated && reporter != "" && reporter == s.Host
	g.PlayedScenario = cleanLine(r.PlayedScenario, 160)
	if r.Seed != "" && r.Seed != strconv.FormatUint(g.Seed, 10) {
		g.addFlag("seed-mismatch")
	}
	g.Result = r.Result
	if r.Result != nil {
		for _, p := range r.Result.Players {
			if p.Disputed {
				g.addFlag("host-disputed")
			}
		}
	}
	g.Replays = g.Replays[:0]
	for _, id := range r.Replays {
		u := t.Uploads[id]
		if u == nil || u.Match != matchID || u.Game != g.Index {
			return errInvalid("Unknown replay %s.", id)
		}
		if !contains(g.Replays, id) {
			g.Replays = append(g.Replays, id)
		}
	}
	// Replays uploaded for this game before the report count too.
	for id, u := range t.Uploads {
		if u.Match == matchID && u.Game == g.Index && !contains(g.Replays, id) {
			g.Replays = append(g.Replays, id)
		}
	}
	t.verifyGame(m, s, g)
	switch {
	case r.Scores[0] > r.Scores[1]:
		g.Winner = 0
	case r.Scores[1] > r.Scores[0]:
		g.Winner = 1
	default:
		g.Winner = -1
		g.addFlag("tie")
	}
	wins := s.Wins()
	if wins[0] >= s.needed() || wins[1] >= s.needed() {
		s.ReportedBy = reporter
		s.ReportedAt = &at
		deadline := now.Add(time.Duration(t.Spec.Ruleset.ConfirmMinutes) * time.Minute)
		s.Deadline = &deadline
		// An organiser's report needs no confirmation.
		if manager && slot < 0 {
			return t.finalize(m, s, now)
		}
		return nil
	}
	if len(s.Games) >= 3*s.BestOf {
		s.addFlag("too-many-ties")
		return nil
	}
	t.nextGame(s)
	return nil
}

// scores maps a result record onto the match's two slots.
func (r *ResultRecord) scores(m *bracket.Match) ([2]float64, error) {
	var out [2]float64
	if r.Format != 1 {
		return out, errInvalid("Unsupported result format %d.", r.Format)
	}
	if len(r.Players) != 2 {
		return out, errInvalid("A tournament game result names its two players.")
	}
	seen := [2]bool{}
	for i := range r.Players {
		p := &r.Players[i]
		p.Key = cleanLine(p.Key, 64)
		p.Replay = strings.ToLower(cleanLine(p.Replay, 64))
		slot := -1
		for s := range m.Slots {
			if m.Slots[s].Entrant != "" && m.Slots[s].Entrant == p.Entrant {
				slot = s
			}
		}
		if slot < 0 || seen[slot] {
			return out, errInvalid("The result names someone who isn't in this match.")
		}
		if math.IsNaN(p.Score) || math.IsInf(p.Score, 0) || math.IsNaN(p.Accuracy) || math.IsInf(p.Accuracy, 0) {
			return out, errInvalid("Invalid score in the result.")
		}
		seen[slot] = true
		out[slot] = p.Score
	}
	r.Match = cleanLine(r.Match, 64)
	r.Mode = cleanLine(r.Mode, 32)
	r.SettingsKey = cleanLine(r.SettingsKey, 128)
	r.ScenarioHash = cleanLine(r.ScenarioHash, 128)
	r.Host = cleanLine(r.Host, 64)
	r.Winner = cleanLine(r.Winner, 64)
	r.Seed = cleanLine(r.Seed, 24)
	return out, nil
}

// blocked: the result can't be accepted without someone looking at it.
func (s *Series) blocked(t *Tournament) bool {
	for _, g := range s.Games {
		if !g.HasScore {
			continue
		}
		for _, f := range g.Flags {
			if f == "seed-mismatch" || f == "replay-suspicious" || f == "replay-rejected" || f == "host-disputed" {
				return true
			}
		}
		if t.Spec.Ruleset.RequireReplays && len(g.Replays) < 2 {
			return true
		}
	}
	return false
}

// Confirm: the opponent of the reporter accepts the result.
func (t *Tournament) Confirm(a Actor, matchID string, now time.Time) error {
	if !a.SignedIn() {
		return ErrSignIn
	}
	m, s, err := t.match(matchID)
	if err != nil {
		return err
	}
	if s == nil || s.ReportedAt == nil || m.State != bracket.Ready {
		return errState("There's no result to confirm.")
	}
	if t.openDispute(matchID) != nil {
		return errState("The match is disputed; an organiser will decide.")
	}
	slot := t.slotOf(m, a)
	switch {
	case slot >= 0 && m.Slots[slot].Entrant == s.ReportedBy:
		return errState("Your opponent confirms the result you reported.")
	case slot < 0 && !t.CanManage(a):
		return errForbidden("You aren't playing in this match.")
	}
	return t.finalize(m, s, now)
}

func (t *Tournament) finalize(m *bracket.Match, s *Series, now time.Time) error {
	wins := s.Wins()
	w := 0
	if wins[1] > wins[0] {
		w = 1
	}
	if err := t.Bracket.Report(m.ID, w, wins); err != nil {
		return errState("%v", err)
	}
	at := now
	s.CompletedAt = &at
	s.Deadline = nil
	t.syncSeries(now)
	return nil
}

func (t *Tournament) openDispute(matchID string) *Dispute {
	for _, d := range t.Disputes {
		if d.Match == matchID && d.Status == DisputeOpen {
			return d
		}
	}
	return nil
}

// OpenDispute: a player contests a result while it awaits confirmation or
// during play. The match stops until an organiser decides.
func (t *Tournament) OpenDispute(a Actor, matchID, reason string, now time.Time) (*Dispute, error) {
	if !a.SignedIn() {
		return nil, ErrSignIn
	}
	m, s, err := t.match(matchID)
	if err != nil {
		return nil, err
	}
	slot := t.slotOf(m, a)
	if slot < 0 {
		return nil, errForbidden("You aren't playing in this match.")
	}
	if m.State != bracket.Ready || s == nil || s.StartedAt == nil {
		return nil, errState("Only a match that's being played or awaiting confirmation can be disputed.")
	}
	if d := t.openDispute(matchID); d != nil {
		return d, nil
	}
	reason = clean(reason, MaxReasonLength)
	if len(reason) < 5 {
		return nil, errInvalid("Say what went wrong.")
	}
	d := &Dispute{ID: t.newID("d"), Match: matchID, OpenedBy: m.Slots[slot].Entrant, Reason: reason, Status: DisputeOpen, CreatedAt: now}
	t.Disputes = append(t.Disputes, d)
	s.Deadline = nil
	t.audit(now, a, "dispute", matchID)
	return d, nil
}

// Resolution of a dispute or override from an organiser.
type Decision struct {
	Decision DisputeDecision
	Winner   string // entrant id, for overturn
	Wins     [2]int
	Note     string
}

func (t *Tournament) ResolveDispute(a Actor, disputeID string, d Decision, now time.Time) (*Dispute, error) {
	if err := t.requireManager(a); err != nil {
		return nil, err
	}
	var dispute *Dispute
	for _, x := range t.Disputes {
		if x.ID == disputeID {
			dispute = x
		}
	}
	if dispute == nil {
		return nil, errNotFound("Unknown dispute.")
	}
	if dispute.Status != DisputeOpen {
		return nil, errState("This dispute is already resolved.")
	}
	m, s, err := t.match(dispute.Match)
	if err != nil {
		return nil, err
	}
	switch d.Decision {
	case DecisionUphold:
		if s == nil || s.ReportedAt == nil {
			return nil, errState("There's no reported result to uphold.")
		}
		dispute.Status = DisputeResolved // finalize needs no open dispute
		if err := t.finalize(m, s, now); err != nil {
			dispute.Status = DisputeOpen
			return nil, err
		}
	case DecisionOverturn:
		reset, err := t.override(m, d.Winner, d.Wins, "")
		if err != nil {
			return nil, err
		}
		for _, id := range reset {
			delete(t.Series, id)
		}
	case DecisionReplay:
		if s != nil {
			fresh := &Series{Match: m.ID, Entrants: s.Entrants, BestOf: s.BestOf, OpenedAt: now, ReadyAt: map[string]time.Time{}, CanHost: map[string]bool{}, Flags: append(s.Flags, "replayed")}
			deadline := now.Add(time.Duration(t.Spec.ReadyWindow) * time.Minute)
			fresh.Deadline = &deadline
			t.Series[m.ID] = fresh
		}
	default:
		return nil, errInvalid("Choose uphold, overturn or replay.")
	}
	at := now
	dispute.Status = DisputeResolved
	dispute.Decision = d.Decision
	dispute.Note = clean(d.Note, MaxReasonLength)
	dispute.ResolvedAt = &at
	ref := publicRef(a.UserRef)
	dispute.ResolvedBy = &ref
	t.syncSeries(now)
	t.audit(now, a, "resolve_dispute", dispute.ID+" "+string(d.Decision))
	return dispute, nil
}

// SetResult: an organiser decides a match (override, forfeit or no-show).
// Changing the winner of a finished match resets every later match that
// depended on it; the returned ids are those matches.
func (t *Tournament) SetResult(a Actor, matchID, winner string, wins [2]int, resolution, reason string, now time.Time) ([]string, error) {
	if err := t.requireManager(a); err != nil {
		return nil, err
	}
	m, _, err := t.match(matchID)
	if err != nil {
		return nil, err
	}
	if resolution == "" {
		resolution = "override"
	}
	if resolution != "override" && resolution != "forfeit" && resolution != "no_show" {
		return nil, errInvalid("Unknown resolution.")
	}
	reset, err := t.override(m, winner, wins, resolution)
	if err != nil {
		return nil, err
	}
	for _, id := range reset {
		delete(t.Series, id)
	}
	if d := t.openDispute(matchID); d != nil {
		at := now
		d.Status, d.Decision, d.ResolvedAt = DisputeResolved, DecisionOverturn, &at
		ref := publicRef(a.UserRef)
		d.ResolvedBy = &ref
		d.Note = "Result set by an organiser."
	}
	t.syncSeries(now)
	if s := t.Series[matchID]; s != nil {
		at := now
		s.CompletedAt = &at
		s.Deadline = nil
		if resolution != "override" {
			s.Resolution = resolution
		}
	}
	t.audit(now, a, "set_result", matchID+" "+winner+" "+cleanLine(reason, MaxReasonLength))
	if t.Status == Completed && !t.Bracket.Done() {
		t.Status = InProgress
	}
	return reset, nil
}

func (t *Tournament) override(m *bracket.Match, winner string, wins [2]int, resolution string) ([]string, error) {
	w := -1
	for i, s := range m.Slots {
		if s.Entrant != "" && s.Entrant == winner {
			w = i
		}
	}
	if w < 0 {
		return nil, errInvalid("The winner must be one of the match's players.")
	}
	if resolution == "forfeit" || resolution == "no_show" {
		if m.State == bracket.Ready {
			if err := t.Bracket.Forfeit(m.ID, m.Slots[1-w].Entrant); err != nil {
				return nil, errState("%v", err)
			}
			return nil, nil
		}
		wins = [2]int{}
	}
	if wins[w] < wins[1-w] || wins[0] < 0 || wins[1] < 0 || wins[0] > 9 || wins[1] > 9 {
		return nil, errInvalid("The winner needs at least as many game wins.")
	}
	reset, err := t.Bracket.SetResult(m.ID, w, wins)
	if err != nil {
		return nil, errState("%v", err)
	}
	return reset, nil
}
