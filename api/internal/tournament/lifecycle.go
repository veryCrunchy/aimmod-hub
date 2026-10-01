package tournament

import (
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/tournament/bracket"
)

// Rating is a stat used for seeding: higher is better (benchmark rank
// points, or a scenario personal best).
type Rating struct {
	Value float64
	Label string
}

// New creates a draft tournament owned by the actor.
func New(id string, spec Spec, organiser Actor, now time.Time) (*Tournament, error) {
	if !organiser.SignedIn() {
		return nil, ErrSignIn
	}
	spec, err := spec.Normalize()
	if err != nil {
		return nil, err
	}
	t := &Tournament{
		ID:        id,
		Slug:      Slugify(spec.Name),
		Spec:      spec,
		Status:    Draft,
		Organiser: publicRef(organiser.UserRef),
		Entrants:  []*Entrant{},
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}
	t.audit(now, organiser, "create", spec.Name)
	return t, nil
}

// Update changes the spec. The format, field size and seeding are fixed once
// the bracket exists; names, description, prize and times stay editable.
func (t *Tournament) Update(a Actor, spec Spec, now time.Time) error {
	if err := t.requireManager(a); err != nil {
		return err
	}
	if t.Status == Completed || t.Status == Cancelled {
		return errState("This tournament is over.")
	}
	spec, err := spec.Normalize()
	if err != nil {
		return err
	}
	if t.Status == InProgress {
		old := t.Spec
		if spec.Format != old.Format || spec.MaxEntrants != old.MaxEntrants || spec.Seeding != old.Seeding ||
			spec.SeedingSource != old.SeedingSource || spec.Options != old.Options || spec.Scheduling != old.Scheduling {
			return errState("The format, field and seeding can't change once the bracket is out.")
		}
		// A ruleset change applies to matches that haven't started.
	}
	if len(t.activeEntrants()) > spec.MaxEntrants {
		return errInvalid("%d players have already entered.", len(t.activeEntrants()))
	}
	t.Spec = spec
	t.audit(now, a, "update", "")
	return nil
}

type AdvanceAction string

const (
	OpenRegistration AdvanceAction = "open_registration"
	OpenCheckIn      AdvanceAction = "open_check_in"
	Start            AdvanceAction = "start"
	Cancel           AdvanceAction = "cancel"
)

// Advance moves the tournament through its lifecycle. Start needs ratings
// when seeding by stats (missing players are seeded last) and a random seed
// for random seeding.
func (t *Tournament) Advance(a Actor, action AdvanceAction, ratings map[int64]Rating, draw int64, now time.Time) error {
	if err := t.requireManager(a); err != nil {
		return err
	}
	switch action {
	case OpenRegistration:
		if t.Status != Draft {
			return errState("Registration is already open.")
		}
		t.Status = Registration
	case OpenCheckIn:
		if t.Status != Registration {
			return errState("Open registration first.")
		}
		t.Status = CheckIn
	case Start:
		if t.Status != Registration && t.Status != CheckIn {
			return errState("The tournament can only start from registration or check-in.")
		}
		if err := t.start(ratings, draw, now); err != nil {
			return err
		}
	case Cancel:
		if t.Status == Completed || t.Status == Cancelled {
			return errState("This tournament is already over.")
		}
		t.Status = Cancelled
	default:
		return errInvalid("Unknown action.")
	}
	t.audit(now, a, string(action), "")
	return nil
}

func (t *Tournament) activeEntrants() []*Entrant {
	var out []*Entrant
	for _, e := range t.Entrants {
		if e.Active() {
			out = append(out, e)
		}
	}
	return out
}

func (t *Tournament) start(ratings map[int64]Rating, draw int64, now time.Time) error {
	var field []*Entrant
	for _, e := range t.activeEntrants() {
		if t.CheckInRequired() && e.CheckedInAt == nil {
			continue
		}
		field = append(field, e)
	}
	if len(field) < 2 {
		return errState("At least two checked-in players are needed to start.")
	}
	for _, e := range t.activeEntrants() {
		if t.CheckInRequired() && e.CheckedInAt == nil {
			e.Status = EntrantNoShow
			e.Seed = 0
		}
	}
	t.seed(field, t.Spec.Seeding, ratings, draw)
	ids := make([]string, len(field))
	for i, e := range field {
		ids[i] = e.ID
		e.Status = EntrantActive
	}
	b, err := bracket.Generate(t.Spec.Format, ids, t.Spec.Options)
	if err != nil {
		return errInvalid("%v", err)
	}
	t.Bracket = b
	t.Series = map[string]*Series{}
	t.Status = InProgress
	t.syncSeries(now)
	return nil
}

