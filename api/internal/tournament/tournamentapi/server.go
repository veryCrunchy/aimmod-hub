// Package tournamentapi serves aimmod.tournament.v1.TournamentService and
// the replay upload endpoint on top of package tournament.
package tournamentapi

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	"github.com/veryCrunchy/aimmod-hub/api/internal/tournament"
	"github.com/veryCrunchy/aimmod-hub/api/internal/tournament/bracket"
	pb "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/tournament/v1"
)

// Store is what the API needs from persistence (store.Store implements it).
type Store interface {
	CreateTournament(context.Context, *tournament.Tournament) error
	LoadTournament(context.Context, string) (*tournament.Tournament, error)
	SaveTournament(context.Context, *tournament.Tournament, time.Time) error
	ListTournaments(context.Context, store.TournamentFilter) ([]*tournament.Tournament, string, error)
	ScenarioPersonalBests(context.Context, []int64, string) (map[int64]float64, error)
	TournamentUserByHandle(context.Context, string) (store.TournamentUser, error)
}

// BenchmarkRanks rates a player on a KovaaK's benchmark: a higher value is
// a better rank. ok is false when the player has no rank there.
type BenchmarkRanks interface {
	Rank(ctx context.Context, benchmarkID uint32, steamID string) (value float64, label string, ok bool, err error)
}

// Authenticator turns request headers into the caller (zero Actor when anonymous).
type Authenticator func(ctx context.Context, header http.Header) (tournament.Actor, error)

// CreatorPolicy decides who may create tournaments.
type CreatorPolicy string

const (
	// CreatorsAdmin: only the Hub administrator (the default).
	CreatorsAdmin CreatorPolicy = "admin"
	// CreatorsVerified: any signed-in user with a verified linked account.
	CreatorsVerified CreatorPolicy = "verified"
	// CreatorsSignedIn: anyone signed in.
	CreatorsSignedIn CreatorPolicy = "signed-in"
)

type Config struct {
	Store      Store
	Auth       Authenticator
	Benchmarks BenchmarkRanks
	Creators   CreatorPolicy
	Now        func() time.Time
	LiveTTL    time.Duration
	IsVerified func(tournament.Actor) bool
}

type Server struct {
	cfg  Config
	live *liveStore
}

func New(cfg Config) *Server {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.LiveTTL <= 0 {
		cfg.LiveTTL = 20 * time.Second
	}
	if cfg.Creators == "" {
		cfg.Creators = CreatorsAdmin
	}
	return &Server{cfg: cfg, live: &liveStore{ttl: cfg.LiveTTL, items: map[string]liveEntry{}, now: cfg.Now}}
}

func (s *Server) now() time.Time { return s.cfg.Now().UTC().Truncate(time.Millisecond) }

func (s *Server) actor(ctx context.Context, h http.Header) (tournament.Actor, error) {
	if s.cfg.Auth == nil {
		return tournament.Actor{}, nil
	}
	a, err := s.cfg.Auth(ctx, h)
	if err != nil {
		return tournament.Actor{}, connect.NewError(connect.CodeUnauthenticated, errors.New("Your AimMod Hub sign-in has expired. Sign in again."))
	}
	return a, nil
}

func (s *Server) signedIn(ctx context.Context, h http.Header) (tournament.Actor, error) {
	a, err := s.actor(ctx, h)
	if err != nil {
		return a, err
	}
	if !a.SignedIn() {
		return a, toConnect(tournament.ErrSignIn)
	}
	return a, nil
}

// toConnect maps domain and store errors to Connect codes. Messages are
// written for players; internal errors stay generic.
func toConnect(err error) error {
	if err == nil {
		return nil
	}
	var ce *connect.Error
	if errors.As(err, &ce) {
		return err
	}
	switch {
	case errors.Is(err, store.ErrTournamentNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("Tournament not found."))
	case errors.Is(err, store.ErrTournamentConflict):
		return connect.NewError(connect.CodeAborted, errors.New("The tournament changed while you were editing. Reload and try again."))
	}
	switch tournament.KindOf(err) {
	case tournament.KindInvalid:
		return connect.NewError(connect.CodeInvalidArgument, err)
	case tournament.KindForbidden:
		return connect.NewError(connect.CodePermissionDenied, err)
	case tournament.KindState:
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case tournament.KindNotFound:
		return connect.NewError(connect.CodeNotFound, err)
	case tournament.KindUnauthenticated:
		return connect.NewError(connect.CodeUnauthenticated, err)
	}
	return connect.NewError(connect.CodeInternal, errors.New("Something went wrong on AimMod Hub."))
}

