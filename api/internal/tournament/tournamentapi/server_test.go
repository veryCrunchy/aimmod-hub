package tournamentapi

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	"github.com/veryCrunchy/aimmod-hub/api/internal/tournament"
	pb "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/tournament/v1"
	"github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/tournament/v1/tournamentv1connect"
)

// memStore keeps tournaments as JSON, like the database does.
type memStore struct {
	mu   sync.Mutex
	docs map[string][]byte
	vers map[string]int64
	pbs  map[int64]float64
}

func newMem() *memStore {
	return &memStore{docs: map[string][]byte{}, vers: map[string]int64{}, pbs: map[int64]float64{}}
}

func (m *memStore) CreateTournament(_ context.Context, t *tournament.Tournament) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, _ := json.Marshal(t)
	m.docs[t.ID], m.vers[t.ID] = data, t.Version
	return nil
}

func (m *memStore) LoadTournament(_ context.Context, id string) (*tournament.Tournament, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, data := range m.docs {
		var t tournament.Tournament
		_ = json.Unmarshal(data, &t)
		if key == id || t.Slug == id {
			t.Version = m.vers[key]
			return &t, nil
		}
	}
	return nil, store.ErrTournamentNotFound
}

func (m *memStore) SaveTournament(_ context.Context, t *tournament.Tournament, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.vers[t.ID] != t.Version {
		return store.ErrTournamentConflict
	}
	t.Version++
	t.UpdatedAt = now
	data, _ := json.Marshal(t)
	m.docs[t.ID], m.vers[t.ID] = data, t.Version
	return nil
}

func (m *memStore) ListTournaments(ctx context.Context, f store.TournamentFilter) ([]*tournament.Tournament, string, error) {
	m.mu.Lock()
	ids := make([]string, 0, len(m.docs))
	for id := range m.docs {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	var out []*tournament.Tournament
	for _, id := range ids {
		t, _ := m.LoadTournament(ctx, id)
		if len(f.Statuses) > 0 && !contains(f.Statuses, string(t.Status)) {
			continue
		}
		if f.MemberUserID != 0 {
			if t.EntrantOf(f.MemberUserID) == nil && t.Organiser.UserID != f.MemberUserID {
				continue
			}
		} else if t.Status == tournament.Draft {
			continue
		}
		out = append(out, t)
	}
	return out, "", nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (m *memStore) ScenarioPersonalBests(_ context.Context, ids []int64, _ string) (map[int64]float64, error) {
	out := map[int64]float64{}
	for _, id := range ids {
		if v, ok := m.pbs[id]; ok {
			out[id] = v
		}
	}
	return out, nil
}

func (m *memStore) TournamentUserByHandle(_ context.Context, handle string) (store.TournamentUser, error) {
	var n int64
	if _, err := fmt.Sscanf(handle, "player%d", &n); err != nil {
		return store.TournamentUser{}, store.ErrTournamentNotFound
	}
	return store.TournamentUser{UserID: n, Handle: handle, DisplayName: "Player " + strconv.FormatInt(n, 10)}, nil
}

type memMedia struct {
	mu    sync.Mutex
	files map[string][]byte
}

func (m *memMedia) Put(_ context.Context, key, _ string, body io.Reader, _ int64) error {
	data, _ := io.ReadAll(body)
	m.mu.Lock()
	m.files[key] = data
	m.mu.Unlock()
	return nil
}
func (m *memMedia) Get(_ context.Context, key string) (io.ReadCloser, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[key]
	if !ok {
		return nil, "", errors.New("missing")
	}
	return io.NopCloser(bytes.NewReader(data)), "application/octet-stream", nil
}
func (m *memMedia) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.files, key)
	m.mu.Unlock()
	return nil
}

// Synthetic accounts: "Bearer user-<n>"; user 1 is the Hub admin.
func testAuth(_ context.Context, h http.Header) (tournament.Actor, error) {
	auth := h.Get("Authorization")
	if auth == "" {
		return tournament.Actor{}, nil
	}
	var n int64
	if _, err := fmt.Sscanf(auth, "Bearer user-%d", &n); err != nil {
		return tournament.Actor{}, errors.New("bad token")
	}
	return tournament.Actor{UserRef: tournament.UserRef{UserID: n, ExternalID: fmt.Sprintf("ext-%d", n), Handle: fmt.Sprintf("player%d", n),
		DisplayName: fmt.Sprintf("Player %d", n), SteamID: fmt.Sprintf("7656119800000%04d", n)}, HubAdmin: n == 1}, nil
}

