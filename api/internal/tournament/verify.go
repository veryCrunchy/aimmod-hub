package tournament

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/tournament/bracket"
)

// Replay verification on the Hub.
//
// AimMod replays (format 2, in-game/native-mod/DESIGN.md "Replay format 2")
// start with "AMRPLAY2", a u32 version, a u32 header length and a JSON
// header; then a u32 compression algorithm, a u32 body size and the body,
// compressed with the Windows Compression API (LZMS). The Hub decodes the
// header and framing here and checks them against the reported game. The
// body (inputs, camera and target tracks) is checked by a Windows verifier
// worker in a later phase; until then the target spawn sequence can't be
// compared with the game's seed and replays say so in their checks.

const (
	MaxReplayBytes  = 8 << 20
	replayMagic     = "AMRPLAY2"
	maxHeaderLength = 64 << 10
)

// ReplayInfo is what the header says about the run.
type ReplayInfo struct {
	ID          string    `json:"id"`
	Scenario    string    `json:"scenario"`
	Reason      string    `json:"reason"`
	Score       float64   `json:"score"`
	Duration    float64   `json:"duration"`
	Frames      int       `json:"frames"`
	InputEvents int       `json:"inputEvents"`
	RecordedAt  time.Time `json:"recordedAt"`
	// Seed, when the game wrote one (AimModCore's seeded starts).
	Seed      string `json:"seed,omitempty"`
	Algorithm uint32 `json:"algorithm"`
	BodySize  uint32 `json:"bodySize"`
}

var ErrNotReplay = errors.New("not an AimMod format 2 replay")

// DecodeReplayHeader reads and checks the header and framing of a replay file.
func DecodeReplayHeader(data []byte) (ReplayInfo, error) {
	var info ReplayInfo
	if len(data) < 24 || !bytes.Equal(data[:8], []byte(replayMagic)) {
		return info, ErrNotReplay
	}
	if v := binary.LittleEndian.Uint32(data[8:]); v != 2 {
		return info, fmt.Errorf("unsupported replay version %d", v)
	}
	n := int(binary.LittleEndian.Uint32(data[12:]))
	if n <= 0 || n > maxHeaderLength || 16+n+8 > len(data) {
		return info, errors.New("invalid replay header length")
	}
	var h struct {
		Kind        string          `json:"kind"`
		Version     int             `json:"version"`
		ID          string          `json:"id"`
		Coordinates string          `json:"coordinates"`
		RecordedAt  string          `json:"recordedAt"`
		Scenario    string          `json:"scenario"`
		Reason      string          `json:"reason"`
		Frames      int             `json:"frames"`
		InputEvents int             `json:"inputEvents"`
		Score       *float64        `json:"score"`
		Duration    *float64        `json:"duration"`
		Seed        json.RawMessage `json:"seed"`
	}
	if err := json.Unmarshal(data[16:16+n], &h); err != nil {
		return info, errors.New("invalid replay header")
	}
	if h.Kind != "header" || h.Version != 2 || h.Coordinates != "unreal-centimeters" || h.ID == "" {
		return info, errors.New("unsupported replay header")
	}
	at, err := time.Parse(time.RFC3339Nano, h.RecordedAt)
	if err != nil {
		return info, errors.New("invalid replay time")
	}
	info = ReplayInfo{ID: h.ID, Scenario: strings.TrimSpace(h.Scenario), Reason: h.Reason, Frames: h.Frames, InputEvents: h.InputEvents, RecordedAt: at.UTC()}
	if h.Score != nil {
		info.Score = *h.Score
	}
	if h.Duration != nil {
		info.Duration = *h.Duration
	}
	if len(h.Seed) > 0 {
		var s string
		if json.Unmarshal(h.Seed, &s) == nil {
			info.Seed = s
		} else {
			var u uint64
			if json.Unmarshal(h.Seed, &u) == nil {
				info.Seed = strconv.FormatUint(u, 10)
			}
		}
	}
	at2 := 16 + n
	info.Algorithm = binary.LittleEndian.Uint32(data[at2:])
	info.BodySize = binary.LittleEndian.Uint32(data[at2+4:])
	if info.BodySize == 0 || info.BodySize > 256<<20 || len(data)-(at2+8) <= 0 {
		return info, errors.New("replay has no body")
	}
	if math.IsNaN(info.Score) || math.IsInf(info.Score, 0) || math.IsNaN(info.Duration) || math.IsInf(info.Duration, 0) {
		return info, errors.New("invalid replay stats")
	}
	return info, nil
}

// Upload is a replay file stored for a game.
type Upload struct {
	ID      string     `json:"id"`
	Entrant string     `json:"entrant"`
	Match   string     `json:"match"`
	Game    int        `json:"game"`
	SHA256  string     `json:"sha256"`
	Size    int64      `json:"size"`
	Key     string     `json:"key"`
	At      time.Time  `json:"at"`
	Info    ReplayInfo `json:"info"`
	Status  string     `json:"status"` // pending, verified, suspicious, rejected
	Checks  []string   `json:"checks,omitempty"`
}

const (
	ReplayPending    = "pending"
	ReplayVerified   = "verified"
	ReplaySuspicious = "suspicious"
	ReplayRejected   = "rejected"
)

