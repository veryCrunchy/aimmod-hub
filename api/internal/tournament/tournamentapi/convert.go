package tournamentapi

import (
	"fmt"
	"strconv"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/tournament"
	"github.com/veryCrunchy/aimmod-hub/api/internal/tournament/bracket"
	pb "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/tournament/v1"
)

func ts(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func tsp(t *time.Time) string {
	if t == nil {
		return ""
	}
	return ts(*t)
}

func parseTime(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, fmt.Errorf("times are RFC 3339, e.g. 2026-05-01T18:00:00Z")
	}
	u := t.UTC()
	return &u, nil
}

var formatToPB = map[bracket.Format]pb.TournamentFormat{
	bracket.SingleElimination: pb.TournamentFormat_TOURNAMENT_FORMAT_SINGLE_ELIMINATION,
	bracket.DoubleElimination: pb.TournamentFormat_TOURNAMENT_FORMAT_DOUBLE_ELIMINATION,
	bracket.RoundRobin:        pb.TournamentFormat_TOURNAMENT_FORMAT_ROUND_ROBIN,
	bracket.Swiss:             pb.TournamentFormat_TOURNAMENT_FORMAT_SWISS,
}

var statusToPB = map[tournament.Status]pb.TournamentStatus{
	tournament.Draft:        pb.TournamentStatus_TOURNAMENT_STATUS_DRAFT,
	tournament.Registration: pb.TournamentStatus_TOURNAMENT_STATUS_REGISTRATION,
	tournament.CheckIn:      pb.TournamentStatus_TOURNAMENT_STATUS_CHECK_IN,
	tournament.InProgress:   pb.TournamentStatus_TOURNAMENT_STATUS_IN_PROGRESS,
	tournament.Completed:    pb.TournamentStatus_TOURNAMENT_STATUS_COMPLETED,
	tournament.Cancelled:    pb.TournamentStatus_TOURNAMENT_STATUS_CANCELLED,
}

var seedingToPB = map[tournament.SeedingMethod]pb.SeedingMethod{
	tournament.SeedManual:    pb.SeedingMethod_SEEDING_METHOD_MANUAL,
	tournament.SeedRandom:    pb.SeedingMethod_SEEDING_METHOD_RANDOM,
	tournament.SeedBenchmark: pb.SeedingMethod_SEEDING_METHOD_BENCHMARK_RANK,
	tournament.SeedScenario:  pb.SeedingMethod_SEEDING_METHOD_SCENARIO_PB,
}

var sideToPB = map[bracket.Side]pb.BracketSide{
	bracket.Winners:    pb.BracketSide_BRACKET_SIDE_WINNERS,
	bracket.Losers:     pb.BracketSide_BRACKET_SIDE_LOSERS,
	bracket.GrandFinal: pb.BracketSide_BRACKET_SIDE_GRAND_FINAL,
	bracket.ThirdPlace: pb.BracketSide_BRACKET_SIDE_THIRD_PLACE,
	bracket.Pool:       pb.BracketSide_BRACKET_SIDE_ROUND_ROBIN,
	bracket.SwissSide:  pb.BracketSide_BRACKET_SIDE_SWISS,
}

var entrantStatusToPB = map[tournament.EntrantStatus]pb.EntrantStatus{
	tournament.EntrantRegistered:   pb.EntrantStatus_ENTRANT_STATUS_REGISTERED,
	tournament.EntrantCheckedIn:    pb.EntrantStatus_ENTRANT_STATUS_CHECKED_IN,
	tournament.EntrantActive:       pb.EntrantStatus_ENTRANT_STATUS_ACTIVE,
	tournament.EntrantEliminated:   pb.EntrantStatus_ENTRANT_STATUS_ELIMINATED,
	tournament.EntrantDisqualified: pb.EntrantStatus_ENTRANT_STATUS_DISQUALIFIED,
	tournament.EntrantWithdrawn:    pb.EntrantStatus_ENTRANT_STATUS_WITHDRAWN,
	tournament.EntrantNoShow:       pb.EntrantStatus_ENTRANT_STATUS_NO_SHOW,
}

