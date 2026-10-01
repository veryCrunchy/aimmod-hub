package tournamentapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	"github.com/veryCrunchy/aimmod-hub/api/internal/tournament"
)

// Media is the Hub's blob storage (media.Storage).
type Media interface {
	Put(ctx context.Context, key string, contentType string, body io.Reader, size int64) error
	Get(ctx context.Context, key string) (io.ReadCloser, string, error)
	Delete(ctx context.Context, key string) error
}

const ReplayPath = "/api/tournaments/v1/replays"

// ReplayHandler serves:
//
//	POST ReplayPath?tournament=<id>&match=<id>&game=<index>
//	     body: the .amreplay file (format 2, at most 8 MB); optional
//	     X-Content-SHA256 header (hex) must match the body.
//	     200 {"id","status","checks"}; 422 {"error"} when it is not a replay.
//	GET  ReplayPath/<tournament>/<replay id>
//	     the file, for the match's players and the tournament's staff.
func (s *Server) ReplayHandler(media Media) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if media == nil {
			writeError(w, http.StatusServiceUnavailable, "Replay storage isn't configured.")
			return
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == ReplayPath:
			s.uploadReplay(w, r, media)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, ReplayPath+"/"):
			s.downloadReplay(w, r, media)
		default:
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed.")
		}
	})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func httpStatus(err error) (int, string) {
	var ce *connect.Error
	if errors.As(toConnect(err), &ce) {
		switch ce.Code() {
		case connect.CodeInvalidArgument:
			return http.StatusBadRequest, ce.Message()
		case connect.CodePermissionDenied:
			return http.StatusForbidden, ce.Message()
		case connect.CodeNotFound:
			return http.StatusNotFound, ce.Message()
		case connect.CodeUnauthenticated:
			return http.StatusUnauthorized, ce.Message()
		case connect.CodeFailedPrecondition, connect.CodeAborted:
			return http.StatusConflict, ce.Message()
		}
	}
	return http.StatusInternalServerError, "Something went wrong on AimMod Hub."
}

func (s *Server) uploadReplay(w http.ResponseWriter, r *http.Request, media Media) {
	a, err := s.actor(r.Context(), r.Header)
	if err != nil || !a.SignedIn() {
		writeError(w, http.StatusUnauthorized, "Sign in to AimMod Hub first.")
		return
	}
	q := r.URL.Query()
	tid, mid := q.Get("tournament"), q.Get("match")
	game, err := strconv.Atoi(q.Get("game"))
	if tid == "" || mid == "" || err != nil || game < 0 || game > 64 || len(mid) > 32 {
		writeError(w, http.StatusBadRequest, "Name the tournament, match and game.")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, tournament.MaxReplayBytes+1))
	if err != nil || len(body) > tournament.MaxReplayBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "Replays are at most 8 MB.")
		return
	}
	sum := sha256.Sum256(body)
	sha := hex.EncodeToString(sum[:])
	if want := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Content-SHA256"))); want != "" && want != sha {
		writeError(w, http.StatusBadRequest, "The replay was damaged on the way. Try again.")
		return
	}
	info, err := tournament.DecodeReplayHeader(body)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "That isn't an AimMod replay this Hub can read.")
		return
	}
	key := "tournaments/" + safeKey(tid) + "/" + safeKey(mid) + "/g" + strconv.Itoa(game) + "-" + strconv.FormatInt(a.UserID, 36) + "-" + sha[:16] + ".amreplay"
	if err := media.Put(r.Context(), key, "application/octet-stream", bytes.NewReader(body), int64(len(body))); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't store the replay.")
		return
	}
	var upload *tournament.Upload
	var replaced string
	_, _, err = s.mutate(r.Context(), tid, func(t *tournament.Tournament, now time.Time) error {
		var err error
		upload, replaced, err = t.AddUpload(a, mid, game, sha, int64(len(body)), key, info, now)
		return err
	})
	if err != nil {
		_ = media.Delete(context.WithoutCancel(r.Context()), key)
		status, msg := httpStatus(err)
		writeError(w, status, msg)
		return
	}
	if replaced != "" && replaced != key {
		_ = media.Delete(context.WithoutCancel(r.Context()), replaced)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"id": upload.ID, "status": upload.Status, "checks": upload.Checks, "sha256": sha})
}

func safeKey(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func (s *Server) downloadReplay(w http.ResponseWriter, r *http.Request, media Media) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, ReplayPath+"/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeError(w, http.StatusNotFound, "Replay not found.")
		return
	}
	a, err := s.actor(r.Context(), r.Header)
	if err != nil || !a.SignedIn() {
		writeError(w, http.StatusUnauthorized, "Sign in to AimMod Hub first.")
		return
	}
	t, _, err := s.load(r.Context(), parts[0])
	if err != nil {
		status, msg := httpStatus(err)
		writeError(w, status, msg)
		return
	}
	u := t.Uploads[parts[1]]
	if u == nil {
		writeError(w, http.StatusNotFound, "Replay not found.")
		return
	}
	m := t.Bracket.Match(u.Match)
	if !t.CanSeePrivate(a) && (m == nil || slotOfViewer(t, m, a) < 0) {
		writeError(w, http.StatusForbidden, "Only the match's players and the organisers can download its replays.")
		return
	}
	rc, _, err := media.Get(r.Context(), u.Key)
	if err != nil {
		writeError(w, http.StatusNotFound, "Replay not found.")
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+safeKey(t.Slug)+"-"+safeKey(u.Match)+"-game"+strconv.Itoa(u.Game+1)+"-"+safeKey(u.Entrant)+`.amreplay"`)
	_, _ = io.Copy(w, rc)
}