type harness struct {
	t      *testing.T
	srv    *httptest.Server
	client tournamentv1connect.TournamentServiceClient
	media  *memMedia
	clock  time.Time
	store  *memStore
}

func newHarness(t *testing.T, creators CreatorPolicy) *harness {
	h := &harness{t: t, media: &memMedia{files: map[string][]byte{}}, clock: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC), store: newMem()}
	server := New(Config{Store: h.store, Auth: testAuth, Creators: creators, Now: func() time.Time { return h.clock }})
	mux := http.NewServeMux()
	path, handler := tournamentv1connect.NewTournamentServiceHandler(server)
	mux.Handle(path, handler)
	mux.Handle(ReplayPath, server.ReplayHandler(h.media))
	mux.Handle(ReplayPath+"/", server.ReplayHandler(h.media))
	h.srv = httptest.NewServer(mux)
	t.Cleanup(h.srv.Close)
	// JSON, the way the in-game client talks to the Hub.
	h.client = tournamentv1connect.NewTournamentServiceClient(h.srv.Client(), h.srv.URL, connect.WithProtoJSON())
	return h
}

func as[T any](user int, msg *T) *connect.Request[T] {
	req := connect.NewRequest(msg)
	if user > 0 {
		req.Header().Set("Authorization", fmt.Sprintf("Bearer user-%d", user))
	}
	return req
}

func code(err error) connect.Code {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce.Code()
	}
	return 0
}

func (h *harness) must(err error) {
	h.t.Helper()
	if err != nil {
		h.t.Fatal(err)
	}
}

func spec() *pb.TournamentSpec {
	return &pb.TournamentSpec{
		Name: "Synthetic Open", Format: pb.TournamentFormat_TOURNAMENT_FORMAT_SINGLE_ELIMINATION, Seeding: pb.SeedingMethod_SEEDING_METHOD_MANUAL,
		Ruleset: &pb.Ruleset{BestOf: 1, Pool: []*pb.PoolScenario{{Name: "Synthetic Track", TimeLimitSeconds: 60}}},
	}
}

func replayBytes(scenario string, score float64, at time.Time) []byte {
	head, _ := json.Marshal(map[string]any{"kind": "header", "version": 2, "id": "r", "coordinates": "unreal-centimeters",
		"recordedAt": at.Format(time.RFC3339), "scenario": scenario, "reason": "completed", "frames": 3600, "inputEvents": 900, "score": score, "duration": 60.0})
	out := []byte("AMRPLAY2")
	out = binary.LittleEndian.AppendUint32(out, 2)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(head)))
	out = append(out, head...)
	out = binary.LittleEndian.AppendUint32(out, 5)
	out = binary.LittleEndian.AppendUint32(out, 16)
	return append(out, make([]byte, 16)...)
}

