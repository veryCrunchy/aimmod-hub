// Package tournament holds AimMod tournaments: registration, check-in,
// seeding, bracket generation (package bracket), match series with picks and
// bans, result reporting, confirmation, disputes and organiser overrides.
//
// A Tournament is one aggregate, loaded and saved whole with an optimistic
// version. Every method takes the current time, so deadlines (check-in,
// no-shows, confirmation windows) are applied deterministically by Tick on
// each read and write instead of by a background job.
package tournament

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/veryCrunchy/aimmod-hub/api/internal/tournament/bracket"
)

type Status string

const (
	Draft        Status = "draft"
	Registration Status = "registration"
	CheckIn      Status = "check_in"
	InProgress   Status = "in_progress"
	Completed    Status = "completed"
	Cancelled    Status = "cancelled"
)

type SeedingMethod string

const (
	SeedManual    SeedingMethod = "manual"
	SeedRandom    SeedingMethod = "random"
	SeedBenchmark SeedingMethod = "benchmark_rank"
	SeedScenario  SeedingMethod = "scenario_pb"
)

type Scheduling string

const (
	ReadyWhenOnline Scheduling = "ready"
	Scheduled       Scheduling = "scheduled"
)

type EntrantStatus string

const (
	EntrantRegistered   EntrantStatus = "registered"
	EntrantCheckedIn    EntrantStatus = "checked_in"
	EntrantActive       EntrantStatus = "active"
	EntrantEliminated   EntrantStatus = "eliminated"
	EntrantDisqualified EntrantStatus = "disqualified"
	EntrantWithdrawn    EntrantStatus = "withdrawn"
	EntrantNoShow       EntrantStatus = "no_show"
)

type Role string

const (
	RoleNone      Role = ""
	RoleOrganiser Role = "organiser"
	RoleAdmin     Role = "admin"
	RoleCaster    Role = "caster"
)

// Limits keep one aggregate small enough to load and save whole.
const (
	MaxEntrants     = 256
	MaxPool         = 16
	MaxVetoSteps    = 16
	MaxStaff        = 16
	MaxInvites      = 512
	MaxAudit        = 300
	MaxNameLength   = 80
	MaxTextLength   = 2000
	MaxReasonLength = 500
	DefaultConfirm  = 15 * time.Minute
)