// AddUpload attaches a stored replay to a game the actor plays in. One file
// per player and game: a new upload replaces the previous one.
func (t *Tournament) AddUpload(a Actor, matchID string, game int, sha string, size int64, key string, info ReplayInfo, now time.Time) (*Upload, string, error) {
	if !a.SignedIn() {
		return nil, "", ErrSignIn
	}
	m, s, err := t.match(matchID)
	if err != nil {
		return nil, "", err
	}
	slot := t.slotOf(m, a)
	if slot < 0 {
		return nil, "", errForbidden("You aren't playing in this match.")
	}
	if s == nil || game < 0 || game >= len(s.Games) {
		return nil, "", errInvalid("That game hasn't started.")
	}
	entrant := m.Slots[slot].Entrant
	replaced := ""
	for id, u := range t.Uploads {
		if u.Match == matchID && u.Game == game && u.Entrant == entrant {
			replaced = u.Key
			delete(t.Uploads, id)
			g := s.Games[game]
			out := g.Replays[:0]
			for _, r := range g.Replays {
				if r != id {
					out = append(out, r)
				}
			}
			g.Replays = out
		}
	}
	if t.Uploads == nil {
		t.Uploads = map[string]*Upload{}
	}
	u := &Upload{ID: t.newID("r"), Entrant: entrant, Match: matchID, Game: game, SHA256: sha, Size: size, Key: key, At: now, Info: info, Status: ReplayPending}
	t.Uploads[u.ID] = u
	if g := s.Games[game]; g.HasScore {
		g.Replays = append(g.Replays, u.ID)
		t.verifyGame(m, s, g)
	}
	return u, replaced, nil
}

// verifyGame checks every replay of a reported game against the report.
func (t *Tournament) verifyGame(m *bracket.Match, s *Series, g *Game) {
	clearFlags := g.Flags[:0]
	for _, f := range g.Flags {
		if !strings.HasPrefix(f, "replay-") {
			clearFlags = append(clearFlags, f)
		}
	}
	g.Flags = clearFlags
	for _, id := range g.Replays {
		u := t.Uploads[id]
		if u == nil {
			continue
		}
		slot := 0
		if m.Slots[1].Entrant == u.Entrant {
			slot = 1
		}
		u.Status, u.Checks = CheckReplay(u.Info, g, g.Scores[slot], s.StartedAt, g.ReportedAt)
		switch u.Status {
		case ReplaySuspicious:
			g.addFlag("replay-suspicious")
		case ReplayRejected:
			g.addFlag("replay-rejected")
		}
	}
}

// CheckReplay compares a replay header with its game. Failed checks make a
// replay suspicious (an organiser decides); a replay of another scenario,
// or with no inputs, is rejected.
func CheckReplay(info ReplayInfo, g *Game, reported float64, started, reportedAt *time.Time) (string, []string) {
	var checks []string
	status := ReplayVerified
	fail := func(level, check string) {
		checks = append(checks, check)
		if level == ReplayRejected || status == ReplayVerified {
			status = level
		}
	}
	played := g.PlayedScenario
	if played == "" {
		played = g.Scenario
	}
	if !strings.EqualFold(info.Scenario, played) && !strings.EqualFold(info.Scenario, g.Scenario) {
		fail(ReplayRejected, "scenario: the replay is of "+info.Scenario)
	} else {
		checks = append(checks, "scenario matches")
	}
	if info.InputEvents <= 0 || info.Frames < 2 {
		fail(ReplayRejected, "inputs: the replay has no recorded input")
	}
	if info.Reason != "completed" {
		fail(ReplaySuspicious, "the run did not complete ("+info.Reason+")")
	}
	if tol := math.Max(1, math.Abs(reported)*0.005); math.Abs(info.Score-reported) > tol {
		fail(ReplaySuspicious, fmt.Sprintf("score: the replay ends on %.1f, the report says %.1f", info.Score, reported))
	} else {
		checks = append(checks, "score matches")
	}
	if g.TimeLimit > 0 && info.Reason == "completed" && math.Abs(info.Duration-float64(g.TimeLimit)) > 3 {
		fail(ReplaySuspicious, fmt.Sprintf("length: %.0f s played, the game is %d s", info.Duration, g.TimeLimit))
	}
	if info.Duration > 0 {
		if fps := float64(info.Frames) / info.Duration; fps < 20 || fps > 2000 {
			fail(ReplaySuspicious, fmt.Sprintf("frame rate: %.0f frames per second is implausible", fps))
		}
	}
	if started != nil && info.RecordedAt.Before(started.Add(-10*time.Minute)) {
		fail(ReplaySuspicious, "time: recorded before the match started")
	}
	if reportedAt != nil && info.RecordedAt.After(reportedAt.Add(10*time.Minute)) {
		fail(ReplaySuspicious, "time: recorded after the game was reported")
	}
	switch {
	case info.Seed == "":
		checks = append(checks, "seed: not in this replay (needs a seeded AimModCore)")
	case info.Seed != strconv.FormatUint(g.Seed, 10):
		fail(ReplaySuspicious, "seed: the replay was played with another seed")
	default:
		checks = append(checks, "seed matches")
	}
	checks = append(checks, "target sequence: checked by the replay verifier (later phase)")
	return status, checks
}
