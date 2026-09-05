package httpserver

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	osuservice "github.com/veryCrunchy/aimmod-hub/api/internal/osu"
)

// The production Hub owns its public OAuth application credential. This test
// supplies no credentials/cookies, uploads nothing, and retains no replay file.
// Replay downloads require public scope, unlike beatmap archive downloads:
// https://osu.ppy.sh/docs/index.html#get-apiv2scoresscoredownload
func TestOfficialReplayLivePublicCredential(t *testing.T) {
	if os.Getenv("AIMMOD_OFFICIAL_REPLAY_LIVE") != "1" {
		t.Skip("set AIMMOD_OFFICIAL_REPLAY_LIVE=1 for the real production replay smoke test")
	}
	base := os.Getenv("AIMMOD_OFFICIAL_REPLAY_HUB_URL")
	if base == "" {
		base = "https://aimmod.app"
	}
	if !strings.HasPrefix(base, "https://") {
		t.Fatal("an HTTPS Hub origin is required")
	}
	scoreID := os.Getenv("AIMMOD_OFFICIAL_REPLAY_SCORE_ID")
	if scoreID == "" {
		t.Skip("set AIMMOD_OFFICIAL_REPLAY_SCORE_ID to an explicitly selected public test score")
	}
	id, err := strconv.ParseInt(scoreID, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != scoreID {
		t.Fatal("invalid live score ID")
	}
	client := &http.Client{Timeout: 50 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	endpoint := strings.TrimRight(base, "/") + "/api/osu/v1/official-scores/" + scoreID
	metadataResponse, err := client.Get(endpoint)
	if err != nil {
		t.Fatal("live score metadata request failed")
	}
	defer metadataResponse.Body.Close()
	if metadataResponse.StatusCode != http.StatusOK {
		t.Fatalf("live metadata HTTP %d", metadataResponse.StatusCode)
	}
	var detail osuservice.OfficialScoreDetail
	if err := json.NewDecoder(io.LimitReader(metadataResponse.Body, 1<<20)).Decode(&detail); err != nil {
		t.Fatal("invalid live metadata JSON")
	}
	if detail.Status != "available" || detail.Item == nil || !detail.Replay.Exists {
		t.Fatalf("score not available for replay testing: score=%s replay=%s", detail.Status, detail.Replay.Status)
	}
	checksum := strings.ToLower(detail.Item.BeatmapChecksum)
	decoded, err := hex.DecodeString(checksum)
	if err != nil || len(decoded) != 16 {
		t.Fatal("live score lacks exact beatmap checksum")
	}
	start := time.Now()
	response, err := client.Get(endpoint + "/replay")
	if err != nil {
		t.Fatal("live replay request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("live official replay HTTP %d (not a synthetic response)", response.StatusCode)
	}
	if response.Header.Get("Content-Type") != "application/octet-stream" {
		t.Fatal("live replay was not returned as binary")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (64<<20)+1))
	if err != nil || len(data) < 64 || len(data) > 64<<20 || data[0] > 3 {
		t.Fatal("live replay is empty, malformed or exceeds the size bound")
	}
	if !bytes.Contains(data[:min(len(data), 1024)], []byte(checksum)) {
		t.Fatal("live replay header does not contain the score's exact beatmap checksum")
	}
	t.Logf("Public-credential replay PASS bytes=%d mode=%d elapsed=%s", len(data), data[0], time.Since(start).Round(time.Millisecond))
}