var stateToPB = map[tournament.SeriesState]pb.MatchState{
	tournament.StatePending:  pb.MatchState_MATCH_STATE_PENDING,
	tournament.StateReady:    pb.MatchState_MATCH_STATE_READY,
	tournament.StateVeto:     pb.MatchState_MATCH_STATE_VETO,
	tournament.StateLive:     pb.MatchState_MATCH_STATE_LIVE,
	tournament.StateAwaiting: pb.MatchState_MATCH_STATE_AWAITING_CONFIRMATION,
	tournament.StateDisputed: pb.MatchState_MATCH_STATE_DISPUTED,
	tournament.StateComplete: pb.MatchState_MATCH_STATE_COMPLETE,
	tournament.StateSkipped:  pb.MatchState_MATCH_STATE_SKIPPED,
}

var vetoToPB = map[tournament.VetoAction]pb.VetoAction{
	tournament.Ban:     pb.VetoAction_VETO_ACTION_BAN,
	tournament.Pick:    pb.VetoAction_VETO_ACTION_PICK,
	tournament.Decider: pb.VetoAction_VETO_ACTION_DECIDER,
}

var roleToPB = map[tournament.Role]pb.StaffRole{
	tournament.RoleOrganiser: pb.StaffRole_STAFF_ROLE_ORGANISER,
	tournament.RoleAdmin:     pb.StaffRole_STAFF_ROLE_ADMIN,
	tournament.RoleCaster:    pb.StaffRole_STAFF_ROLE_CASTER,
}

var replayStatusToPB = map[string]pb.ReplayStatus{
	tournament.ReplayPending:    pb.ReplayStatus_REPLAY_STATUS_PENDING,
	tournament.ReplayVerified:   pb.ReplayStatus_REPLAY_STATUS_VERIFIED,
	tournament.ReplaySuspicious: pb.ReplayStatus_REPLAY_STATUS_SUSPICIOUS,
	tournament.ReplayRejected:   pb.ReplayStatus_REPLAY_STATUS_REJECTED,
}