// UserRef identifies a Hub account. UserID never leaves the server; the
// external id, handle and names are public profile data.
type UserRef struct {
	UserID      int64  `json:"userId"`
	ExternalID  string `json:"externalId"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
	SteamID     string `json:"steamId,omitempty"`
}

// Actor is the caller of an operation.
type Actor struct {
	UserRef
	// HubAdmin: the Hub's own administrator, who can act on every tournament.
	HubAdmin bool
	// Verified: the account has a verified linked game account.
	Verified bool
}

func (a Actor) SignedIn() bool { return a.UserID != 0 }

type PoolScenario struct {
	Name      string `json:"name"`
	Hash      string `json:"hash,omitempty"`
	TimeLimit int    `json:"timeLimit,omitempty"`
}

type VetoAction string

const (
	Ban     VetoAction = "ban"
	Pick    VetoAction = "pick"
	Decider VetoAction = "decider"
)

type VetoActor string

const (
	HigherSeed VetoActor = "higher"
	LowerSeed  VetoActor = "lower"
)

type VetoStep struct {
	Action VetoAction `json:"action"`
	Actor  VetoActor  `json:"actor"`
}

type RoundBestOf struct {
	Side      bracket.Side `json:"side"`
	FromRound int          `json:"fromRound"` // 0: the side's last round
	BestOf    int          `json:"bestOf"`
}

type Ruleset struct {
	GameMode       string         `json:"gameMode"`
	BestOf         int            `json:"bestOf"`
	RoundBestOf    []RoundBestOf  `json:"roundBestOf,omitempty"`
	Pool           []PoolScenario `json:"pool"`
	Veto           []VetoStep     `json:"veto,omitempty"`
	Countdown      int            `json:"countdown"`
	Spectators     bool           `json:"spectators"`
	RequireReplays bool           `json:"requireReplays"`
	ConfirmMinutes int            `json:"confirmMinutes"`
}

// Spec is everything an organiser sets when creating or editing.
type Spec struct {
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Format        bracket.Format  `json:"format"`
	InviteOnly    bool            `json:"inviteOnly"`
	MaxEntrants   int             `json:"maxEntrants"`
	Seeding       SeedingMethod   `json:"seeding"`
	SeedingSource string          `json:"seedingSource,omitempty"`
	Scheduling    Scheduling      `json:"scheduling"`
	CheckInOpens  *time.Time      `json:"checkInOpens,omitempty"`
	CheckInCloses *time.Time      `json:"checkInCloses,omitempty"`
	StartsAt      *time.Time      `json:"startsAt,omitempty"`
	ReadyWindow   int             `json:"readyWindow"` // minutes a ready match waits for the second player
	NoShow        int             `json:"noShow"`      // minutes after a scheduled start before a no-show forfeit
	Ruleset       Ruleset         `json:"ruleset"`
	Options       bracket.Options `json:"options"`
	Prize         string          `json:"prize,omitempty"`
}

type Entrant struct {
	ID           string        `json:"id"`
	User         UserRef       `json:"user"`
	Seed         int           `json:"seed"`
	Status       EntrantStatus `json:"status"`
	RegisteredAt time.Time     `json:"registeredAt"`
	CheckedInAt  *time.Time    `json:"checkedInAt,omitempty"`
	Rating       *float64      `json:"rating,omitempty"`
	RatingLabel  string        `json:"ratingLabel,omitempty"`
}

type StaffMember struct {
	User UserRef `json:"user"`
	Role Role    `json:"role"`
}

type AuditEntry struct {
	At     time.Time `json:"at"`
	Actor  UserRef   `json:"actor"`
	Action string    `json:"action"`
	Detail string    `json:"detail,omitempty"`
}

type DisputeStatus string

const (
	DisputeOpen     DisputeStatus = "open"
	DisputeResolved DisputeStatus = "resolved"
)

type DisputeDecision string

const (
	DecisionUphold   DisputeDecision = "uphold"
	DecisionOverturn DisputeDecision = "overturn"
	DecisionReplay   DisputeDecision = "replay"
)

type Dispute struct {
	ID         string          `json:"id"`
	Match      string          `json:"match"`
	OpenedBy   string          `json:"openedBy"` // entrant id
	Reason     string          `json:"reason"`
	Status     DisputeStatus   `json:"status"`
	Decision   DisputeDecision `json:"decision,omitempty"`
	Note       string          `json:"note,omitempty"`
	CreatedAt  time.Time       `json:"createdAt"`
	ResolvedAt *time.Time      `json:"resolvedAt,omitempty"`
	ResolvedBy *UserRef        `json:"resolvedBy,omitempty"`
}

// Tournament is the aggregate stored as one document.
type Tournament struct {
	ID        string             `json:"id"`
	Slug      string             `json:"slug"`
	Spec      Spec               `json:"spec"`
	Status    Status             `json:"status"`
	Organiser UserRef            `json:"organiser"`
	Staff     []StaffMember      `json:"staff,omitempty"`
	Invites   []string           `json:"invites,omitempty"` // lower-case handles
	Entrants  []*Entrant         `json:"entrants"`
	Bracket   *bracket.Bracket   `json:"bracket,omitempty"`
	Series    map[string]*Series `json:"series,omitempty"`
	Disputes  []*Dispute         `json:"disputes,omitempty"`
	Uploads   map[string]*Upload `json:"uploads,omitempty"`
	Audit     []AuditEntry       `json:"audit,omitempty"`
	// RandomSeed records the draw for random seeding, so it can be audited.
	RandomSeed int64     `json:"randomSeed,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Version    int64     `json:"version"`
	nextID     int
}

// Error carries a category the API maps to a status code.
type Error struct {
	Kind    Kind
	Message string
}

type Kind int

const (
	KindInvalid Kind = iota + 1
	KindForbidden
	KindState
	KindNotFound
	KindUnauthenticated
)

func (e *Error) Error() string { return e.Message }