// load reads a tournament and applies time-based changes, saving them when
// possible (a concurrent save simply wins; the next read applies them again).
func (s *Server) load(ctx context.Context, id string) (*tournament.Tournament, time.Time, error) {
	if strings.TrimSpace(id) == "" {
		return nil, time.Time{}, connect.NewError(connect.CodeInvalidArgument, errors.New("Which tournament?"))
	}
	t, err := s.cfg.Store.LoadTournament(ctx, id)
	if err != nil {
		return nil, time.Time{}, toConnect(err)
	}
	now := s.now()
	if t.Tick(now) {
		if err := s.cfg.Store.SaveTournament(ctx, t, now); err != nil && !errors.Is(err, store.ErrTournamentConflict) {
			return nil, now, toConnect(err)
		}
	}
	return t, now, nil
}

// mutate loads, ticks, applies fn and saves, retrying on version conflicts.
func (s *Server) mutate(ctx context.Context, id string, fn func(t *tournament.Tournament, now time.Time) error) (*tournament.Tournament, time.Time, error) {
	if strings.TrimSpace(id) == "" {
		return nil, time.Time{}, connect.NewError(connect.CodeInvalidArgument, errors.New("Which tournament?"))
	}
	for attempt := 0; ; attempt++ {
		t, err := s.cfg.Store.LoadTournament(ctx, id)
		if err != nil {
			return nil, time.Time{}, toConnect(err)
		}
		now := s.now()
		t.Tick(now)
		if err := fn(t, now); err != nil {
			return nil, now, toConnect(err)
		}
		t.Tick(now)
		err = s.cfg.Store.SaveTournament(ctx, t, now)
		if errors.Is(err, store.ErrTournamentConflict) && attempt < 3 {
			continue
		}
		if err != nil {
			return nil, now, toConnect(err)
		}
		return t, now, nil
	}
}

func newID() string {
	var b [10]byte
	_, _ = rand.Read(b[:])
	return "t_" + hex.EncodeToString(b[:])
}

func draw() int64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return int64(binary.LittleEndian.Uint64(b[:]) >> 1)
}

func (s *Server) canCreate(a tournament.Actor) bool {
	switch s.cfg.Creators {
	case CreatorsSignedIn:
		return a.SignedIn()
	case CreatorsVerified:
		return a.HubAdmin || a.SignedIn() && s.cfg.IsVerified != nil && s.cfg.IsVerified(a)
	}
	return a.HubAdmin
}

// ratings fetches seeding stats for the field.
func (s *Server) ratings(ctx context.Context, t *tournament.Tournament, method tournament.SeedingMethod, source string) (map[int64]tournament.Rating, error) {
	out := map[int64]tournament.Rating{}
	var field []*tournament.Entrant
	for _, e := range t.Entrants {
		if e.Active() {
			field = append(field, e)
		}
	}
	switch method {
	case tournament.SeedScenario:
		ids := make([]int64, 0, len(field))
		for _, e := range field {
			ids = append(ids, e.User.UserID)
		}
		best, err := s.cfg.Store.ScenarioPersonalBests(ctx, ids, source)
		if err != nil {
			return nil, err
		}
		for id, v := range best {
			out[id] = tournament.Rating{Value: v, Label: formatScore(v)}
		}
	case tournament.SeedBenchmark:
		if s.cfg.Benchmarks == nil {
			return out, nil
		}
		var id uint32
		for _, c := range source {
			if c < '0' || c > '9' || id > 1<<28 {
				return nil, &tournament.Error{Kind: tournament.KindInvalid, Message: "Benchmark seeding needs a benchmark id."}
			}
			id = id*10 + uint32(c-'0')
		}
		for _, e := range field {
			if e.User.SteamID == "" {
				continue
			}
			v, label, ok, err := s.cfg.Benchmarks.Rank(ctx, id, e.User.SteamID)
			if err != nil {
				// One unavailable profile seeds that player last rather than failing the start.
				continue
			}
			if ok {
				out[e.User.UserID] = tournament.Rating{Value: v, Label: label}
			}
		}
	}
	return out, nil
}

func formatScore(v float64) string {
	return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64)
}

// ---- RPCs ------------------------------------------------------------

