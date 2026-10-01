# Tournaments

AimMod Hub runs tournaments for AimMod for KovaaK's: registration, check-in,
seeding, brackets, match series with picks and bans, result reporting,
confirmation, disputes and organiser overrides. The in-game service talks to
this API; matches are played in AimMod's own multiplayer lobbies.

The full design, including the in-game match flow, anti-cheat, spectating and
casting, lives in the AimMod repository at `in-game/docs/tournaments.md`.
This document covers what the Hub owns.

**Policy.** Tournament games never touch KovaaK's ranked leaderboards: every
game is played in freeplay, in an AimMod lobby. Results count only after
AimMod verifies them: the lobby host validates each game, the replays are
uploaded here and checked, and the other player confirms.

## Code

| Path | What |
|---|---|
| `proto/aimmod/tournament/v1/tournament.proto` | Contract: `TournamentService` and its messages. |
| `api/internal/tournament/bracket` | Pure bracket engine: single and double elimination, round robin, Swiss, standings. |
| `api/internal/tournament` | The tournament aggregate: lifecycle, seeding, series, disputes, overrides, replay checks. No storage, no clock of its own. |
| `api/internal/tournament/tournamentapi` | Connect handler, replay upload and download, live match state. |
| `api/internal/store/tournaments.go` | Postgres persistence. |
| `api/internal/http/tournaments.go` | Wiring: authentication, benchmark ranks, routes. |
| `web/src/pages/Tournament*.tsx`, `web/src/components/TournamentBracket.tsx` | Website: list, create, event page with bracket, match page, live overview. |

## Formats

The engine takes entrants in seed order and produces matches with explicit
sources (seed, winner of, loser of), so any client can draw the bracket.

- **Single elimination.** Standard line-up (1 v 16, 8 v 9, ...) so the top two
  seeds can only meet in the final. Fields that aren't a power of two give
  byes to the top seeds; a bye is a walkover. Optional bronze match.
- **Double elimination.** A winners bracket of `k` rounds and a losers
  bracket of `2(k-1)` rounds. Losers-bracket odd rounds pair survivors, even
  rounds drop winners-bracket losers in, reversed on alternate rounds so early
  opponents don't meet again straight away. Grand final with an optional
  reset: the second final is played only when the losers-bracket champion
  wins the first.
- **Round robin.** Circle method; odd fields rest one player per round. Once
  or twice per pair. Standings: wins, then head-to-head for a two-way tie,
  game difference, games won, seed. Head-to-head isn't used for bigger ties,
  where it can be cyclic.
- **Swiss.** Round 1 pairs the top half of the seeds against the bottom half.
  Later rounds pair the ranked table top-down with the nearest opponent not
  met yet, backtracking when needed. Odd fields give one bye per round to the
  lowest-ranked player without one; a bye is a win with no games. The next
  round is paired as soon as the current one is decided. Buchholz is the
  first tie-break. Default rounds: `ceil(log2(n))`.

Every match is a best-of-N series (1 to 9, odd). A ruleset can make some
rounds longer, for example a best-of-5 final.

Standings place elimination entrants by the stage they went out. Players who
go out at the same stage share a place: both semi-final losers are 3rd
without a bronze match.

## Lifecycle

```
draft -> registration -> check_in -> in_progress -> completed
   \__________\_____________\____________\-> cancelled
```

- **Draft.** Only the organiser and staff see it.
- **Registration.** Open, or invite-only (by Hub handle). A cap on entrants.
  Teams come later; entrants are single players for now.
- **Check-in.** Optional window. It opens on time (applied on the next read)
  or by hand. Players who haven't checked in when the organiser starts are
  marked no-show and left out of the bracket.
- **Start.** Seeds the field and generates the bracket. Seeds can still change
  (and the bracket is generated again) until the first match starts.
- **In progress.** Matches open as their entrants become known.
- **Completed.** When the bracket has a result. Organisers can still correct
  results; the tournament reopens if a correction needs more matches.

### Seeding

- **Manual**: the order the organiser sets; anyone unseeded follows by
  registration time.
- **Random**: a draw from a recorded 63-bit seed, so it can be audited and
  reproduced.
- **Benchmark rank**: the player's overall rank on a KovaaK's benchmark
  (`seeding_source` is the benchmark id), read through the Hub's existing
  KovaaK's benchmark client with the player's linked Steam id. The sum of
  scenario ranks breaks ties. Unranked players go last.
