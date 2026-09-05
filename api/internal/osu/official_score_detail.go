package osu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
)

type OfficialReplayAvailability struct {
	Exists            bool   `json:"exists"`
	DownloadAvailable bool   `json:"downloadAvailable"`
	Status            string `json:"status"`
	DownloadURL       string `json:"downloadUrl,omitempty"`
}

type OfficialScoreDetail struct {
	Status string                     `json:"status"`
	Item   *PublicScoreItem           `json:"item,omitempty"`
	Replay OfficialReplayAvailability `json:"replay"`
}

func (s *Server) GetPublicScore(ctx context.Context, id int64) (OfficialScoreDetail, error) {
	result := OfficialScoreDetail{Status: "not_configured", Replay: OfficialReplayAvailability{Status: "unavailable"}}
	if id <= 0 {
		return result, fmt.Errorf("invalid score ID")
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if s == nil || s.official == nil || !s.official.configured() {
		return result, nil
	}
	token, err := s.official.accessToken(ctx)
	if err != nil {
		result.Status = "authentication_failed"
		return result, ctx.Err()
	}
	body, err := s.official.getScorePage(ctx, "/api/v2/scores/"+strconv.FormatInt(id, 10), nil, token)
	if err != nil {
		result.Status = scoreErrorStatus(err)
		return result, ctx.Err()
	}
	var score officialScore
	if json.Unmarshal(body, &score) != nil || score.ID != id || score.UserID <= 0 || score.RulesetID == nil || scoreMode(*score.RulesetID) == "" {
		result.Status = "invalid_response"
		return result, nil
	}
	items := MergePublicScores(nil, []OfficialPublicScore{normalizePublicScore(score, scoreMode(*score.RulesetID))})
	if len(items) != 1 {
		result.Status = "invalid_response"
		return result, nil
	}
	result.Item = &items[0]
	result.Status = "available"
	result.Replay.Exists = score.HasReplay
	result.Replay.Status = "not_available"
	if score.HasReplay {
		result.Replay.Status = "permission_unchecked"
		result.Replay.DownloadURL = fmt.Sprintf("/api/osu/v1/official-scores/%d/replay", id)
	}
	return result, nil
}

const maximumOfficialReplayBytes = 64 << 20

var officialReplaySlots = make(chan struct{}, 2)

type OfficialReplayDownload struct {
	Status string        `json:"status"`
	Body   io.ReadCloser `json:"-"`
	Size   int64         `json:"-"`
}

type temporaryReplay struct {
	*os.File
	once sync.Once
}

func (f *temporaryReplay) Close() error {
	var err error
	f.once.Do(func() {
		err = f.File.Close()
		removeErr := os.Remove(f.Name())
		if err == nil {
			err = removeErr
		}
		<-officialReplaySlots
	})
	return err
}

// The application's public credential is tried against the actual download
// endpoint. has_replay is not permission. Bytes and failures are never cached:
// access may change independently of the cached public score metadata.
func (s *Server) DownloadPublicReplay(ctx context.Context, id int64) (OfficialReplayDownload, error) {
	result := OfficialReplayDownload{Status: "not_configured"}
	if id <= 0 {
		return result, fmt.Errorf("invalid score ID")
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if s == nil || s.official == nil || !s.official.configured() {
		return result, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	select {
	case officialReplaySlots <- struct{}{}:
	case <-ctx.Done():
		return result, ctx.Err()
	}
	transferred := false
	defer func() {
		if !transferred {
			<-officialReplaySlots
		}
	}()
	a := s.official
	token, err := a.accessToken(ctx)
	if err != nil {
		result.Status = "authentication_failed"
		return result, ctx.Err()
	}
	if err = a.limiter.wait(ctx); err != nil {
		return result, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, a.client.resolve(fmt.Sprintf("/api/v2/scores/%d/download", id), nil), nil)
	if err != nil {
		return result, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("User-Agent", a.userAgent)
	request.Header.Set("x-api-version", "20220705")
	client := *a.http
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(request)
	if err != nil {
		result.Status = "unavailable"
		return result, ctx.Err()
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		result.Status = scoreErrorStatus(&upstreamHTTPError{StatusCode: response.StatusCode})
		if response.StatusCode == 403 {
			result.Status = "permission_denied"
		}
		if response.StatusCode >= 300 && response.StatusCode < 400 {
			result.Status = "redirect_rejected"
		}
		return result, nil
	}
	if response.ContentLength > maximumOfficialReplayBytes {
		result.Status = "too_large"
		return result, nil
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0]))
	if contentType != "application/octet-stream" && contentType != "application/x-osu-replay" {
		result.Status = "invalid_response"
		return result, nil
	}
	file, err := os.CreateTemp("", "aimmod-official-replay-*.osr")
	if err != nil {
		return result, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = file.Close()
			_ = os.Remove(file.Name())
		}
	}()
	size, err := io.Copy(file, io.LimitReader(response.Body, maximumOfficialReplayBytes+1))
	if err != nil {
		result.Status = "unavailable"
		return result, ctx.Err()
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if size > maximumOfficialReplayBytes {
		result.Status = "too_large"
		return result, nil
	}
	if size < 8 {
		result.Status = "invalid_response"
		return result, nil
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return result, err
	}
	var mode [1]byte
	if _, err = io.ReadFull(file, mode[:]); err != nil || mode[0] > 3 {
		result.Status = "invalid_response"
		return result, nil
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return result, err
	}
	keep = true
	transferred = true
	return OfficialReplayDownload{Status: "available", Body: &temporaryReplay{File: file}, Size: size}, nil
}

func (s *Server) GetPublicScoreProfile(ctx context.Context, id int64, mode string) (store.OsuPublicProfile, error) {
	if s == nil || s.official == nil {
		return store.OsuPublicProfile{}, fmt.Errorf("official API unavailable")
	}
	ruleset := map[string]osuv1.Ruleset{"osu": osuv1.Ruleset_RULESET_OSU, "taiko": osuv1.Ruleset_RULESET_TAIKO, "fruits": osuv1.Ruleset_RULESET_CATCH, "mania": osuv1.Ruleset_RULESET_MANIA}[mode]
	user, err := s.official.userProfile(ctx, &osuv1.GetOfficialUserProfileRequest{Identifier: strconv.FormatInt(id, 10), LookupKey: osuv1.OfficialUserLookupKey_OFFICIAL_USER_LOOKUP_KEY_ID, Ruleset: ruleset})
	if err != nil {
		return store.OsuPublicProfile{}, err
	}
	profile := store.OsuPublicProfile{OsuUserID: int64(user.UserId), OsuUsername: user.Username, CountryCode: user.CountryCode, AvatarURL: user.AvatarUrl, RecentReplays: []store.OsuPublicReplay{}}
	if statistics := user.Statistics; statistics != nil {
		profile.PerformancePoints = &statistics.Pp
		if statistics.GlobalRank > 0 {
			rank := int64(statistics.GlobalRank)
			profile.GlobalRank = &rank
		}
		profile.PlayCount = int64(statistics.PlayCount)
		profile.PlayTimeSeconds = int64(statistics.PlayTimeSeconds)
	}
	return profile, nil
}