func errInvalid(format string, a ...any) error {
	return &Error{KindInvalid, fmt.Sprintf(format, a...)}
}
func errForbidden(msg string) error { return &Error{KindForbidden, msg} }
func errState(format string, a ...any) error {
	return &Error{KindState, fmt.Sprintf(format, a...)}
}
func errNotFound(msg string) error { return &Error{KindNotFound, msg} }

var ErrSignIn = &Error{KindUnauthenticated, "Sign in to AimMod Hub first."}

// KindOf returns an error's category (0 when it is not a tournament error).
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return 0
}

func clean(s string, max int) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > max {
		s = strings.TrimSpace(string(r[:max]))
	}
	return s
}

func cleanLine(s string, max int) string {
	return strings.Join(strings.Fields(clean(s, max)), " ")
}

// Normalize fills defaults and checks a spec. Times must come in UTC or
// with an offset; they are stored in UTC.
func (s Spec) Normalize() (Spec, error) {
	s.Name = cleanLine(s.Name, MaxNameLength)
	s.Description = clean(s.Description, MaxTextLength)
	s.Prize = clean(s.Prize, 500)
	if len([]rune(s.Name)) < 3 {
		return s, errInvalid("Give the tournament a name of at least 3 characters.")
	}
	switch s.Format {
	case bracket.SingleElimination, bracket.DoubleElimination, bracket.RoundRobin, bracket.Swiss:
	case "":
		s.Format = bracket.SingleElimination
	default:
		return s, errInvalid("Unknown format.")
	}
	if s.MaxEntrants == 0 {
		s.MaxEntrants = 32
	}
	if s.MaxEntrants < 2 || s.MaxEntrants > MaxEntrants {
		return s, errInvalid("Entrants must be between 2 and %d.", MaxEntrants)
	}
	if s.Format == bracket.RoundRobin && s.MaxEntrants > 32 {
		return s, errInvalid("Round robin is limited to 32 entrants.")
	}
	switch s.Seeding {
	case "":
		s.Seeding = SeedRandom
	case SeedManual, SeedRandom:
	case SeedBenchmark, SeedScenario:
		s.SeedingSource = cleanLine(s.SeedingSource, 200)
		if s.SeedingSource == "" {
			return s, errInvalid("Choose the benchmark or scenario to seed by.")
		}
	default:
		return s, errInvalid("Unknown seeding method.")
	}
	switch s.Scheduling {
	case "":
		s.Scheduling = ReadyWhenOnline
	case ReadyWhenOnline, Scheduled:
	default:
		return s, errInvalid("Unknown scheduling mode.")
	}
	if s.ReadyWindow == 0 {
		s.ReadyWindow = 30
	}
	if s.NoShow == 0 {
		s.NoShow = 10
	}
	if s.ReadyWindow < 5 || s.ReadyWindow > 24*60 || s.NoShow < 2 || s.NoShow > 120 {
		return s, errInvalid("Ready window must be 5 minutes to a day, no-show grace 2 to 120 minutes.")
	}
	for _, t := range []**time.Time{&s.CheckInOpens, &s.CheckInCloses, &s.StartsAt} {
		if *t != nil {
			u := (*t).UTC().Truncate(time.Second)
			*t = &u
		}
	}
	if (s.CheckInOpens == nil) != (s.CheckInCloses == nil) {
		return s, errInvalid("Check-in needs both an opening and a closing time.")
	}
	if s.CheckInOpens != nil && !s.CheckInCloses.After(*s.CheckInOpens) {
		return s, errInvalid("Check-in must close after it opens.")
	}
	if s.StartsAt != nil && s.CheckInCloses != nil && s.StartsAt.Before(*s.CheckInCloses) {
		return s, errInvalid("The tournament can't start before check-in closes.")
	}
	if s.Scheduling == Scheduled && s.StartsAt == nil {
		return s, errInvalid("Scheduled tournaments need a start time.")
	}
	if s.Options.Cycles == 0 {
		s.Options.Cycles = 1
	}
	if s.Options.Cycles < 1 || s.Options.Cycles > 2 {
		return s, errInvalid("Round robin plays each pair once or twice.")
	}
	if s.Options.SwissRounds < 0 || s.Options.SwissRounds > 15 {
		return s, errInvalid("Swiss rounds must be between 1 and 15.")
	}
	r, err := s.Ruleset.normalize()
	if err != nil {
		return s, err
	}
	s.Ruleset = r
	return s, nil
}

