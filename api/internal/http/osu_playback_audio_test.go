package httpserver

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
	osuv1connect "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1/osuv1connect"
)

type playbackMetadataFunc func(context.Context, *connect.Request[osuv1.GetBeatmapItemRequest]) (*connect.Response[osuv1.GetBeatmapItemResponse], error)

func (f playbackMetadataFunc) GetBeatmapItem(ctx context.Context, req *connect.Request[osuv1.GetBeatmapItemRequest]) (*connect.Response[osuv1.GetBeatmapItemResponse], error) {
	return f(ctx, req)
}

func audioTestItem(checksum string) *osuv1.BeatmapItem {
	return &osuv1.BeatmapItem{
		Provider: osuv1.Provider_PROVIDER_OSU_OFFICIAL, SourceId: "1",
		DownloadHandoff: &osuv1.DownloadHandoff{Available: true},
		Difficulties:    []*osuv1.BeatmapDifficulty{{BeatmapId: "2", BeatmapsetId: "1", Checksum: checksum, DownloadHandoff: &osuv1.DownloadHandoff{Available: true}}},
	}
}

func audioTestProvider(item *osuv1.BeatmapItem) playbackMetadataFunc {
	return func(_ context.Context, req *connect.Request[osuv1.GetBeatmapItemRequest]) (*connect.Response[osuv1.GetBeatmapItemResponse], error) {
		if req.Msg.GetProvider() != osuv1.Provider_PROVIDER_OSU_OFFICIAL || req.Msg.GetSourceId() != "1" {
			return nil, fmt.Errorf("unexpected metadata request")
		}
		return connect.NewResponse(&osuv1.GetBeatmapItemResponse{Item: item}), nil
	}
}