func (h *harness) upload(user int, tid, mid string, game int, body []byte) (int, map[string]any) {
	url := fmt.Sprintf("%s%s?tournament=%s&match=%s&game=%d", h.srv.URL, ReplayPath, tid, mid, game)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Authorization", fmt.Sprintf("Bearer user-%d", user))
	resp, err := h.srv.Client().Do(req)
	h.must(err)
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestCreatorPolicy(t *testing.T) {
	h := newHarness(t, CreatorsAdmin)
	ctx := context.Background()
	_, err := h.client.CreateTournament(ctx, as(0, &pb.CreateTournamentRequest{Spec: spec()}))
	if code(err) != connect.CodeUnauthenticated {
		t.Fatalf("anonymous create: %v", err)
	}
	_, err = h.client.CreateTournament(ctx, as(2, &pb.CreateTournamentRequest{Spec: spec()}))
	if code(err) != connect.CodePermissionDenied {
		t.Fatalf("non-admin create: %v", err)
	}
	bad := spec()
	bad.Ruleset.BestOf = 4
	_, err = h.client.CreateTournament(ctx, as(1, &pb.CreateTournamentRequest{Spec: bad}))
	if code(err) != connect.CodeInvalidArgument {
		t.Fatalf("bad spec: %v", err)
	}
	bad = spec()
	bad.StartsAt = "tomorrow"
	_, err = h.client.CreateTournament(ctx, as(1, &pb.CreateTournamentRequest{Spec: bad}))
	if code(err) != connect.CodeInvalidArgument {
		t.Fatalf("bad time: %v", err)
	}
	created, err := h.client.CreateTournament(ctx, as(1, &pb.CreateTournamentRequest{Spec: spec()}))
	h.must(err)
	if created.Msg.Tournament.Status != pb.TournamentStatus_TOURNAMENT_STATUS_DRAFT || created.Msg.Tournament.Slug != "synthetic-open" {
		t.Fatalf("created %+v", created.Msg.Tournament)
	}
	// Drafts are hidden from everyone else.
	_, err = h.client.GetTournament(ctx, as(2, &pb.GetTournamentRequest{Tournament: "synthetic-open"}))
	if code(err) != connect.CodeNotFound {
		t.Fatalf("draft visible: %v", err)
	}
	list, err := h.client.ListTournaments(ctx, as(0, &pb.ListTournamentsRequest{}))
	h.must(err)
	if len(list.Msg.Tournaments) != 0 {
		t.Fatal("draft listed")
	}
	open := newHarness(t, CreatorsSignedIn)
	_, err = open.client.CreateTournament(ctx, as(5, &pb.CreateTournamentRequest{Spec: spec()}))
	h.must(err)
}

func TestTournamentFlowOverTheAPI(t *testing.T) {
	h := newHarness(t, CreatorsAdmin)
	ctx := context.Background()
	created, err := h.client.CreateTournament(ctx, as(1, &pb.CreateTournamentRequest{Spec: spec()}))
	h.must(err)
	tid := created.Msg.Tournament.Id
	_, err = h.client.Register(ctx, as(2, &pb.RegisterRequest{TournamentId: tid}))
	if code(err) != connect.CodeFailedPrecondition {
		t.Fatalf("registered into a draft: %v", err)
	}
	_, err = h.client.AdvanceTournament(ctx, as(2, &pb.AdvanceTournamentRequest{TournamentId: tid, Action: pb.AdvanceAction_ADVANCE_ACTION_OPEN_REGISTRATION}))
	if code(err) != connect.CodePermissionDenied {
		t.Fatalf("player opened registration: %v", err)
	}
	_, err = h.client.AdvanceTournament(ctx, as(1, &pb.AdvanceTournamentRequest{TournamentId: tid, Action: pb.AdvanceAction_ADVANCE_ACTION_OPEN_REGISTRATION}))
	h.must(err)
	for _, u := range []int{2, 3} {
		_, err := h.client.Register(ctx, as(u, &pb.RegisterRequest{TournamentId: tid}))
		h.must(err)
	}
	got, err := h.client.GetTournament(ctx, as(4, &pb.GetTournamentRequest{Tournament: tid}))
	h.must(err)
	if !got.Msg.Viewer.CanRegister || got.Msg.Viewer.EntrantId != "" || len(got.Msg.Entrants) != 2 || len(got.Msg.Audit) != 0 {
		t.Fatalf("viewer %+v", got.Msg.Viewer)
	}
	_, err = h.client.AdvanceTournament(ctx, as(1, &pb.AdvanceTournamentRequest{TournamentId: tid, Action: pb.AdvanceAction_ADVANCE_ACTION_START}))
	h.must(err)
	// The higher seed (player 2) hosts; the opponent's Steam id is there for the invite.
	mine, err := h.client.ListMyMatches(ctx, as(2, &pb.ListMyMatchesRequest{}))
	h.must(err)
	if len(mine.Msg.Matches) != 1 || !mine.Msg.Matches[0].Host || mine.Msg.Matches[0].OpponentSteamId != "76561198000000003" {
		t.Fatalf("my matches %+v", mine.Msg.Matches)
	}
	other, _ := h.client.ListMyMatches(ctx, as(3, &pb.ListMyMatchesRequest{}))
	if other.Msg.Matches[0].Host {
		t.Fatal("both players host")
	}
	mid := mine.Msg.Matches[0].Match.Id
	for _, u := range []int{2, 3} {
		_, err := h.client.MarkReady(ctx, as(u, &pb.MarkReadyRequest{TournamentId: tid, MatchId: mid, Ready: true, CanHost: true}))
		h.must(err)
	}
	// Players see the game's seed; anonymous viewers don't until the match ends.
	pm, err := h.client.GetMatch(ctx, as(3, &pb.GetMatchRequest{TournamentId: tid, MatchId: mid}))
	h.must(err)
	anon, err := h.client.GetMatch(ctx, as(0, &pb.GetMatchRequest{TournamentId: tid, MatchId: mid}))
	h.must(err)
	if pm.Msg.Match.State != pb.MatchState_MATCH_STATE_LIVE || pm.Msg.Match.Games[0].Seed == "" || anon.Msg.Match.Games[0].Seed != "" {
		t.Fatalf("seed visibility: player %q anon %q", pm.Msg.Match.Games[0].Seed, anon.Msg.Match.Games[0].Seed)
	}
	seed := pm.Msg.Match.Games[0].Seed
	// Live state from the host shows up in the overview.
	_, err = h.client.ReportLiveState(ctx, as(4, &pb.ReportLiveStateRequest{TournamentId: tid, Live: &pb.LiveMatch{MatchId: mid}}))
	if code(err) != connect.CodePermissionDenied {
		t.Fatalf("outsider live state: %v", err)
	}
	ea, eb := pm.Msg.EntrantA.Id, pm.Msg.EntrantB.Id
	_, err = h.client.ReportLiveState(ctx, as(2, &pb.ReportLiveStateRequest{TournamentId: tid, Live: &pb.LiveMatch{MatchId: mid, Phase: "live",
		Players: []*pb.LivePlayer{{EntrantId: ea, Score: 500, PingMs: 30, Connection: "connected"}, {EntrantId: eb, Score: 450}}, LobbyToken: "lobby-synthetic"}}))
	h.must(err)
	// The opponent's client learns the lobby to join; the public doesn't.
	joiner, _ := h.client.ListMyMatches(ctx, as(3, &pb.ListMyMatchesRequest{}))
	if joiner.Msg.Matches[0].LobbyToken != "lobby-synthetic" {
		t.Fatalf("lobby token for the opponent: %q", joiner.Msg.Matches[0].LobbyToken)
	}
	if pub, _ := h.client.GetMatch(ctx, as(0, &pb.GetMatchRequest{TournamentId: tid, MatchId: mid})); pub.Msg.Live.LobbyToken != "" {
		t.Fatal("lobby token leaked to the public")
	}
	ov, err := h.client.GetOverview(ctx, as(0, &pb.GetOverviewRequest{TournamentId: tid}))
	h.must(err)
	if len(ov.Msg.Active) != 1 || len(ov.Msg.Live) != 1 || ov.Msg.Live[0].Players[0].Score != 500 {
		t.Fatalf("overview %+v", ov.Msg)
	}
	h.clock = h.clock.Add(time.Minute)
	ov, _ = h.client.GetOverview(ctx, as(0, &pb.GetOverviewRequest{TournamentId: tid}))
	if len(ov.Msg.Live) != 0 {
		t.Fatal("stale live state kept")
	}
	// Replays: not a replay, a stranger, then both players.
	if status, _ := h.upload(2, tid, mid, 0, []byte("definitely not a replay file")); status != http.StatusUnprocessableEntity {
		t.Fatalf("junk upload %d", status)
	}
	if status, _ := h.upload(4, tid, mid, 0, replayBytes("Synthetic Track", 1000, h.clock)); status != http.StatusForbidden {
		t.Fatalf("stranger upload %d", status)
	}
	status, r2 := h.upload(2, tid, mid, 0, replayBytes("Synthetic Track", 1000, h.clock))
	if status != http.StatusOK {
		t.Fatalf("upload %d %v", status, r2)
	}
	_, r3 := h.upload(3, tid, mid, 0, replayBytes("Synthetic Track", 900, h.clock))
	if len(h.media.files) != 2 {
		t.Fatalf("stored %d files", len(h.media.files))
	}
	rep, err := h.client.ReportGame(ctx, as(2, &pb.ReportGameRequest{TournamentId: tid, MatchId: mid, GameIndex: 0, ScoreA: 1000, ScoreB: 900,
		ReplayIds: []string{r2["id"].(string), r3["id"].(string)}, HostValidated: true, Seed: seed, PlayedScenario: "Synthetic Track"}))
	h.must(err)
	g := rep.Msg.Match.Games[0]
	if rep.Msg.Match.State != pb.MatchState_MATCH_STATE_AWAITING_CONFIRMATION || len(g.Replays) != 2 || g.Replays[0].Status != pb.ReplayStatus_REPLAY_STATUS_VERIFIED || !g.HostValidated {
		t.Fatalf("report %+v", rep.Msg.Match)
	}
	// Download: players and staff only.
	dl := func(user int) int {
		req, _ := http.NewRequest(http.MethodGet, h.srv.URL+ReplayPath+"/"+tid+"/"+r3["id"].(string), nil)
		if user > 0 {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer user-%d", user))
		}
		resp, err := h.srv.Client().Do(req)
		h.must(err)
		resp.Body.Close()
		return resp.StatusCode
	}
	if dl(0) != http.StatusUnauthorized || dl(4) != http.StatusForbidden || dl(2) != http.StatusOK || dl(1) != http.StatusOK {
		t.Fatal("replay download permissions")
	}
	_, err = h.client.ConfirmResult(ctx, as(2, &pb.ConfirmResultRequest{TournamentId: tid, MatchId: mid}))
	if code(err) != connect.CodeFailedPrecondition {
		t.Fatalf("reporter confirmed: %v", err)
	}
	_, err = h.client.ConfirmResult(ctx, as(3, &pb.ConfirmResultRequest{TournamentId: tid, MatchId: mid}))
	h.must(err)
	final, err := h.client.GetTournament(ctx, as(0, &pb.GetTournamentRequest{Tournament: tid}))
	h.must(err)
	if final.Msg.Tournament.Status != pb.TournamentStatus_TOURNAMENT_STATUS_COMPLETED || final.Msg.Tournament.ChampionEntrantId != ea {
		t.Fatalf("final %+v", final.Msg.Tournament)
	}
	if final.Msg.Matches[0].Games[0].Seed != seed || final.Msg.Standings[0].EntrantId != ea {
		t.Fatal("completed match should show its seed and standings")
	}
	if final.Msg.Matches[0].Label != "Final" {
		t.Fatalf("label %q", final.Msg.Matches[0].Label)
	}
}

func TestDisputeAndOverrideOverTheAPI(t *testing.T) {
	h := newHarness(t, CreatorsAdmin)
	ctx := context.Background()
	created, _ := h.client.CreateTournament(ctx, as(1, &pb.CreateTournamentRequest{Spec: spec()}))
	tid := created.Msg.Tournament.Id
	h.client.AdvanceTournament(ctx, as(1, &pb.AdvanceTournamentRequest{TournamentId: tid, Action: pb.AdvanceAction_ADVANCE_ACTION_OPEN_REGISTRATION}))
	for _, u := range []int{2, 3, 4, 5} {
		_, err := h.client.Register(ctx, as(u, &pb.RegisterRequest{TournamentId: tid}))
		h.must(err)
	}
	_, err := h.client.SetSeeds(ctx, as(1, &pb.SetSeedsRequest{TournamentId: tid, EntrantIds: []string{"e4", "e3"}}))
	h.must(err)
	_, err = h.client.AdvanceTournament(ctx, as(1, &pb.AdvanceTournamentRequest{TournamentId: tid, Action: pb.AdvanceAction_ADVANCE_ACTION_START}))
	h.must(err)
	full, _ := h.client.GetTournament(ctx, as(1, &pb.GetTournamentRequest{Tournament: tid}))
	if full.Msg.Entrants[3].Seed != 1 || len(full.Msg.Audit) == 0 {
		t.Fatalf("seeds %+v", full.Msg.Entrants)
	}
	m := full.Msg.Matches[0] // e4 (player 5) v the 4th seed
	a, b := m.SlotA.EntrantId, m.SlotB.EntrantId
	user := map[string]int{"e1": 2, "e2": 3, "e3": 4, "e4": 5}
	for _, e := range []string{a, b} {
		_, err := h.client.MarkReady(ctx, as(user[e], &pb.MarkReadyRequest{TournamentId: tid, MatchId: m.Id, Ready: true, CanHost: true}))
		h.must(err)
	}
	_, err = h.client.ReportGame(ctx, as(user[a], &pb.ReportGameRequest{TournamentId: tid, MatchId: m.Id, GameIndex: 0, ScoreA: 10, ScoreB: 5}))
	h.must(err)
	d, err := h.client.OpenDispute(ctx, as(user[b], &pb.OpenDisputeRequest{TournamentId: tid, MatchId: m.Id, Reason: "The scoreboard froze for me."}))
	h.must(err)
	res, err := h.client.ResolveDispute(ctx, as(1, &pb.ResolveDisputeRequest{TournamentId: tid, DisputeId: d.Msg.Dispute.Id,
		Decision: pb.DisputeDecision_DISPUTE_DECISION_OVERTURN, WinnerEntrantId: b, WinsA: 0, WinsB: 1, Note: "Checked the replays."}))
	h.must(err)
	if res.Msg.Match.WinnerEntrantId != b || res.Msg.Dispute.Status != pb.DisputeStatus_DISPUTE_STATUS_RESOLVED {
		t.Fatalf("resolve %+v", res.Msg)
	}
	set, err := h.client.SetMatchResult(ctx, as(1, &pb.SetMatchResultRequest{TournamentId: tid, MatchId: m.Id, WinnerEntrantId: a, WinsA: 1}))
	h.must(err)
	if set.Msg.Match.WinnerEntrantId != a || set.Msg.Match.Resolution != pb.Resolution_RESOLUTION_ADMIN_OVERRIDE {
		t.Fatalf("override %+v", set.Msg.Match)
	}
	_, err = h.client.Disqualify(ctx, as(2, &pb.DisqualifyRequest{TournamentId: tid, EntrantId: a}))
	if code(err) != connect.CodePermissionDenied {
		t.Fatalf("player DQ: %v", err)
	}
	dq, err := h.client.Disqualify(ctx, as(1, &pb.DisqualifyRequest{TournamentId: tid, EntrantId: a, Reason: "Synthetic"}))
	h.must(err)
	if dq.Msg.Entrant.Status != pb.EntrantStatus_ENTRANT_STATUS_DISQUALIFIED {
		t.Fatal("DQ status")
	}
	staff, err := h.client.SetStaff(ctx, as(1, &pb.SetStaffRequest{TournamentId: tid, Handle: "player9", Role: pb.StaffRole_STAFF_ROLE_CASTER}))
	h.must(err)
	if len(staff.Msg.Staff) != 2 || staff.Msg.Staff[1].Role != pb.StaffRole_STAFF_ROLE_CASTER {
		t.Fatalf("staff %+v", staff.Msg.Staff)
	}
	_, err = h.client.SetStaff(ctx, as(1, &pb.SetStaffRequest{TournamentId: tid, Handle: "nobody", Role: pb.StaffRole_STAFF_ROLE_ADMIN}))
	if code(err) != connect.CodeNotFound {
		t.Fatalf("unknown staff: %v", err)
	}
}

func TestStatSeedingUsesPersonalBests(t *testing.T) {
	h := newHarness(t, CreatorsAdmin)
	ctx := context.Background()
	sp := spec()
	sp.Seeding, sp.SeedingSource = pb.SeedingMethod_SEEDING_METHOD_SCENARIO_PB, "Synthetic Track"
	created, _ := h.client.CreateTournament(ctx, as(1, &pb.CreateTournamentRequest{Spec: sp}))
	tid := created.Msg.Tournament.Id
	h.client.AdvanceTournament(ctx, as(1, &pb.AdvanceTournamentRequest{TournamentId: tid, Action: pb.AdvanceAction_ADVANCE_ACTION_OPEN_REGISTRATION}))
	for _, u := range []int{2, 3, 4} {
		h.client.Register(ctx, as(u, &pb.RegisterRequest{TournamentId: tid}))
	}
	h.store.pbs[4], h.store.pbs[3] = 2000, 1000
	_, err := h.client.AdvanceTournament(ctx, as(1, &pb.AdvanceTournamentRequest{TournamentId: tid, Action: pb.AdvanceAction_ADVANCE_ACTION_START}))
	h.must(err)
	full, _ := h.client.GetTournament(ctx, as(0, &pb.GetTournamentRequest{Tournament: tid}))
	seeds := map[string]int32{}
	for _, e := range full.Msg.Entrants {
		seeds[e.User.Handle] = e.Seed
	}
	if seeds["player4"] != 1 || seeds["player3"] != 2 || seeds["player2"] != 3 {
		t.Fatalf("seeds %v", seeds)
	}
	if !strings.Contains(full.Msg.Entrants[2].RatingLabel, "2000") {
		t.Fatalf("rating label %q", full.Msg.Entrants[2].RatingLabel)
	}
}