func (s *Server) ListTournaments(ctx context.Context, req *connect.Request[pb.ListTournamentsRequest]) (*connect.Response[pb.ListTournamentsResponse], error) {
	a, err := s.actor(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	f := store.TournamentFilter{Limit: int(req.Msg.Limit), Cursor: req.Msg.Cursor}
	for _, st := range req.Msg.Statuses {
		for k, v := range statusToPB {
			if v == st {
				f.Statuses = append(f.Statuses, string(k))
			}
		}
	}
	if req.Msg.Mine {
		if !a.SignedIn() {
			return nil, toConnect(tournament.ErrSignIn)
		}
		f.MemberUserID = a.UserID
	}
	list, next, err := s.cfg.Store.ListTournaments(ctx, f)
	if err != nil {
		if strings.Contains(err.Error(), "invalid cursor") {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("Invalid page."))
		}
		return nil, toConnect(err)
	}
	resp := &pb.ListTournamentsResponse{NextCursor: next}
	now := s.now()
	for _, t := range list {
		t.Tick(now)
		item := &pb.TournamentSummary{Tournament: tournamentPB(t)}
		if e := t.EntrantOf(a.UserID); e != nil {
			item.Self = entrantPB(e, places(t)[e.ID])
		}
		resp.Tournaments = append(resp.Tournaments, item)
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) GetTournament(ctx context.Context, req *connect.Request[pb.GetTournamentRequest]) (*connect.Response[pb.GetTournamentResponse], error) {
	a, err := s.actor(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	t, now, err := s.load(ctx, req.Msg.Tournament)
	if err != nil {
		return nil, err
	}
	if t.Status == tournament.Draft && !t.CanManage(a) {
		return nil, toConnect(store.ErrTournamentNotFound)
	}
	return connect.NewResponse(s.full(t, a, now)), nil
}

func (s *Server) full(t *tournament.Tournament, a tournament.Actor, now time.Time) *pb.GetTournamentResponse {
	resp := &pb.GetTournamentResponse{Tournament: tournamentPB(t), Viewer: viewerPB(t, a, now), Standings: standingsPB(t)}
	pl := places(t)
	for _, e := range t.Entrants {
		resp.Entrants = append(resp.Entrants, entrantPB(e, pl[e.ID]))
	}
	if t.Bracket != nil {
		for _, m := range t.Bracket.Matches {
			resp.Matches = append(resp.Matches, matchPB(t, m, a))
		}
	}
	for _, d := range t.Disputes {
		resp.Disputes = append(resp.Disputes, disputePB(d))
	}
	resp.Staff = append(resp.Staff, &pb.Staff{User: userPB(t.Organiser), Role: pb.StaffRole_STAFF_ROLE_ORGANISER})
	for _, st := range t.Staff {
		resp.Staff = append(resp.Staff, &pb.Staff{User: userPB(st.User), Role: roleToPB[st.Role]})
	}
	if t.CanManage(a) {
		for i := len(t.Audit) - 1; i >= 0 && len(resp.Audit) < 100; i-- {
			e := t.Audit[i]
			resp.Audit = append(resp.Audit, &pb.AuditEntry{At: ts(e.At), Actor: userPB(e.Actor), Action: e.Action, Detail: e.Detail})
		}
		resp.InvitedHandles = append(resp.InvitedHandles, t.Invites...)
	}
	return resp
}

func (s *Server) CreateTournament(ctx context.Context, req *connect.Request[pb.CreateTournamentRequest]) (*connect.Response[pb.CreateTournamentResponse], error) {
	a, err := s.signedIn(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	if !s.canCreate(a) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("Creating tournaments isn't open to your account yet."))
	}
	spec, err := specFromPB(req.Msg.Spec)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	t, err := tournament.New(newID(), spec, a, s.now())
	if err != nil {
		return nil, toConnect(err)
	}
	if err := s.cfg.Store.CreateTournament(ctx, t); err != nil {
		return nil, toConnect(err)
	}
	return connect.NewResponse(&pb.CreateTournamentResponse{Tournament: tournamentPB(t)}), nil
}

func (s *Server) UpdateTournament(ctx context.Context, req *connect.Request[pb.UpdateTournamentRequest]) (*connect.Response[pb.UpdateTournamentResponse], error) {
	a, err := s.signedIn(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	spec, err := specFromPB(req.Msg.Spec)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	t, _, err := s.mutate(ctx, req.Msg.TournamentId, func(t *tournament.Tournament, now time.Time) error {
		if req.Msg.Version != 0 && req.Msg.Version != t.Version {
			return store.ErrTournamentConflict
		}
		return t.Update(a, spec, now)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UpdateTournamentResponse{Tournament: tournamentPB(t)}), nil
}

func (s *Server) AdvanceTournament(ctx context.Context, req *connect.Request[pb.AdvanceTournamentRequest]) (*connect.Response[pb.AdvanceTournamentResponse], error) {
	a, err := s.signedIn(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	action := map[pb.AdvanceAction]tournament.AdvanceAction{
		pb.AdvanceAction_ADVANCE_ACTION_OPEN_REGISTRATION: tournament.OpenRegistration,
		pb.AdvanceAction_ADVANCE_ACTION_OPEN_CHECK_IN:     tournament.OpenCheckIn,
		pb.AdvanceAction_ADVANCE_ACTION_START:             tournament.Start,
		pb.AdvanceAction_ADVANCE_ACTION_CANCEL:            tournament.Cancel,
	}[req.Msg.Action]
	if action == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("Unknown action."))
	}
	var ratings map[int64]tournament.Rating
	if action == tournament.Start {
		pre, err := s.cfg.Store.LoadTournament(ctx, req.Msg.TournamentId)
		if err != nil {
			return nil, toConnect(err)
		}
		if !pre.CanManage(a) {
			return nil, toConnect(&tournament.Error{Kind: tournament.KindForbidden, Message: "Only the tournament's organisers can do that."})
		}
		if ratings, err = s.ratings(ctx, pre, pre.Spec.Seeding, pre.Spec.SeedingSource); err != nil {
			return nil, toConnect(err)
		}
	}
	d := draw()
	t, _, err := s.mutate(ctx, req.Msg.TournamentId, func(t *tournament.Tournament, now time.Time) error {
		return t.Advance(a, action, ratings, d, now)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.AdvanceTournamentResponse{Tournament: tournamentPB(t)}), nil
}

func (s *Server) Register(ctx context.Context, req *connect.Request[pb.RegisterRequest]) (*connect.Response[pb.RegisterResponse], error) {
	a, err := s.signedIn(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	var e *tournament.Entrant
	_, _, err = s.mutate(ctx, req.Msg.TournamentId, func(t *tournament.Tournament, now time.Time) error {
		var err error
		e, err = t.Register(a, now)
		return err
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.RegisterResponse{Entrant: entrantPB(e, 0)}), nil
}

func (s *Server) Withdraw(ctx context.Context, req *connect.Request[pb.WithdrawRequest]) (*connect.Response[pb.WithdrawResponse], error) {
	a, err := s.signedIn(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	if _, _, err := s.mutate(ctx, req.Msg.TournamentId, func(t *tournament.Tournament, now time.Time) error {
		return t.Withdraw(a, now)
	}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.WithdrawResponse{}), nil
}

func (s *Server) CheckIn(ctx context.Context, req *connect.Request[pb.CheckInRequest]) (*connect.Response[pb.CheckInResponse], error) {
	a, err := s.signedIn(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	var e *tournament.Entrant
	if _, _, err := s.mutate(ctx, req.Msg.TournamentId, func(t *tournament.Tournament, now time.Time) error {
		var err error
		e, err = t.CheckIn(a, now)
		return err
	}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CheckInResponse{Entrant: entrantPB(e, 0)}), nil
}

func (s *Server) InviteEntrants(ctx context.Context, req *connect.Request[pb.InviteEntrantsRequest]) (*connect.Response[pb.InviteEntrantsResponse], error) {
	a, err := s.signedIn(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	if len(req.Msg.Handles) > 100 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("Invite at most 100 players at once."))
	}
	t, _, err := s.mutate(ctx, req.Msg.TournamentId, func(t *tournament.Tournament, now time.Time) error {
		return t.SetInvites(a, req.Msg.Handles, req.Msg.Remove, now)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.InviteEntrantsResponse{InvitedHandles: t.Invites}), nil
}

func (s *Server) entrants(t *tournament.Tournament) []*pb.Entrant {
	var out []*pb.Entrant
	pl := places(t)
	for _, e := range t.Entrants {
		out = append(out, entrantPB(e, pl[e.ID]))
	}
	return out
}

func (s *Server) SetSeeds(ctx context.Context, req *connect.Request[pb.SetSeedsRequest]) (*connect.Response[pb.SetSeedsResponse], error) {
	a, err := s.signedIn(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	t, _, err := s.mutate(ctx, req.Msg.TournamentId, func(t *tournament.Tournament, now time.Time) error {
		return t.SetSeeds(a, req.Msg.EntrantIds, now)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.SetSeedsResponse{Entrants: s.entrants(t)}), nil
}

func (s *Server) Reseed(ctx context.Context, req *connect.Request[pb.ReseedRequest]) (*connect.Response[pb.ReseedResponse], error) {
	a, err := s.signedIn(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	method := seedingFromPB[req.Msg.Method]
	if method == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("Choose a seeding method."))
	}
	pre, err := s.cfg.Store.LoadTournament(ctx, req.Msg.TournamentId)
	if err != nil {
		return nil, toConnect(err)
	}
	if !pre.CanManage(a) {
		return nil, toConnect(&tournament.Error{Kind: tournament.KindForbidden, Message: "Only the tournament's organisers can do that."})
	}
	ratings, err := s.ratings(ctx, pre, method, req.Msg.SeedingSource)
	if err != nil {
		return nil, toConnect(err)
	}
	d := draw()
	t, _, err := s.mutate(ctx, req.Msg.TournamentId, func(t *tournament.Tournament, now time.Time) error {
		return t.Reseed(a, method, req.Msg.SeedingSource, ratings, d, now)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ReseedResponse{Entrants: s.entrants(t)}), nil
}

func (s *Server) ListMyMatches(ctx context.Context, req *connect.Request[pb.ListMyMatchesRequest]) (*connect.Response[pb.ListMyMatchesResponse], error) {
	a, err := s.signedIn(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	list, _, err := s.cfg.Store.ListTournaments(ctx, store.TournamentFilter{MemberUserID: a.UserID, Limit: 50,
		Statuses: []string{string(tournament.Registration), string(tournament.CheckIn), string(tournament.InProgress)}})
	if err != nil {
		return nil, toConnect(err)
	}
	resp := &pb.ListMyMatchesResponse{}
	now := s.now()
	for _, t := range list {
		if t.Tick(now) {
			_ = s.cfg.Store.SaveTournament(ctx, t, now)
		}
		self := t.EntrantOf(a.UserID)
		if self == nil || !self.Active() {
			continue
		}
		if t.CheckInOpen(now) && self.CheckedInAt == nil {
			resp.CheckIn = append(resp.CheckIn, &pb.TournamentSummary{Tournament: tournamentPB(t), Self: entrantPB(self, 0)})
		}
		if t.Status != tournament.InProgress || t.Bracket == nil {
			continue
		}
		for _, m := range t.Bracket.Matches {
			if m.State != bracket.Ready || !m.Has(self.ID) {
				continue
			}
			resp.Matches = append(resp.Matches, s.myMatch(t, m, a, self))
		}
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) myMatch(t *tournament.Tournament, m *bracket.Match, a tournament.Actor, self *tournament.Entrant) *pb.MyMatch {
	opp := m.Slots[0].Entrant
	if opp == self.ID {
		opp = m.Slots[1].Entrant
	}
	other := t.Entrant(opp)
	mm := &pb.MyMatch{TournamentId: t.ID, TournamentName: t.Spec.Name, Match: matchPB(t, m, a), Self: entrantPB(self, 0),
		Opponent: entrantPB(other, 0), Ruleset: rulesetPB(t.Spec.Ruleset)}
	if t.Spec.Scheduling == tournament.Scheduled {
		mm.Scheduling = pb.SchedulingMode_SCHEDULING_MODE_SCHEDULED
	} else {
		mm.Scheduling = pb.SchedulingMode_SCHEDULING_MODE_READY_WHEN_ONLINE
	}
	if sr := t.Series[m.ID]; sr != nil && sr.Host != "" {
		mm.Host = sr.Host == self.ID
	} else {
		mm.Host = self.Seed > 0 && (other == nil || other.Seed == 0 || self.Seed < other.Seed)
	}
	if other != nil {
		mm.OpponentSteamId = other.User.SteamID
	}
	if l := s.live.get(t.ID, m.ID); l != nil {
		mm.LobbyToken = l.LobbyToken
	}
	return mm
}

func (s *Server) GetMatch(ctx context.Context, req *connect.Request[pb.GetMatchRequest]) (*connect.Response[pb.GetMatchResponse], error) {
	a, err := s.actor(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	t, _, err := s.load(ctx, req.Msg.TournamentId)
	if err != nil {
		return nil, err
	}
	if t.Bracket == nil || t.Bracket.Match(req.Msg.MatchId) == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("Match not found."))
	}
	return connect.NewResponse(s.matchResponse(t, req.Msg.MatchId, a)), nil
}

func (s *Server) matchResponse(t *tournament.Tournament, id string, a tournament.Actor) *pb.GetMatchResponse {
	m := t.Bracket.Match(id)
	pl := places(t)
	resp := &pb.GetMatchResponse{Tournament: tournamentPB(t), Match: matchPB(t, m, a), Viewer: viewerPB(t, a, s.now()),
		EntrantA: entrantPB(t.Entrant(m.Slots[0].Entrant), pl[m.Slots[0].Entrant]),
		EntrantB: entrantPB(t.Entrant(m.Slots[1].Entrant), pl[m.Slots[1].Entrant])}
	resp.Live = publicLive(s.live.get(t.ID, id), t.CanSeePrivate(a) || slotOfViewer(t, m, a) >= 0)
	for _, d := range t.Disputes {
		if d.Match == id && (d.Status == tournament.DisputeOpen || resp.Dispute == nil) {
			resp.Dispute = disputePB(d)
		}
	}
	return resp
}

func (s *Server) matchMutation(ctx context.Context, h http.Header, tid, mid string, fn func(t *tournament.Tournament, a tournament.Actor, now time.Time) error) (*pb.Match, error) {
	a, err := s.signedIn(ctx, h)
	if err != nil {
		return nil, err
	}
	t, _, err := s.mutate(ctx, tid, func(t *tournament.Tournament, now time.Time) error { return fn(t, a, now) })
	if err != nil {
		return nil, err
	}
	m := t.Bracket.Match(mid)
	if m == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("Match not found."))
	}
	return matchPB(t, m, a), nil
}

func (s *Server) MarkReady(ctx context.Context, req *connect.Request[pb.MarkReadyRequest]) (*connect.Response[pb.MarkReadyResponse], error) {
	m, err := s.matchMutation(ctx, req.Header(), req.Msg.TournamentId, req.Msg.MatchId, func(t *tournament.Tournament, a tournament.Actor, now time.Time) error {
		return t.MarkReady(a, req.Msg.MatchId, req.Msg.Ready, req.Msg.CanHost, now)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.MarkReadyResponse{Match: m}), nil
}

func (s *Server) SubmitVeto(ctx context.Context, req *connect.Request[pb.SubmitVetoRequest]) (*connect.Response[pb.SubmitVetoResponse], error) {
	m, err := s.matchMutation(ctx, req.Header(), req.Msg.TournamentId, req.Msg.MatchId, func(t *tournament.Tournament, a tournament.Actor, now time.Time) error {
		return t.Veto(a, req.Msg.MatchId, req.Msg.Scenario, now)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.SubmitVetoResponse{Match: m}), nil
}

func (s *Server) ReportGame(ctx context.Context, req *connect.Request[pb.ReportGameRequest]) (*connect.Response[pb.ReportGameResponse], error) {
	r := req.Msg
	m, err := s.matchMutation(ctx, req.Header(), r.TournamentId, r.MatchId, func(t *tournament.Tournament, a tournament.Actor, now time.Time) error {
		return t.ReportGame(a, r.MatchId, tournament.GameReport{Index: int(r.GameIndex), Scores: [2]float64{r.ScoreA, r.ScoreB},
			Replays: r.ReplayIds, HostValidated: r.HostValidated, PlayedScenario: r.PlayedScenario, Seed: r.Seed, Result: resultFromPB(r.Result)}, now)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ReportGameResponse{Match: m}), nil
}

func (s *Server) ConfirmResult(ctx context.Context, req *connect.Request[pb.ConfirmResultRequest]) (*connect.Response[pb.ConfirmResultResponse], error) {
	m, err := s.matchMutation(ctx, req.Header(), req.Msg.TournamentId, req.Msg.MatchId, func(t *tournament.Tournament, a tournament.Actor, now time.Time) error {
		return t.Confirm(a, req.Msg.MatchId, now)
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ConfirmResultResponse{Match: m}), nil
}

func (s *Server) OpenDispute(ctx context.Context, req *connect.Request[pb.OpenDisputeRequest]) (*connect.Response[pb.OpenDisputeResponse], error) {
	a, err := s.signedIn(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	var d *tournament.Dispute
	if _, _, err := s.mutate(ctx, req.Msg.TournamentId, func(t *tournament.Tournament, now time.Time) error {
		var err error
		d, err = t.OpenDispute(a, req.Msg.MatchId, req.Msg.Reason, now)
		return err
	}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.OpenDisputeResponse{Dispute: disputePB(d)}), nil
}

func (s *Server) ResolveDispute(ctx context.Context, req *connect.Request[pb.ResolveDisputeRequest]) (*connect.Response[pb.ResolveDisputeResponse], error) {
	a, err := s.signedIn(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	decision := map[pb.DisputeDecision]tournament.DisputeDecision{
		pb.DisputeDecision_DISPUTE_DECISION_UPHOLD:   tournament.DecisionUphold,
		pb.DisputeDecision_DISPUTE_DECISION_OVERTURN: tournament.DecisionOverturn,
		pb.DisputeDecision_DISPUTE_DECISION_REPLAY:   tournament.DecisionReplay,
	}[req.Msg.Decision]
	var d *tournament.Dispute
	t, _, err := s.mutate(ctx, req.Msg.TournamentId, func(t *tournament.Tournament, now time.Time) error {
		var err error
		d, err = t.ResolveDispute(a, req.Msg.DisputeId, tournament.Decision{Decision: decision, Winner: req.Msg.WinnerEntrantId,
			Wins: [2]int{int(req.Msg.WinsA), int(req.Msg.WinsB)}, Note: req.Msg.Note}, now)
		return err
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ResolveDisputeResponse{Dispute: disputePB(d), Match: matchPB(t, t.Bracket.Match(d.Match), a)}), nil
}

func (s *Server) SetMatchResult(ctx context.Context, req *connect.Request[pb.SetMatchResultRequest]) (*connect.Response[pb.SetMatchResultResponse], error) {
	r := req.Msg
	resolution := map[pb.Resolution]string{
		pb.Resolution_RESOLUTION_UNSPECIFIED:    "override",
		pb.Resolution_RESOLUTION_ADMIN_OVERRIDE: "override",
		pb.Resolution_RESOLUTION_FORFEIT:        "forfeit",
		pb.Resolution_RESOLUTION_NO_SHOW:        "no_show",
	}[r.Resolution]
	if resolution == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("Results are overrides, forfeits or no-shows."))
	}
	var reset []string
	m, err := s.matchMutation(ctx, req.Header(), r.TournamentId, r.MatchId, func(t *tournament.Tournament, a tournament.Actor, now time.Time) error {
		var err error
		reset, err = t.SetResult(a, r.MatchId, r.WinnerEntrantId, [2]int{int(r.WinsA), int(r.WinsB)}, resolution, r.Reason, now)
		return err
	})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.SetMatchResultResponse{Match: m, ResetMatchIds: reset}), nil
}

func (s *Server) Disqualify(ctx context.Context, req *connect.Request[pb.DisqualifyRequest]) (*connect.Response[pb.DisqualifyResponse], error) {
	a, err := s.signedIn(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	var e *tournament.Entrant
	if _, _, err := s.mutate(ctx, req.Msg.TournamentId, func(t *tournament.Tournament, now time.Time) error {
		var err error
		e, err = t.Disqualify(a, req.Msg.EntrantId, req.Msg.Reason, now)
		return err
	}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.DisqualifyResponse{Entrant: entrantPB(e, 0)}), nil
}

func (s *Server) SetStaff(ctx context.Context, req *connect.Request[pb.SetStaffRequest]) (*connect.Response[pb.SetStaffResponse], error) {
	a, err := s.signedIn(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	role, ok := roleFromPB[req.Msg.Role]
	if !ok && req.Msg.Role != pb.StaffRole_STAFF_ROLE_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("Unknown role."))
	}
	u, err := s.cfg.Store.TournamentUserByHandle(ctx, req.Msg.Handle)
	if err != nil {
		if errors.Is(err, store.ErrTournamentNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("No AimMod Hub player has that handle."))
		}
		return nil, toConnect(err)
	}
	ref := tournament.UserRef{UserID: u.UserID, ExternalID: u.ExternalID, Handle: u.Handle, DisplayName: u.DisplayName, AvatarURL: u.AvatarURL}
	t, _, err := s.mutate(ctx, req.Msg.TournamentId, func(t *tournament.Tournament, now time.Time) error {
		return t.SetStaff(a, ref, role, now)
	})
	if err != nil {
		return nil, err
	}
	resp := &pb.SetStaffResponse{Staff: []*pb.Staff{{User: userPB(t.Organiser), Role: pb.StaffRole_STAFF_ROLE_ORGANISER}}}
	for _, st := range t.Staff {
		resp.Staff = append(resp.Staff, &pb.Staff{User: userPB(st.User), Role: roleToPB[st.Role]})
	}
	return connect.NewResponse(resp), nil
}

func (s *Server) ReportLiveState(ctx context.Context, req *connect.Request[pb.ReportLiveStateRequest]) (*connect.Response[pb.ReportLiveStateResponse], error) {
	a, err := s.signedIn(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	live := req.Msg.Live
	if live == nil || len(live.Players) > 16 || len(live.Scenario) > 200 || len(live.Phase) > 16 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("Invalid live state."))
	}
	t, now, err := s.load(ctx, req.Msg.TournamentId)
	if err != nil {
		return nil, err
	}
	var m *bracket.Match
	if t.Bracket != nil {
		m = t.Bracket.Match(live.MatchId)
	}
	if m == nil || m.State != bracket.Ready {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("That match isn't being played."))
	}
	if slotOfViewer(t, m, a) < 0 && !t.CanManage(a) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("Only the match's players report its live state."))
	}
	for _, p := range live.Players {
		if !m.Has(p.EntrantId) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("Live state names someone outside the match."))
		}
	}
	if len(live.LobbyToken) > 128 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("Invalid lobby."))
	}
	stored := &pb.LiveMatch{MatchId: live.MatchId, GameIndex: live.GameIndex, Scenario: live.Scenario, Phase: live.Phase,
		Players: live.Players, UpdatedAt: ts(now), Spectators: live.Spectators, LobbyToken: live.LobbyToken}
	s.live.put(t.ID, live.MatchId, stored, now)
	return connect.NewResponse(&pb.ReportLiveStateResponse{}), nil
}

func (s *Server) GetOverview(ctx context.Context, req *connect.Request[pb.GetOverviewRequest]) (*connect.Response[pb.GetOverviewResponse], error) {
	a, err := s.actor(ctx, req.Header())
	if err != nil {
		return nil, err
	}
	t, _, err := s.load(ctx, req.Msg.TournamentId)
	if err != nil {
		return nil, err
	}
	if t.Status == tournament.Draft && !t.CanManage(a) {
		return nil, toConnect(store.ErrTournamentNotFound)
	}
	resp := &pb.GetOverviewResponse{Tournament: tournamentPB(t), Entrants: s.entrants(t)}
	if t.Bracket != nil {
		for _, m := range t.Bracket.Matches {
			if m.State != bracket.Ready {
				continue
			}
			resp.Active = append(resp.Active, matchPB(t, m, a))
			if l := s.live.get(t.ID, m.ID); l != nil {
				resp.Live = append(resp.Live, publicLive(l, t.CanSeePrivate(a) || slotOfViewer(t, m, a) >= 0))
			}
		}
	}
	return connect.NewResponse(resp), nil
}

// publicLive hides the lobby token from everyone but the players and staff.
func publicLive(l *pb.LiveMatch, insider bool) *pb.LiveMatch {
	if l == nil || insider || l.LobbyToken == "" {
		return l
	}
	c := *l
	c.LobbyToken = ""
	return &c
}

// ---- live state ----------------------------------------------------------

type liveEntry struct {
	at   time.Time
	live *pb.LiveMatch
}

// liveStore keeps hosts' live match state in memory: it is ephemeral and
// expires on its own. Several Hub instances would need a shared store.
type liveStore struct {
	mu    sync.Mutex
	ttl   time.Duration
	items map[string]liveEntry
	now   func() time.Time
}

func (l *liveStore) put(tid, mid string, live *pb.LiveMatch, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, v := range l.items {
		if now.Sub(v.at) > l.ttl {
			delete(l.items, k)
		}
	}
	if len(l.items) > 10000 {
		return
	}
	l.items[tid+"/"+mid] = liveEntry{at: now, live: live}
}

func (l *liveStore) get(tid, mid string) *pb.LiveMatch {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.items[tid+"/"+mid]
	if !ok {
		return nil
	}
	if l.now().Sub(e.at) > l.ttl {
		delete(l.items, tid+"/"+mid)
		return nil
	}
	return e.live
}