func audioTestZip(t *testing.T, names, bodies []string) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for i, name := range names {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = io.WriteString(w, bodies[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func audioTestRequest(checksum string) string {
	return osuPlaybackMapPrefix + "2/audio?beatmapsetId=1&checksum=" + checksum
}

func TestPlaybackAudioExactArchiveSongCachingRangeAndRestrictionRecheck(t *testing.T) {
	mapData := "osu file format v14\r\n[General]\r\nAudioFilename: song.mp3\r\n"
	checksum := fmt.Sprintf("%x", md5.Sum([]byte(mapData)))
	item := audioTestItem(checksum)
	h := newOsuPlaybackHandler(audioTestProvider(item))
	song := "ID3" + strings.Repeat("full-song-bytes", 1000)
	archive := audioTestZip(t, []string{"nested/map.osu", "nested/song.mp3", "wrong.mp3"}, []string{mapData, song, "ID3wrong"})
	calls := 0
	h.audio.client.Transport = playbackRoundTripper(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://catboy.best/d/1" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("unsafe upstream request")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(archive))}, nil
	})
	request := func(method, rangeValue string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, audioTestRequest(checksum), nil)
		if rangeValue != "" {
			r.Header.Set("Range", rangeValue)
		}
		h.ServeHTTP(w, r)
		return w
	}
	w := request("GET", "")
	if w.Code != 200 || w.Body.String() != song || w.Header().Get("Content-Type") != "audio/mpeg" {
		t.Fatalf("song not exact: %d %s", w.Code, w.Body.String())
	}
	etag := w.Header().Get("ETag")
	w = request("GET", "bytes=3-8")
	if w.Code != 206 || w.Body.String() != song[3:9] || calls != 1 {
		t.Fatal("range/cache failed", w.Code, calls)
	}
	w = request("HEAD", "")
	if w.Code != 200 || w.Body.Len() != 0 || w.Header().Get("Content-Length") != fmt.Sprint(len(song)) {
		t.Fatal("HEAD failed")
	}
	r := httptest.NewRequest("GET", audioTestRequest(checksum), nil)
	r.Header.Set("If-None-Match", etag)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 304 || w.Body.Len() != 0 {
		t.Fatal("conditional failed")
	}
	item.DownloadHandoff.Available = false
	w = request("GET", "")
	if w.Code != 403 || w.Header().Get("Cache-Control") != "no-store" || calls != 1 {
		t.Fatal("cache bypassed official restriction")
	}
}

func TestPlaybackAudioMetadataFailuresNeverDownload(t *testing.T) {
	checksum := strings.Repeat("a", 32)
	for _, test := range []struct {
		name   string
		mutate func(*osuv1.BeatmapItem)
		status int
	}{
		{"set restricted", func(i *osuv1.BeatmapItem) { i.DownloadHandoff.Available = false }, 403},
		{"unknown availability", func(i *osuv1.BeatmapItem) { i.DownloadHandoff = nil }, 403},
		{"difficulty restricted", func(i *osuv1.BeatmapItem) { i.Difficulties[0].DownloadHandoff.Available = false }, 403},
		{"wrong provider", func(i *osuv1.BeatmapItem) { i.Provider = osuv1.Provider_PROVIDER_OSU_COLLECTOR }, 502},
		{"wrong set", func(i *osuv1.BeatmapItem) { i.SourceId = "3" }, 502},
		{"wrong membership", func(i *osuv1.BeatmapItem) { i.Difficulties[0].BeatmapsetId = "3" }, 404},
		{"wrong map", func(i *osuv1.BeatmapItem) { i.Difficulties[0].BeatmapId = "3" }, 404},
		{"wrong revision", func(i *osuv1.BeatmapItem) { i.Difficulties[0].Checksum = strings.Repeat("b", 32) }, 409},
	} {
		t.Run(test.name, func(t *testing.T) {
			item := audioTestItem(checksum)
			test.mutate(item)
			h := newOsuPlaybackAudioHandler(audioTestProvider(item))
			h.client.Transport = playbackRoundTripper(func(*http.Request) (*http.Response, error) { t.Error("restriction reached mirror"); return nil, io.EOF })
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", audioTestRequest(checksum), nil))
			if w.Code != test.status {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
	h := newOsuPlaybackAudioHandler(playbackMetadataFunc(func(context.Context, *connect.Request[osuv1.GetBeatmapItemRequest]) (*connect.Response[osuv1.GetBeatmapItemResponse], error) {
		return nil, fmt.Errorf("secret test credential")
	}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", audioTestRequest(checksum), nil))
	if w.Code != 503 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal("metadata failure exposed details")
	}
}

func TestPlaybackAudioRejectsQueriesWithoutMetadata(t *testing.T) {
	h := newOsuPlaybackAudioHandler(playbackMetadataFunc(func(context.Context, *connect.Request[osuv1.GetBeatmapItemRequest]) (*connect.Response[osuv1.GetBeatmapItemResponse], error) {
		t.Error("invalid request reached metadata")
		return nil, io.EOF
	}))
	valid := "beatmapsetId=1&checksum=" + strings.Repeat("a", 32)
	for _, query := range []string{"", valid + "&url=https://evil.test", valid + "&checksum=" + strings.Repeat("a", 32), strings.Replace(valid, "Id=1", "Id=0", 1), strings.Replace(valid, "Id=1", "Id=01", 1), "beatmapsetId=1&checksum=%zz", "beatmapsetId=1&checksum=abc", valid + "&beatmapsetId=1"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", osuPlaybackMapPrefix+"2/audio?"+query, nil))
		if w.Code != 400 {
			t.Error(query, w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", audioTestRequest(strings.Repeat("a", 32)), nil))
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
}

func TestPlaybackAudioArchiveRejectsUnsafeOrAmbiguousPaths(t *testing.T) {
	mapData := "osu file format v14\n[General]\nAudioFilename: song.mp3\n"
	checksum := fmt.Sprintf("%x", md5.Sum([]byte(mapData)))
	for _, name := range []string{"../evil", "/absolute", "C:\\drive", "\\\\server\\share", "a/../song.mp3", "a//b", "nul\x00", "SONG.mp3"} {
		archive := audioTestZip(t, []string{"map.osu", "song.mp3", name}, []string{mapData, "ID3song", "bad"})
		if _, err := extractPlaybackAudio(context.Background(), archive, checksum); err == nil {
			t.Errorf("accepted unsafe path %q", name)
		}
	}
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	f := &zip.FileHeader{Name: "link"}
	f.SetMode(os.ModeSymlink | 0777)
	w, _ := z.CreateHeader(f)
	_, _ = io.WriteString(w, "song.mp3")
	_ = z.Close()
	if _, err := extractPlaybackAudio(context.Background(), b.Bytes(), checksum); err == nil {
		t.Fatal("accepted symlink")
	}
}

func TestPlaybackAudioRequiresExactMapAndNamedAudio(t *testing.T) {
	for _, test := range []struct {
		name, audioName, audioData string
		extra                      string
	}{
		{"missing", "missing.mp3", "ID3song", ""},
		{"traversal", "../song.mp3", "ID3song", ""},
		{"remote", "https://evil/song.mp3", "ID3song", ""},
		{"duplicate", "song.mp3", "ID3song", "AudioFilename: second.mp3\n"},
		{"fake audio", "song.mp3", "<html>not audio</html>", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			mapData := "osu file format v14\n[General]\nAudioFilename: " + test.audioName + "\n" + test.extra
			archive := audioTestZip(t, []string{"map.osu", "song.mp3"}, []string{mapData, test.audioData})
			checksum := fmt.Sprintf("%x", md5.Sum([]byte(mapData)))
			if _, err := extractPlaybackAudio(context.Background(), archive, checksum); err == nil {
				t.Fatal("accepted invalid song")
			}
		})
	}
	archive := audioTestZip(t, []string{"map.osu", "song.mp3"}, []string{"osu file format v14\n[General]\nAudioFilename: song.mp3\n", "ID3song"})
	if _, err := extractPlaybackAudio(context.Background(), archive, strings.Repeat("a", 32)); err == nil {
		t.Fatal("accepted wrong revision")
	}
}

func TestPlaybackAudioArchiveBoundsAndCancellation(t *testing.T) {
	names, bodies := make([]string, playbackArchiveEntries+1), make([]string, playbackArchiveEntries+1)
	for i := range names {
		names[i] = fmt.Sprintf("%d.txt", i)
	}
	if _, err := extractPlaybackAudio(context.Background(), audioTestZip(t, names, bodies), ""); err == nil {
		t.Fatal("entry bound ignored")
	}
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	_, err := z.CreateRaw(&zip.FileHeader{Name: "bomb", Method: zip.Store, UncompressedSize64: playbackUnpackedLimit + 1})
	if err != nil {
		t.Fatal(err)
	}
	_ = z.Close()
	if _, err := extractPlaybackAudio(context.Background(), b.Bytes(), ""); err == nil {
		t.Fatal("expansion bound ignored")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := extractPlaybackAudio(ctx, audioTestZip(t, []string{"map.osu"}, []string{"map"}), ""); err == nil {
		t.Fatal("cancel ignored")
	}
	file := &zip.File{FileHeader: zip.FileHeader{UncompressedSize64: playbackAudioLimit + 1}}
	if _, err := readPlaybackZipFile(context.Background(), file, playbackAudioLimit); err == nil {
		t.Fatal("audio bound ignored")
	}
}

func TestPlaybackAudioUpstreamDenialNeverFallsBackOrCaches(t *testing.T) {
	for _, status := range []int{301, 401, 403, 404, 429, 451, 500} {
		h := newOsuPlaybackAudioHandler(nil)
		calls := 0
		h.client.Transport = playbackRoundTripper(func(*http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: status, Header: http.Header{"Location": []string{"https://other.test/archive"}}, Body: io.NopCloser(strings.NewReader("denied"))}, nil
		})
		if _, err := h.download(context.Background(), "1", ""); err == nil || calls != 1 || len(h.cache) != 0 {
			t.Fatal("denial bypass", status, calls)
		}
	}
	h := newOsuPlaybackAudioHandler(nil)
	h.client.Transport = playbackRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, ContentLength: playbackArchiveLimit + 1, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	if _, err := h.download(context.Background(), "1", ""); err == nil {
		t.Fatal("compressed size ignored")
	}
}

func TestPlaybackAudioCacheBoundedAndExpired(t *testing.T) {
	h := newOsuPlaybackAudioHandler(nil)
	now := time.Now()
	for i := 0; i < playbackAudioCacheEntries+1; i++ {
		h.remember(fmt.Sprint(i), playbackAudioEntry{data: []byte("song"), expires: now.Add(time.Hour), accessed: now.Add(time.Duration(i) * time.Second)})
	}
	if len(h.cache) != playbackAudioCacheEntries || h.cacheBytes != playbackAudioCacheEntries*4 {
		t.Fatal("unbounded cache")
	}
	if _, ok := h.cached("0"); ok {
		t.Fatal("oldest not evicted")
	}
	h.remember("expired", playbackAudioEntry{data: []byte("old"), expires: now.Add(-time.Second)})
	if _, ok := h.cached("expired"); ok {
		t.Fatal("expiry ignored")
	}
}

func TestPlaybackAudioConcurrencyBound(t *testing.T) {
	var calls atomic.Int32
	entered, release := make(chan struct{}, 2), make(chan struct{})
	h := newOsuPlaybackAudioHandler(playbackMetadataFunc(func(ctx context.Context, _ *connect.Request[osuv1.GetBeatmapItemRequest]) (*connect.Response[osuv1.GetBeatmapItemResponse], error) {
		calls.Add(1)
		entered <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil, io.EOF
	}))
	done := make(chan struct{}, 2)
	for _, checksum := range []string{strings.Repeat("a", 32), strings.Repeat("b", 32)} {
		go func(sum string) {
			defer func() { done <- struct{}{} }()
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", audioTestRequest(sum), nil))
		}(checksum)
	}
	<-entered
	<-entered
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", audioTestRequest(strings.Repeat("c", 32)), nil))
	close(release)
	<-done
	<-done
	if w.Code != 503 || calls.Load() != 2 {
		t.Fatal("concurrency unbounded", w.Code, calls.Load())
	}
}

// Explicit opt-in: real official metadata through a running Hub's existing
// credential-owning provider, followed by the real public mirror. No uploads.
func TestPlaybackAudioLiveUpstream(t *testing.T) {
	if os.Getenv("AIMMOD_PLAYBACK_AUDIO_LIVE") != "1" {
		t.Skip("set AIMMOD_PLAYBACK_AUDIO_LIVE=1 and AIMMOD_PLAYBACK_METADATA_URL for real upstream smoke")
	}
	base := os.Getenv("AIMMOD_PLAYBACK_METADATA_URL")
	if !strings.HasPrefix(base, "https://") {
		t.Fatal("explicit HTTPS Hub metadata URL required")
	}
	setID := os.Getenv("AIMMOD_PLAYBACK_AUDIO_SET_ID")
	if setID == "" {
		setID = "1"
	}
	client := osuv1connect.NewOsuServiceClient(&http.Client{Timeout: 15 * time.Second}, base)
	response, err := client.GetBeatmapItem(context.Background(), connect.NewRequest(&osuv1.GetBeatmapItemRequest{Provider: osuv1.Provider_PROVIDER_OSU_OFFICIAL, SourceId: setID}))
	if err != nil {
		t.Fatal("real official metadata unavailable", connect.CodeOf(err))
	}
	item := response.Msg.GetItem()
	if !item.GetDownloadHandoff().GetAvailable() || len(item.GetDifficulties()) == 0 {
		t.Fatal("real set unavailable; no archive download attempted")
	}
	difficulty := item.GetDifficulties()[0]
	h := newOsuPlaybackAudioHandler(client)
	url := osuPlaybackMapPrefix + difficulty.GetBeatmapId() + "/audio?beatmapsetId=" + setID + "&checksum=" + difficulty.GetChecksum()
	w := httptest.NewRecorder()
	start := time.Now()
	h.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
	if w.Code != 200 {
		t.Fatalf("real song route HTTP%d: %s", w.Code, w.Body.String())
	}
	if w.Body.Len() < 100000 {
		t.Fatal("real song unexpectedly short")
	}
	t.Logf("REAL upstream PASS set=%s map=%s checksum=%s audio=%s bytes=%d elapsed=%s", setID, difficulty.GetBeatmapId(), difficulty.GetChecksum(), w.Header().Get("Content-Type"), w.Body.Len(), time.Since(start).Round(time.Millisecond))
}