- **Scenario PB**: the best score on one scenario among the player's runs
  uploaded to the Hub (`seeding_source` is the scenario name). Players
  without a run go last.

### Scheduling, no-shows and confirmation

There's no background job. Every read and write applies whatever time has
decided, in order (`Tournament.Tick`), and saves when something changed:

- **Ready when online** (default): a match opens when both entrants are known
  and waits `ready_window_minutes` (30) for both players to mark ready.
- **Scheduled**: first-round matches wait for the start time plus
  `no_show_minutes` (10). Later rounds behave like ready-when-online.
- At the deadline, a player who marked ready wins by no-show. If neither did,
  the match is flagged for the organisers.
- After the last game is reported, the opponent has `confirm_minutes` (15) to
  confirm or dispute. When the window passes the result is accepted, unless
  something needs review: a seed mismatch, a suspicious or rejected replay, a
  host-flagged dispute, or missing replays when the ruleset requires them.

## Match flow

1. Both players mark ready (`MarkReady`), saying whether their client can host
   a Steam lobby. The **host** is the higher seed, unless only the other
   player can host. That client creates the lobby and invites the opponent.
2. **Veto** (`SubmitVeto`), when the ruleset has one: bans and picks by seed
   (higher or lower). The scenario left over is the decider. Games are the
   picks in order, then the decider. A custom veto is written for the
   ruleset's best-of; longer rounds use the standard veto (alternate bans
   from the higher seed down to N scenarios, then alternate picks). Without a
   veto, the games follow the pool in order.
3. **Games.** Each game gets a fresh 32-bit **seed** (the range AimModCore's
   `start-scenario` accepts) from a cryptographic
   source. Both players play the game with that seed, so the targets are the
   same for both. The seed is shown to the players and staff once the game
   starts, and to everyone after the match.
4. **Report** (`ReportGame`), usually by the host's client, with the full
   result record (below), the replay ids and whether the host validated the
   game. A tied game is replayed on the same scenario with a new seed.
5. When a player reaches the wins needed, the opponent **confirms**
   (`ConfirmResult`) or **disputes** (`OpenDispute`). Confirmation advances
   the bracket.

### Result record

`ReportGame.result` is the per-game record proposed in the AimMod
repository's `in-game/docs/game-modes.md` section 8.4, adopted with three
changes: each player also carries an `entrant_id` (the hashed player key is
only meaningful to the local history), the game's `seed` is echoed, and times
are Unix milliseconds. When the record is present the scores come from it,
and it must name exactly the match's two entrants. Mode-specific fields
(frags, deaths, claims, rejected, track percent) are stored for the
head-to-head modes that will follow score races.

### Disputes and overrides

- A dispute stops the match. Organisers see both replays (download is limited
  to the match's players and the staff) and decide: **uphold** the report,
  **overturn** it (set the winner and score), or **replay** the match.
- `SetMatchResult` sets any result: an override, a forfeit or a no-show.
  Changing the winner of a finished match resets every later match that
  depended on it; the reset matches are returned so clients can tell players.
- `Disqualify` forfeits the entrant's current and future matches. Results
  already played stand.

## Verification on the Hub

Replays are uploaded with `POST /api/tournaments/v1/replays?tournament=&match=&game=`
(bearer token or session; at most 8 MB; an optional `X-Content-SHA256` must
match). The file goes to the Hub's media storage (local or S3) under
`tournaments/<tournament>/<match>/`. One replay per player and game; a new
upload replaces the old one.

The Hub decodes the format 2 header and framing and checks them against the
reported game:

- the replay is of the game's scenario (or the generated match scenario that
  was played);
- it has recorded input and the run completed;
- the final score is within 0.5% (or 1 point) of the reported score;
- the length matches the game's time limit;
- the frame rate is plausible (20 to 2000 frames per second);
- it was recorded during the match;
- the seed in the header matches the game's seed, once AimModCore writes one.

A failed check makes the replay *suspicious* (an organiser decides); a replay
of another scenario, or with no input, is *rejected*. The replay body is
LZMS-compressed (Windows Compression API), so the deep checks (the target
spawn sequence against the seed, hit validation from inputs and camera, view
speed) belong to a Windows verifier worker in a later phase. Replays say so
in their checks.

## Live state and the overview