// seed orders field in place and numbers it from 1.
func (t *Tournament) seed(field []*Entrant, method SeedingMethod, ratings map[int64]Rating, draw int64) {
	byRegistration := func(i, j int) bool {
		if !field[i].RegisteredAt.Equal(field[j].RegisteredAt) {
			return field[i].RegisteredAt.Before(field[j].RegisteredAt)
		}
		return field[i].ID < field[j].ID
	}
	sort.SliceStable(field, byRegistration)
	switch method {
	case SeedManual:
		// Manual seeds first in their order; anyone unseeded follows by registration.
		sort.SliceStable(field, func(i, j int) bool {
			a, b := field[i].Seed, field[j].Seed
			if (a > 0) != (b > 0) {
				return a > 0
			}
			return a > 0 && a < b
		})
	case SeedRandom:
		t.RandomSeed = draw
		r := rand.New(rand.NewSource(draw))
		r.Shuffle(len(field), func(i, j int) { field[i], field[j] = field[j], field[i] })
	case SeedBenchmark, SeedScenario:
		for _, e := range field {
			e.Rating, e.RatingLabel = nil, ""
			if r, ok := ratings[e.User.UserID]; ok {
				v := r.Value
				e.Rating, e.RatingLabel = &v, r.Label
			}
		}
		sort.SliceStable(field, func(i, j int) bool {
			a, b := field[i].Rating, field[j].Rating
			if (a != nil) != (b != nil) {
				return a != nil
			}
			return a != nil && *a > *b
		})
	}
	for i, e := range field {
		e.Seed = i + 1
	}
}

// Register enters the actor. Invite-only tournaments need an invite for the
// actor's handle.
func (t *Tournament) Register(a Actor, now time.Time) (*Entrant, error) {
	if !a.SignedIn() {
		return nil, ErrSignIn
	}
	if t.Status != Registration && t.Status != CheckIn {
		return nil, errState("Registration is closed.")
	}
	if e := t.EntrantOf(a.UserID); e != nil {
		if e.Active() {
			return e, nil
		}
		if e.Status == EntrantDisqualified {
			return nil, errForbidden("You were disqualified from this tournament.")
		}
	}
	if t.Spec.InviteOnly && !t.Invited(a.Handle) && !t.CanManage(a) {
		return nil, errForbidden("This tournament is invite-only.")
	}
	if len(t.activeEntrants()) >= t.Spec.MaxEntrants {
		return nil, errState("The tournament is full.")
	}
	if e := t.EntrantOf(a.UserID); e != nil {
		e.Status, e.RegisteredAt, e.CheckedInAt, e.User = EntrantRegistered, now, nil, a.UserRef
		t.audit(now, a, "register", e.ID)
		return e, nil
	}
	e := &Entrant{ID: t.newID("e"), User: a.UserRef, Status: EntrantRegistered, RegisteredAt: now}
	t.Entrants = append(t.Entrants, e)
	t.audit(now, a, "register", e.ID)
	return e, nil
}

// Withdraw takes the actor out. Before the start they simply leave; once the
// bracket is out every remaining match is forfeited.
func (t *Tournament) Withdraw(a Actor, now time.Time) error {
	e := t.EntrantOf(a.UserID)
	if e == nil || !e.Active() {
		return errNotFound("You haven't entered this tournament.")
	}
	switch t.Status {
	case Registration, CheckIn, Draft:
		e.Status = EntrantWithdrawn
		e.Seed = 0
	case InProgress:
		e.Status = EntrantWithdrawn
		t.Bracket.Withdraw(e.ID)
		t.syncSeries(now)
	default:
		return errState("This tournament is over.")
	}
	t.audit(now, a, "withdraw", e.ID)
	return nil
}

// CheckIn confirms the actor will play. Only during the check-in window.
func (t *Tournament) CheckIn(a Actor, now time.Time) (*Entrant, error) {
	e := t.EntrantOf(a.UserID)
	if e == nil || !e.Active() {
		return nil, errNotFound("You haven't entered this tournament.")
	}
	if !t.CheckInOpen(now) {
		return nil, errState("Check-in isn't open.")
	}
	if e.CheckedInAt == nil {
		at := now
		e.CheckedInAt = &at
		e.Status = EntrantCheckedIn
		t.audit(now, a, "check_in", e.ID)
	}
	return e, nil
}

// CheckInOpen: during check-in status, inside the window when one is set.
func (t *Tournament) CheckInOpen(now time.Time) bool {
	if t.Status != CheckIn {
		return false
	}
	if t.Spec.CheckInOpens != nil && now.Before(*t.Spec.CheckInOpens) {
		return false
	}
	if t.Spec.CheckInCloses != nil && !now.Before(*t.Spec.CheckInCloses) {
		return false
	}
	return true
}