func validBestOf(n int) bool { return n >= 1 && n <= 9 && n%2 == 1 }

func (r Ruleset) normalize() (Ruleset, error) {
	if r.GameMode == "" {
		r.GameMode = "score-race"
	}
	if r.GameMode != "score-race" {
		// Head-to-head modes (tracking duel) follow once the lobby supports them in a series.
		return r, errInvalid("Tournament games are score races for now.")
	}
	if r.BestOf == 0 {
		r.BestOf = 1
	}
	if !validBestOf(r.BestOf) {
		return r, errInvalid("Best of must be an odd number from 1 to 9.")
	}
	for i, rb := range r.RoundBestOf {
		if !validBestOf(rb.BestOf) || rb.FromRound < 0 {
			return r, errInvalid("Round best-of %d is invalid.", i+1)
		}
	}
	if len(r.Pool) == 0 {
		return r, errInvalid("Add at least one scenario to the pool.")
	}
	if len(r.Pool) > MaxPool {
		return r, errInvalid("The pool holds at most %d scenarios.", MaxPool)
	}
	seen := map[string]bool{}
	for i := range r.Pool {
		p := &r.Pool[i]
		p.Name = cleanLine(p.Name, 128)
		p.Hash = strings.ToLower(strings.TrimSpace(p.Hash))
		if p.Name == "" {
			return r, errInvalid("Pool scenario %d has no name.", i+1)
		}
		key := strings.ToLower(p.Name)
		if seen[key] {
			return r, errInvalid("%s is in the pool twice.", p.Name)
		}
		seen[key] = true
		if p.TimeLimit != 0 && (p.TimeLimit < 10 || p.TimeLimit > 600) {
			return r, errInvalid("Time limits are 10 to 600 seconds.")
		}
		for _, c := range p.Hash {
			if !strings.ContainsRune("0123456789abcdef", c) || len(p.Hash) > 64 {
				return r, errInvalid("Scenario hashes are hexadecimal.")
			}
		}
	}
	if len(r.Veto) > MaxVetoSteps {
		return r, errInvalid("At most %d veto steps.", MaxVetoSteps)
	}
	bans, picks := 0, 0
	for _, step := range r.Veto {
		switch step.Action {
		case Ban:
			bans++
		case Pick:
			picks++
		default:
			return r, errInvalid("Veto steps are bans or picks.")
		}
		if step.Actor != HigherSeed && step.Actor != LowerSeed {
			return r, errInvalid("Each veto step belongs to the higher or the lower seed.")
		}
	}
	if len(r.Veto) > 0 {
		left := len(r.Pool) - bans - picks
		if left < 1 {
			return r, errInvalid("The veto must leave a decider scenario.")
		}
		// Every game needs its own scenario: picks first, then what remains.
		if len(r.Pool)-bans < r.BestOf {
			return r, errInvalid("A best of %d needs %d scenarios left after bans.", r.BestOf, r.BestOf)
		}
		// Longer rounds (a best-of-5 final) use the standard veto, which needs the pool to cover them.
		for _, rb := range r.RoundBestOf {
			if rb.BestOf > len(r.Pool) {
				return r, errInvalid("A best of %d needs at least %d scenarios in the pool.", rb.BestOf, rb.BestOf)
			}
		}
	}
	if r.Countdown == 0 {
		r.Countdown = 5
	}
	if r.Countdown < 3 || r.Countdown > 10 {
		return r, errInvalid("Countdown is 3 to 10 seconds.")
	}
	if r.ConfirmMinutes == 0 {
		r.ConfirmMinutes = int(DefaultConfirm / time.Minute)
	}
	if r.ConfirmMinutes < 1 || r.ConfirmMinutes > 24*60 {
		return r, errInvalid("Confirmation window is 1 minute to a day.")
	}
	return r, nil
}