The match host's client pushes a live snapshot every few seconds
(`ReportLiveState`): phase, game, and per player the score, accuracy, time
left, ping and connection. It's kept in memory for 20 seconds and served by
`GetOverview` (every running match at once) and `GetMatch`. Several Hub
instances would need a shared store for it (Redis or a small table).

## Data model

One row per tournament in `tournaments` with the whole aggregate as JSONB in
`doc` and an optimistic `version`, plus `tournament_members` (organiser,
staff and active entrants) for "my tournaments". Events are bounded (256
entrants, about 500 matches), so loading and saving the whole aggregate keeps
every rule in one place and every change atomic. Concurrent writes retry on
a version conflict. If events grow, matches and games can move to their own
tables without changing the API.

The aggregate holds: the spec (format, options, ruleset, schedule, seeding),
entrants (Hub user, seed, status, check-in, rating used for seeding), the
bracket, one series per match (readiness, host, veto log, games with seeds,
scores, replay ids, result records, flags, deadlines), disputes, replay
uploads with their checks, staff, invites and an audit log (last 300
entries).

## API

`aimmod.tournament.v1.TournamentService` (Connect; JSON or binary):

| RPC | Who | What |
|---|---|---|
| `ListTournaments` | anyone | Public events by status; `mine` for the caller's. |
| `GetTournament` | anyone | Event, entrants, matches, standings, disputes, staff and the caller's permissions. Audit and invites for staff. |
| `CreateTournament` | see below | A draft owned by the caller. |
| `UpdateTournament` | staff | Edit; format, field and seeding are fixed once the bracket is out. |
| `AdvanceTournament` | staff | Open registration, open check-in, start, cancel. |
| `Register`, `Withdraw`, `CheckIn` | players | |
| `InviteEntrants` | staff | Invite or uninvite by Hub handle. |
| `SetSeeds`, `Reseed` | staff | Until the first match starts. |
| `ListMyMatches` | players | Open matches with the ruleset, whether this client hosts, and the opponent's Steam id for the lobby invite (only to the match's players); events awaiting check-in. |
| `GetMatch` | anyone | A match with its games and live state. Seeds only to players and staff until the match ends. |
| `MarkReady`, `SubmitVeto`, `ReportGame`, `ConfirmResult`, `OpenDispute` | the match's players | |
| `ResolveDispute`, `SetMatchResult`, `Disqualify` | staff | |
| `SetStaff` | organiser | Admins (run matches and disputes) and casters (private match details). |
| `ReportLiveState` | the match's players | |
| `GetOverview` | anyone | Running matches with live state. |

Plus `POST /api/tournaments/v1/replays` (upload) and
`GET /api/tournaments/v1/replays/<tournament>/<replay>` (download, players
and staff).

### Authentication and roles

- The in-game client sends its device upload token (`Authorization: Bearer`);
  the website uses its session cookie. Cookie requests from another origin
  are treated as anonymous.
- Roles per tournament: **organiser** (creator), **admin** and **caster**
  staff. The Hub administrator (`AIMMOD_HUB_ADMIN_DISCORD_USER_ID`) can act on
  every tournament.
- Who can create tournaments is set by `AIMMOD_HUB_TOURNAMENT_CREATORS`:
  `admin` (default: the Hub administrator only), `verified` (accounts with a
  verified linked game account) or `signed-in`.

## Tests

- `go test ./api/internal/tournament/...` covers the engine (every field size
  from 2 to 70 for both elimination formats, byes, the losers bracket, the
  reset, round robin pairings and tie-breaks, Swiss without rematches,
  withdrawals, overrides, JSON round trips), the aggregate (lifecycle,
  seeding, veto, ties, no-shows, auto-confirmation, disputes, staff, replay
  checks) and the API over Connect JSON with an in-memory store.
- `AIMMOD_TOURNAMENT_TEST_DATABASE_URL=postgres://... go test ./api/internal/store -run TestTournamentPersistence`
  checks persistence, version conflicts, paging and seeding queries against a
  throwaway schema.
- `pnpm --dir web test` includes the bracket layout and form helpers.

## Not done yet

- Teams (entrants are single players).
- The Windows replay verifier worker (target sequence against the seed, hit
  validation for head-to-head modes).
- Notifications outside AimMod (Discord, email) for check-in and match calls.
- Per-match scheduling by organisers (match start times beyond round 1).