// SetInvites adds or removes invited handles.
func (t *Tournament) SetInvites(a Actor, handles []string, remove bool, now time.Time) error {
	if err := t.requireManager(a); err != nil {
		return err
	}
	set := map[string]bool{}
	for _, h := range t.Invites {
		set[h] = true
	}
	for _, h := range handles {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" || len(h) > 64 || strings.ContainsAny(h, " \t\n/") {
			return errInvalid("Invite players by their Hub handle.")
		}
		if remove {
			delete(set, h)
		} else {
			set[h] = true
		}
	}
	if len(set) > MaxInvites {
		return errInvalid("At most %d invites.", MaxInvites)
	}
	t.Invites = t.Invites[:0]
	for h := range set {
		t.Invites = append(t.Invites, h)
	}
	sort.Strings(t.Invites)
	t.audit(now, a, "invites", strings.Join(handles, ", "))
	return nil
}

// SetSeeds sets manual seeds: listed entrants first, the rest after in their
// current order. Before the bracket exists, or before any match is played
// (the bracket is then generated again).
func (t *Tournament) SetSeeds(a Actor, order []string, now time.Time) error {
	if err := t.requireManager(a); err != nil {
		return err
	}
	if err := t.canReseed(); err != nil {
		return err
	}
	field := t.seedable()
	pos := map[string]int{}
	for i, id := range order {
		if _, dup := pos[id]; dup {
			return errInvalid("An entrant is listed twice.")
		}
		pos[id] = i
	}
	for id := range pos {
		if e := t.Entrant(id); e == nil || !e.Active() {
			return errInvalid("Unknown entrant %s.", id)
		}
	}
	sort.SliceStable(field, func(i, j int) bool {
		pi, oki := pos[field[i].ID]
		pj, okj := pos[field[j].ID]
		if oki != okj {
			return oki
		}
		if oki {
			return pi < pj
		}
		si, sj := field[i].Seed, field[j].Seed
		if (si > 0) != (sj > 0) {
			return si > 0
		}
		return si < sj
	})
	for i, e := range field {
		e.Seed = i + 1
	}
	t.Spec.Seeding = SeedManual
	t.audit(now, a, "seeds", strings.Join(order, ","))
	return t.regenerate(now)
}

// Reseed applies a seeding method again.
func (t *Tournament) Reseed(a Actor, method SeedingMethod, source string, ratings map[int64]Rating, draw int64, now time.Time) error {
	if err := t.requireManager(a); err != nil {
		return err
	}
	if err := t.canReseed(); err != nil {
		return err
	}
	spec := t.Spec
	spec.Seeding, spec.SeedingSource = method, source
	spec, err := spec.Normalize()
	if err != nil {
		return err
	}
	t.Spec = spec
	field := t.seedable()
	t.seed(field, method, ratings, draw)
	t.audit(now, a, "reseed", string(method))
	return t.regenerate(now)
}

func (t *Tournament) canReseed() error {
	switch t.Status {
	case Draft, Registration, CheckIn:
		return nil
	case InProgress:
		for _, m := range t.Bracket.Matches {
			if m.State == bracket.Complete && m.Resolution != bracket.Walkover {
				return errState("Seeds are fixed once a match has been played.")
			}
		}
		for _, s := range t.Series {
			if s.StartedAt != nil || len(s.Veto) > 0 {
				return errState("Seeds are fixed once a match has started.")
			}
		}
		return nil
	}
	return errState("This tournament is over.")
}

func (t *Tournament) seedable() []*Entrant {
	var field []*Entrant
	for _, e := range t.activeEntrants() {
		if t.Status == InProgress && t.Bracket != nil && !contains(t.Bracket.Entrants, e.ID) {
			continue
		}
		field = append(field, e)
	}
	sort.SliceStable(field, func(i, j int) bool {
		si, sj := field[i].Seed, field[j].Seed
		if (si > 0) != (sj > 0) {
			return si > 0
		}
		if si != sj {
			return si < sj
		}
		return field[i].RegisteredAt.Before(field[j].RegisteredAt)
	})
	return field
}