// DefaultVeto: alternate bans (higher seed first) down to best-of
// scenarios, then alternate picks for all but the last game; the last game
// is the decider.
func DefaultVeto(pool, bestOf int) []VetoStep {
	var steps []VetoStep
	actor := func(i int) VetoActor {
		if i%2 == 0 {
			return HigherSeed
		}
		return LowerSeed
	}
	n := 0
	for i := 0; i < pool-bestOf; i++ {
		steps = append(steps, VetoStep{Ban, actor(n)})
		n++
	}
	for i := 0; i < bestOf-1; i++ {
		steps = append(steps, VetoStep{Pick, actor(i)})
	}
	return steps
}

func (t *Tournament) newID(prefix string) string {
	if t.nextID == 0 {
		// Rebuild the counter after loading: ids are prefix + number.
		for _, e := range t.Entrants {
			t.bump(e.ID)
		}
		for _, d := range t.Disputes {
			t.bump(d.ID)
		}
		for id := range t.Uploads {
			t.bump(id)
		}
	}
	t.nextID++
	return fmt.Sprintf("%s%d", prefix, t.nextID)
}

func (t *Tournament) bump(id string) {
	i := len(id)
	for i > 0 && id[i-1] >= '0' && id[i-1] <= '9' {
		i--
	}
	if n, err := strconv.Atoi(id[i:]); err == nil && n > t.nextID {
		t.nextID = n
	}
}

func (t *Tournament) audit(now time.Time, a Actor, action, detail string) {
	t.Audit = append(t.Audit, AuditEntry{At: now, Actor: publicRef(a.UserRef), Action: action, Detail: detail})
	if len(t.Audit) > MaxAudit {
		t.Audit = t.Audit[len(t.Audit)-MaxAudit:]
	}
}

// publicRef strips what must not be stored in shared places (the Steam id).
func publicRef(u UserRef) UserRef {
	u.SteamID = ""
	return u
}

// RoleOf: the caller's staff role in this tournament.
func (t *Tournament) RoleOf(a Actor) Role {
	if !a.SignedIn() {
		return RoleNone
	}
	if a.UserID == t.Organiser.UserID {
		return RoleOrganiser
	}
	for _, s := range t.Staff {
		if s.User.UserID == a.UserID {
			return s.Role
		}
	}
	if a.HubAdmin {
		return RoleAdmin
	}
	return RoleNone
}

// CanManage: organiser, tournament admins and the Hub admin.
func (t *Tournament) CanManage(a Actor) bool {
	r := t.RoleOf(a)
	return r == RoleOrganiser || r == RoleAdmin || a.HubAdmin
}

// CanSeePrivate: managers and casters (seeds of running games, live state).
func (t *Tournament) CanSeePrivate(a Actor) bool {
	return t.CanManage(a) || t.RoleOf(a) == RoleCaster
}

func (t *Tournament) requireManager(a Actor) error {
	if !a.SignedIn() {
		return ErrSignIn
	}
	if !t.CanManage(a) {
		return errForbidden("Only the tournament's organisers can do that.")
	}
	return nil
}

func (t *Tournament) Entrant(id string) *Entrant {
	for _, e := range t.Entrants {
		if e.ID == id {
			return e
		}
	}
	return nil
}

func (t *Tournament) EntrantOf(userID int64) *Entrant {
	if userID == 0 {
		return nil
	}
	for _, e := range t.Entrants {
		if e.User.UserID == userID {
			return e
		}
	}
	return nil
}

// Active entrants are still in the field (not withdrawn, disqualified or no-shows).
func (e *Entrant) Active() bool {
	switch e.Status {
	case EntrantWithdrawn, EntrantDisqualified, EntrantNoShow:
		return false
	}
	return true
}

func (t *Tournament) Invited(handle string) bool {
	h := strings.ToLower(strings.TrimSpace(handle))
	for _, i := range t.Invites {
		if i == h {
			return true
		}
	}
	return false
}

func (t *Tournament) CheckInRequired() bool { return t.Spec.CheckInOpens != nil }

// Slugify turns a name into a URL part; the store appends a suffix when taken.
func Slugify(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
		if b.Len() >= 48 {
			break
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		s = "tournament"
	}
	return s
}