func reverse[K comparable, V comparable](m map[K]V) map[V]K {
	out := make(map[V]K, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

var (
	formatFromPB  = reverse(formatToPB)
	seedingFromPB = reverse(seedingToPB)
	sideFromPB    = reverse(sideToPB)
	roleFromPB    = reverse(roleToPB)
)

func userPB(u tournament.UserRef) *pb.UserRef {
	return &pb.UserRef{UserExternalId: u.ExternalID, Handle: u.Handle, DisplayName: u.DisplayName, AvatarUrl: u.AvatarURL}
}

func rulesetPB(r tournament.Ruleset) *pb.Ruleset {
	out := &pb.Ruleset{GameMode: r.GameMode, BestOf: int32(r.BestOf), CountdownSeconds: int32(r.Countdown), Spectators: r.Spectators,
		RequireReplays: r.RequireReplays, ConfirmMinutes: int32(r.ConfirmMinutes)}
	for _, rb := range r.RoundBestOf {
		out.RoundBestOf = append(out.RoundBestOf, &pb.RoundBestOf{Side: sideToPB[rb.Side], FromRound: int32(rb.FromRound), BestOf: int32(rb.BestOf)})
	}
	for _, p := range r.Pool {
		out.Pool = append(out.Pool, &pb.PoolScenario{Name: p.Name, Hash: p.Hash, TimeLimitSeconds: int32(p.TimeLimit)})
	}
	for _, v := range r.Veto {
		actor := pb.VetoActor_VETO_ACTOR_HIGHER_SEED
		if v.Actor == tournament.LowerSeed {
			actor = pb.VetoActor_VETO_ACTOR_LOWER_SEED
		}
		out.Veto = append(out.Veto, &pb.VetoStep{Action: vetoToPB[v.Action], Actor: actor})
	}
	return out
}

func rulesetFromPB(r *pb.Ruleset) tournament.Ruleset {
	if r == nil {
		return tournament.Ruleset{}
	}
	out := tournament.Ruleset{GameMode: r.GameMode, BestOf: int(r.BestOf), Countdown: int(r.CountdownSeconds), Spectators: r.Spectators,
		RequireReplays: r.RequireReplays, ConfirmMinutes: int(r.ConfirmMinutes)}
	for _, rb := range r.RoundBestOf {
		out.RoundBestOf = append(out.RoundBestOf, tournament.RoundBestOf{Side: sideFromPB[rb.Side], FromRound: int(rb.FromRound), BestOf: int(rb.BestOf)})
	}
	for _, p := range r.Pool {
		out.Pool = append(out.Pool, tournament.PoolScenario{Name: p.Name, Hash: p.Hash, TimeLimit: int(p.TimeLimitSeconds)})
	}
	for _, v := range r.Veto {
		step := tournament.VetoStep{Actor: tournament.HigherSeed}
		if v.Actor == pb.VetoActor_VETO_ACTOR_LOWER_SEED {
			step.Actor = tournament.LowerSeed
		}
		switch v.Action {
		case pb.VetoAction_VETO_ACTION_BAN:
			step.Action = tournament.Ban
		case pb.VetoAction_VETO_ACTION_PICK:
			step.Action = tournament.Pick
		default:
			step.Action = "invalid"
		}
		out.Veto = append(out.Veto, step)
	}
	return out
}

func specFromPB(s *pb.TournamentSpec) (tournament.Spec, error) {
	if s == nil {
		return tournament.Spec{}, fmt.Errorf("missing spec")
	}
	out := tournament.Spec{
		Name: s.Name, Description: s.Description, Format: formatFromPB[s.Format], InviteOnly: s.RegistrationMode == pb.RegistrationMode_REGISTRATION_MODE_INVITE_ONLY,
		MaxEntrants: int(s.MaxEntrants), Seeding: seedingFromPB[s.Seeding], SeedingSource: s.SeedingSource,
		ReadyWindow: int(s.ReadyWindowMinutes), NoShow: int(s.NoShowMinutes), Ruleset: rulesetFromPB(s.Ruleset), Prize: s.Prize,
	}
	if s.Format != pb.TournamentFormat_TOURNAMENT_FORMAT_UNSPECIFIED && out.Format == "" {
		out.Format = "invalid"
	}
	switch s.Scheduling {
	case pb.SchedulingMode_SCHEDULING_MODE_SCHEDULED:
		out.Scheduling = tournament.Scheduled
	case pb.SchedulingMode_SCHEDULING_MODE_READY_WHEN_ONLINE:
		out.Scheduling = tournament.ReadyWhenOnline
	}
	if o := s.Options; o != nil {
		out.Options = bracket.Options{ThirdPlace: o.ThirdPlaceMatch, GrandFinalReset: o.GrandFinalReset, Cycles: int(o.RoundRobinCycles), SwissRounds: int(o.SwissRounds)}
	}
	var err error
	if out.CheckInOpens, err = parseTime(s.CheckInOpensAt); err != nil {
		return out, err
	}
	if out.CheckInCloses, err = parseTime(s.CheckInClosesAt); err != nil {
		return out, err
	}
	if out.StartsAt, err = parseTime(s.StartsAt); err != nil {
		return out, err
	}
	return out, nil
}

func tournamentPB(t *tournament.Tournament) *pb.Tournament {
	s := t.Spec
	mode := pb.RegistrationMode_REGISTRATION_MODE_OPEN
	if s.InviteOnly {
		mode = pb.RegistrationMode_REGISTRATION_MODE_INVITE_ONLY
	}
	sched := pb.SchedulingMode_SCHEDULING_MODE_READY_WHEN_ONLINE
	if s.Scheduling == tournament.Scheduled {
		sched = pb.SchedulingMode_SCHEDULING_MODE_SCHEDULED
	}
	active, checked := 0, 0
	for _, e := range t.Entrants {
		if e.Active() {
			active++
			if e.CheckedInAt != nil {
				checked++
			}
		}
	}
	out := &pb.Tournament{
		Id: t.ID, Slug: t.Slug, Name: s.Name, Description: s.Description, Format: formatToPB[s.Format], Status: statusToPB[t.Status],
		RegistrationMode: mode, MaxEntrants: int32(s.MaxEntrants), Seeding: seedingToPB[s.Seeding], SeedingSource: s.SeedingSource,
		Scheduling: sched, CheckInOpensAt: tsp(s.CheckInOpens), CheckInClosesAt: tsp(s.CheckInCloses), StartsAt: tsp(s.StartsAt),
		ReadyWindowMinutes: int32(s.ReadyWindow), NoShowMinutes: int32(s.NoShow), Ruleset: rulesetPB(s.Ruleset),
		Options: &pb.TournamentOptions{ThirdPlaceMatch: s.Options.ThirdPlace, GrandFinalReset: s.Options.GrandFinalReset,
			RoundRobinCycles: int32(s.Options.Cycles), SwissRounds: int32(s.Options.SwissRounds)},
		Organiser: userPB(t.Organiser), EntrantCount: int32(active), CheckedInCount: int32(checked), Prize: s.Prize,
		CreatedAt: ts(t.CreatedAt), UpdatedAt: ts(t.UpdatedAt), Version: t.Version,
	}
	if t.Bracket != nil {
		out.ChampionEntrantId = t.Bracket.Champion()
		if t.Spec.Format == bracket.Swiss {
			out.Options.SwissRounds = int32(t.Bracket.Options.SwissRounds)
		}
	}
	return out
}

func places(t *tournament.Tournament) map[string]int {
	out := map[string]int{}
	if t.Bracket != nil {
		for _, s := range t.Bracket.Standings() {
			out[s.Entrant] = s.Place
		}
	}
	return out
}

func entrantPB(e *tournament.Entrant, place int) *pb.Entrant {
	if e == nil {
		return nil
	}
	out := &pb.Entrant{Id: e.ID, User: userPB(e.User), Seed: int32(e.Seed), Status: entrantStatusToPB[e.Status],
		RegisteredAt: ts(e.RegisteredAt), CheckedInAt: tsp(e.CheckedInAt), RatingLabel: e.RatingLabel, Place: int32(place)}
	if e.Rating != nil {
		out.Rating = *e.Rating
	}
	return out
}

// Label names a match for people: "Winners semi-final", "Losers round 3", "Grand final".
func Label(t *tournament.Tournament, m *bracket.Match) string {
	last := t.Bracket.Rounds(m.Side)
	switch m.Side {
	case bracket.Winners:
		name := "Winners"
		if t.Spec.Format == bracket.SingleElimination {
			name = ""
		}
		round := "round " + strconv.Itoa(m.Round)
		switch last - m.Round {
		case 0:
			round = "final"
		case 1:
			round = "semi-final"
		case 2:
			if last >= 4 {
				round = "quarter-final"
			}
		}
		if name == "" {
			return upperFirst(round)
		}
		return name + " " + round
	case bracket.Losers:
		if m.Round == last {
			return "Losers final"
		}
		return "Losers round " + strconv.Itoa(m.Round)
	case bracket.GrandFinal:
		if m.Round == 2 {
			return "Grand final reset"
		}
		return "Grand final"
	case bracket.ThirdPlace:
		return "Bronze match"
	case bracket.SwissSide:
		return "Swiss round " + strconv.Itoa(m.Round)
	}
	return "Round " + strconv.Itoa(m.Round)
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}

func slotPB(s bracket.Slot) *pb.MatchSlot {
	return &pb.MatchSlot{EntrantId: s.Entrant, Source: string(s.From.Kind), SourceMatchId: s.From.Match, Empty: s.State == bracket.SlotEmpty}
}

// matchPB converts a match. Seeds are shown to the players and private
// viewers once a game has started, and to everyone after the match.
func matchPB(t *tournament.Tournament, m *bracket.Match, viewer tournament.Actor) *pb.Match {
	state := t.State(m)
	out := &pb.Match{
		Id: m.ID, Side: sideToPB[m.Side], Round: int32(m.Round), Position: int32(m.Position), Label: Label(t, m),
		SlotA: slotPB(m.Slots[0]), SlotB: slotPB(m.Slots[1]), State: stateToPB[state], BestOf: int32(t.BestOf(m)),
		WinnerEntrantId: m.WinnerID(), CurrentGame: -1,
	}
	if m.WinnerTo != nil {
		out.WinnerToMatchId = m.WinnerTo.Match
	}
	if m.LoserTo != nil {
		out.LoserToMatchId = m.LoserTo.Match
	}
	switch m.Resolution {
	case bracket.Played:
		out.Resolution = pb.Resolution_RESOLUTION_PLAYED
	case bracket.Walkover:
		out.Resolution = pb.Resolution_RESOLUTION_WALKOVER
	case bracket.Forfeit:
		out.Resolution = pb.Resolution_RESOLUTION_FORFEIT
	case bracket.Override:
		out.Resolution = pb.Resolution_RESOLUTION_ADMIN_OVERRIDE
	}
	if m.State == bracket.Complete {
		out.WinsA, out.WinsB = int32(m.Score[0]), int32(m.Score[1])
	}
	s := t.Series[m.ID]
	if s == nil {
		return out
	}
	if s.Resolution == "no_show" {
		out.Resolution = pb.Resolution_RESOLUTION_NO_SHOW
	}
	if m.State != bracket.Complete {
		w := s.Wins()
		out.WinsA, out.WinsB = int32(w[0]), int32(w[1])
	}
	out.BestOf = int32(s.BestOf)
	out.HostEntrantId = s.Host
	out.ScheduledAt = tsp(s.ScheduledAt)
	out.Deadline = tsp(s.Deadline)
	out.ReportedBy = s.ReportedBy
	out.Flags = append(out.Flags, s.Flags...)
	for id := range s.ReadyAt {
		out.ReadyEntrantIds = append(out.ReadyEntrantIds, id)
	}
	sortStrings(out.ReadyEntrantIds)
	for _, d := range t.Disputes {
		if d.Match == m.ID && d.Status == tournament.DisputeOpen {
			out.DisputeId = d.ID
		}
	}
	turn, action := t.VetoTurn(m)
	out.VetoTurnEntrantId, out.VetoTurnAction = turn, vetoToPB[action]
	for _, v := range s.Veto {
		out.Veto = append(out.Veto, &pb.VetoEntry{Step: int32(v.Step), Action: vetoToPB[v.Action], EntrantId: v.Entrant, Scenario: v.Scenario, At: ts(v.At)})
	}
	insider := t.CanSeePrivate(viewer) || slotOfViewer(t, m, viewer) >= 0
	for _, g := range s.Games {
		pg := &pb.Game{Index: int32(g.Index), Scenario: g.Scenario, ScenarioHash: g.Hash, TimeLimitSeconds: int32(g.TimeLimit),
			HasScore: g.HasScore, Winner: int32(g.Winner), ReportedBy: g.ReportedBy, ReportedAt: tsp(g.ReportedAt),
			HostValidated: g.HostValidated, Flags: append([]string(nil), g.Flags...), PlayedScenario: g.PlayedScenario}
		if g.HasScore {
			pg.ScoreA, pg.ScoreB = g.Scores[0], g.Scores[1]
		}
		if insider || state == tournament.StateComplete {
			pg.Seed = strconv.FormatUint(g.Seed, 10)
		}
		pg.Result = resultToPB(g.Result, insider || state == tournament.StateComplete)
		for _, id := range g.Replays {
			if u := t.Uploads[id]; u != nil {
				pg.Replays = append(pg.Replays, &pb.ReplayRef{Id: u.ID, EntrantId: u.Entrant, Sha256: u.SHA256, ByteSize: u.Size,
					Status: replayStatusToPB[u.Status], Checks: append([]string(nil), u.Checks...), Scenario: u.Info.Scenario,
					Score: u.Info.Score, DurationSeconds: u.Info.Duration})
			}
		}
		out.Games = append(out.Games, pg)
	}
	if g := s.Current(); g != nil && s.ReportedAt == nil {
		out.CurrentGame = int32(g.Index)
	}
	return out
}

func resultFromPB(r *pb.GameResult) *tournament.ResultRecord {
	if r == nil {
		return nil
	}
	out := &tournament.ResultRecord{Format: int(r.Format), Match: r.Match, Mode: r.Mode, SettingsKey: r.SettingsKey, ScenarioHash: r.ScenarioHash,
		StartedAt: r.StartedAt, EndedAt: r.EndedAt, Host: r.Host, Winner: r.Winner, Seed: r.Seed}
	for _, p := range r.Players {
		rp := tournament.ResultPlayer{Key: p.Key, Entrant: p.EntrantId, Team: int(p.Team), Place: int(p.Place), Score: p.Score, Frags: int(p.Frags),
			Deaths: int(p.Deaths), Claims: int(p.Claims), Rejected: int(p.Rejected), Disputed: p.Disputed, Replay: p.Replay, Accuracy: p.Accuracy}
		if p.TrackPercent != nil {
			v := *p.TrackPercent
			rp.TrackPercent = &v
		}
		out.Players = append(out.Players, rp)
	}
	return out
}

// resultToPB: the seed inside the record follows the same visibility as the game's seed.
func resultToPB(r *tournament.ResultRecord, showSeed bool) *pb.GameResult {
	if r == nil {
		return nil
	}
	out := &pb.GameResult{Format: int32(r.Format), Match: r.Match, Mode: r.Mode, SettingsKey: r.SettingsKey, ScenarioHash: r.ScenarioHash,
		StartedAt: r.StartedAt, EndedAt: r.EndedAt, Host: r.Host, Winner: r.Winner}
	if showSeed {
		out.Seed = r.Seed
	}
	for _, p := range r.Players {
		out.Players = append(out.Players, &pb.GameResultPlayer{Key: p.Key, EntrantId: p.Entrant, Team: int32(p.Team), Place: int32(p.Place), Score: p.Score,
			Frags: int32(p.Frags), Deaths: int32(p.Deaths), Claims: int32(p.Claims), Rejected: int32(p.Rejected), TrackPercent: p.TrackPercent,
			Disputed: p.Disputed, Replay: p.Replay, Accuracy: p.Accuracy})
	}
	return out
}

func slotOfViewer(t *tournament.Tournament, m *bracket.Match, a tournament.Actor) int {
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

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func standingsPB(t *tournament.Tournament) []*pb.Standing {
	if t.Bracket == nil {
		return nil
	}
	var out []*pb.Standing
	for _, s := range t.Bracket.Standings() {
		out = append(out, &pb.Standing{EntrantId: s.Entrant, Place: int32(s.Place), Wins: int32(s.Wins), Losses: int32(s.Losses),
			GamesWon: int32(s.GamesWon), GamesLost: int32(s.GamesLost), Points: int32(s.Points), Buchholz: int32(s.Buchholz), Eliminated: s.Eliminated})
	}
	return out
}

func disputePB(d *tournament.Dispute) *pb.Dispute {
	if d == nil {
		return nil
	}
	out := &pb.Dispute{Id: d.ID, MatchId: d.Match, OpenedByEntrantId: d.OpenedBy, Reason: d.Reason, ResolutionNote: d.Note,
		CreatedAt: ts(d.CreatedAt), ResolvedAt: tsp(d.ResolvedAt), Status: pb.DisputeStatus_DISPUTE_STATUS_OPEN}
	if d.Status == tournament.DisputeResolved {
		out.Status = pb.DisputeStatus_DISPUTE_STATUS_RESOLVED
	}
	switch d.Decision {
	case tournament.DecisionUphold:
		out.Decision = pb.DisputeDecision_DISPUTE_DECISION_UPHOLD
	case tournament.DecisionOverturn:
		out.Decision = pb.DisputeDecision_DISPUTE_DECISION_OVERTURN
	case tournament.DecisionReplay:
		out.Decision = pb.DisputeDecision_DISPUTE_DECISION_REPLAY
	}
	if d.ResolvedBy != nil {
		out.ResolvedBy = userPB(*d.ResolvedBy)
	}
	return out
}

func viewerPB(t *tournament.Tournament, a tournament.Actor, now time.Time) *pb.Viewer {
	v := &pb.Viewer{SignedIn: a.SignedIn(), Role: roleToPB[t.RoleOf(a)], CanManage: t.CanManage(a), Invited: t.Invited(a.Handle)}
	e := t.EntrantOf(a.UserID)
	if e != nil && e.Active() {
		v.EntrantId = e.ID
		v.CanCheckIn = t.CheckInOpen(now) && e.CheckedInAt == nil
	}
	open := t.Status == tournament.Registration || t.Status == tournament.CheckIn
	v.CanRegister = a.SignedIn() && open && (e == nil || !e.Active()) && (!t.Spec.InviteOnly || v.Invited || v.CanManage)
	return v
}