func (t *Tournament) regenerate(now time.Time) error {
	if t.Status != InProgress {
		return nil
	}
	field := t.seedable()
	ids := make([]string, len(field))
	for i, e := range field {
		ids[i] = e.ID
	}
	b, err := bracket.Generate(t.Spec.Format, ids, t.Spec.Options)
	if err != nil {
		return errInvalid("%v", err)
	}
	t.Bracket = b
	t.Series = map[string]*Series{}
	t.syncSeries(now)
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Tick applies everything time decides: check-in opening, no-show forfeits,
// accepting unconfirmed results, and completion. It reports whether
// anything changed so callers know to save.
func (t *Tournament) Tick(now time.Time) bool {
	changed := false
	if t.Status == Registration && t.Spec.CheckInOpens != nil && !now.Before(*t.Spec.CheckInOpens) {
		t.Status = CheckIn
		changed = true
	}
	if t.Status != InProgress || t.Bracket == nil {
		return changed
	}
	if t.syncSeries(now) {
		changed = true
	}
	for _, m := range t.Bracket.Matches {
		s := t.Series[m.ID]
		if s == nil || m.State != bracket.Ready || s.Deadline == nil || now.Before(*s.Deadline) {
			continue
		}
		switch {
		case s.ReportedAt != nil:
			if t.openDispute(m.ID) == nil && !s.blocked(t) {
				if err := t.finalize(m, s, now); err == nil {
					s.addFlag("auto-confirmed")
					changed = true
				}
			} else if s.addFlag("needs-review") {
				changed = true
			}
		case s.StartedAt == nil:
			if t.noShow(m, s, now) {
				changed = true
			}
		}
	}
	if t.syncSeries(now) {
		changed = true
	}
	if t.Bracket.Done() {
		t.Status = Completed
		changed = true
	}
	if t.updateEntrantStatus() {
		changed = true
	}
	return changed
}

// noShow: one player ready and the other not by the deadline forfeits the
// absent one. Nobody ready flags the match for the organisers.
func (t *Tournament) noShow(m *bracket.Match, s *Series, now time.Time) bool {
	a, z := m.Entrants()
	_, ra := s.ReadyAt[a]
	_, rz := s.ReadyAt[z]
	switch {
	case ra && !rz:
		_ = t.Bracket.Forfeit(m.ID, z)
	case rz && !ra:
		_ = t.Bracket.Forfeit(m.ID, a)
	default:
		return s.addFlag("no-show-both")
	}
	s.Resolution = "no_show"
	s.CompletedAt = &now
	return true
}

func (t *Tournament) updateEntrantStatus() bool {
	if t.Bracket == nil {
		return false
	}
	changed := false
	for _, st := range t.Bracket.Standings() {
		e := t.Entrant(st.Entrant)
		if e == nil || !e.Active() {
			continue
		}
		want := EntrantActive
		if st.Eliminated {
			want = EntrantEliminated
		}
		if e.Status != want {
			e.Status = want
			changed = true
		}
	}
	return changed
}

// Disqualify removes an entrant; their remaining matches are forfeited.
func (t *Tournament) Disqualify(a Actor, entrantID, reason string, now time.Time) (*Entrant, error) {
	if err := t.requireManager(a); err != nil {
		return nil, err
	}
	e := t.Entrant(entrantID)
	if e == nil {
		return nil, errNotFound("Unknown entrant.")
	}
	if e.Status == EntrantDisqualified {
		return e, nil
	}
	e.Status = EntrantDisqualified
	if t.Status == InProgress && t.Bracket != nil && contains(t.Bracket.Entrants, e.ID) {
		t.Bracket.Withdraw(e.ID)
		t.syncSeries(now)
	} else {
		e.Seed = 0
	}
	t.audit(now, a, "disqualify", e.ID+": "+cleanLine(reason, MaxReasonLength))
	return e, nil
}

// SetStaff adds, changes or (RoleNone) removes a staff member.
func (t *Tournament) SetStaff(a Actor, user UserRef, role Role, now time.Time) error {
	if !a.SignedIn() {
		return ErrSignIn
	}
	if t.RoleOf(a) != RoleOrganiser && !a.HubAdmin {
		return errForbidden("Only the organiser can change the staff.")
	}
	if user.UserID == t.Organiser.UserID {
		return errInvalid("The organiser is always staff.")
	}
	switch role {
	case RoleNone, RoleAdmin, RoleCaster:
	default:
		return errInvalid("Staff are admins or casters.")
	}
	out := t.Staff[:0]
	for _, s := range t.Staff {
		if s.User.UserID != user.UserID {
			out = append(out, s)
		}
	}
	t.Staff = out
	if role != RoleNone {
		if len(t.Staff) >= MaxStaff {
			return errInvalid("At most %d staff.", MaxStaff)
		}
		t.Staff = append(t.Staff, StaffMember{User: publicRef(user), Role: role})
	}
	t.audit(now, a, "staff", user.Handle+" "+string(role))
	return nil
}
