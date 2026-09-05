package httpserver

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	osuv1 "github.com/veryCrunchy/aimmod-hub/gen/go/aimmod/osu/v1"
	"golang.org/x/sync/singleflight"
)

const (
	playbackArchiveLimit      = 64 << 20
	playbackUnpackedLimit     = 256 << 20
	playbackAudioLimit        = 32 << 20
	playbackAudioCacheLimit   = 96 << 20
	playbackAudioCacheEntries = 32
	playbackArchiveEntries    = 1024
	playbackMapScanLimit      = 16 << 20
	playbackAudioTimeout      = 45 * time.Second
)

// The existing official adapter owns OAuth and availability normalization.
// Neither its credentials nor user-supplied URLs are sent to the archive mirror.
type osuPlaybackMetadataProvider interface {
	GetBeatmapItem(context.Context, *connect.Request[osuv1.GetBeatmapItemRequest]) (*connect.Response[osuv1.GetBeatmapItemResponse], error)
}

type playbackAudioEntry struct {
	data        []byte
	contentType string
	etag        string
	expires     time.Time
	accessed    time.Time
}

type osuPlaybackAudioHandler struct {
	metadata   osuPlaybackMetadataProvider
	client     *http.Client
	capacity   chan struct{}
	requests   singleflight.Group
	mu         sync.Mutex
	cache      map[string]playbackAudioEntry
	cacheBytes int
}

func newOsuPlaybackAudioHandler(metadata osuPlaybackMetadataProvider) *osuPlaybackAudioHandler {
	return &osuPlaybackAudioHandler{
		metadata: metadata,
		client:   &http.Client{Timeout: playbackAudioTimeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }},
		capacity: make(chan struct{}, 2), cache: make(map[string]playbackAudioEntry),
	}
}

func (h *osuPlaybackAudioHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, osuPlaybackMapPrefix), "/")
	query, queryErr := url.ParseQuery(r.URL.RawQuery)
	if !strings.HasPrefix(r.URL.Path, osuPlaybackMapPrefix) || len(parts) != 2 || parts[1] != "audio" {
		http.NotFound(w, r)
		return
	}
	checksum := strings.ToLower(query.Get("checksum"))
	decoded, checksumErr := hex.DecodeString(checksum)
	setID := query.Get("beatmapsetId")
	if !playbackPositiveID(parts[0]) || !playbackPositiveID(setID) || queryErr != nil || len(query) != 2 || len(query["checksum"]) != 1 || len(query["beatmapsetId"]) != 1 || checksumErr != nil || len(decoded) != md5.Size {
		http.Error(w, "a positive beatmap ID, beatmapsetId and 32-hex checksum are required", http.StatusBadRequest)
		return
	}
	key := setID + ":" + parts[0] + ":" + checksum
	result := h.requests.DoChan(key, func() (any, error) {
		select {
		case h.capacity <- struct{}{}:
			defer func() { <-h.capacity }()
		default:
			return nil, audioFailure(http.StatusServiceUnavailable, "Song downloads are busy. Try again shortly.")
		}
		ctx, cancel := context.WithTimeout(context.Background(), playbackAudioTimeout)
		defer cancel()
		// Recheck official restrictions before cache hits too; archive caching must
		// never override a provider's later download-disabled response.
		if err := h.validateMetadata(ctx, parts[0], setID, checksum); err != nil {
			return nil, err
		}
		if cached, ok := h.cached(key); ok {
			return cached, nil
		}
		entry, err := h.download(ctx, setID, checksum)
		if err != nil {
			return nil, err
		}
		h.remember(key, entry)
		return entry, nil
	})
	select {
	case <-r.Context().Done():
		return
	case result := <-result:
		if result.Err != nil {
			status, message := http.StatusBadGateway, "The full song could not be loaded. Try again shortly."
			if err, ok := result.Err.(*playbackFetchError); ok {
				status, message = err.status, err.message
			}
			http.Error(w, message, status)
			return
		}
		entry := result.Val.(playbackAudioEntry)
		w.Header().Set("Content-Type", entry.contentType)
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Header().Set("ETag", entry.etag)
		http.ServeContent(w, r, "song", time.Time{}, bytes.NewReader(entry.data))
	}
}

func playbackPositiveID(value string) bool {
	id, err := strconv.ParseUint(value, 10, 31)
	return err == nil && id > 0 && strconv.FormatUint(id, 10) == value
}

func audioFailure(status int, message string) error { return &playbackFetchError{status, message} }

func (h *osuPlaybackAudioHandler) validateMetadata(ctx context.Context, mapID, setID, checksum string) error {
	response, err := h.metadata.GetBeatmapItem(ctx, connect.NewRequest(&osuv1.GetBeatmapItemRequest{
		Provider: osuv1.Provider_PROVIDER_OSU_OFFICIAL, SourceId: setID,
	}))
	if err != nil || response == nil || response.Msg == nil || response.Msg.GetItem() == nil {
		return audioFailure(http.StatusServiceUnavailable, "Official beatmap availability could not be verified. Try again shortly.")
	}
	item := response.Msg.GetItem()
	if item.GetProvider() != osuv1.Provider_PROVIDER_OSU_OFFICIAL || item.GetSourceId() != setID {
		return audioFailure(http.StatusBadGateway, "Official beatmap details did not match this set.")
	}
	if !item.GetDownloadHandoff().GetAvailable() {
		return audioFailure(http.StatusForbidden, "The provider has disabled downloads for this beatmapset.")
	}
	for _, difficulty := range item.GetDifficulties() {
		if difficulty.GetBeatmapId() != mapID || difficulty.GetBeatmapsetId() != setID {
			continue
		}
		if !difficulty.GetDownloadHandoff().GetAvailable() {
			return audioFailure(http.StatusForbidden, "The provider has disabled downloads for this difficulty.")
		}
		if !strings.EqualFold(difficulty.GetChecksum(), checksum) {
			return audioFailure(http.StatusConflict, "This replay uses a different beatmap revision. The matching full song is unavailable.")
		}
		return nil
	}
	return audioFailure(http.StatusNotFound, "This difficulty does not belong to the requested beatmapset.")
}

func (h *osuPlaybackAudioHandler) download(ctx context.Context, setID, checksum string) (playbackAudioEntry, error) {
	// Public, documented archive API: https://catboy.best/api/openapi.json /d/{id}.
	// No alternate hosts, redirect following, or retry through a denial.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://catboy.best/d/"+setID, nil)
	if err != nil {
		return playbackAudioEntry{}, err
	}
	req.Header.Set("User-Agent", "AimMod-Hub/1.0 (https://aimmod.app; replay audio)")
	response, err := h.client.Do(req)
	if err != nil {
		return playbackAudioEntry{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		status := http.StatusBadGateway
		if response.StatusCode == 401 || response.StatusCode == 403 || response.StatusCode == 451 {
			status = http.StatusForbidden
		}
		if response.StatusCode == 404 {
			status = http.StatusNotFound
		}
		if response.StatusCode == 429 {
			status = http.StatusServiceUnavailable
		}
		return playbackAudioEntry{}, audioFailure(status, "The archive provider has not made this song available for download.")
	}
	if response.ContentLength > playbackArchiveLimit {
		return playbackAudioEntry{}, audioFailure(http.StatusRequestEntityTooLarge, "The beatmap archive exceeds the song download size limit.")
	}
	archive, err := io.ReadAll(io.LimitReader(response.Body, playbackArchiveLimit+1))
	if err != nil {
		return playbackAudioEntry{}, err
	}
	if len(archive) > playbackArchiveLimit {
		return playbackAudioEntry{}, audioFailure(http.StatusRequestEntityTooLarge, "The beatmap archive exceeds the song download size limit.")
	}
	return extractPlaybackAudio(ctx, archive, checksum)
}

func extractPlaybackAudio(ctx context.Context, archive []byte, checksum string) (playbackAudioEntry, error) {
	z, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return playbackAudioEntry{}, fmt.Errorf("invalid beatmap archive")
	}
	if len(z.File) > playbackArchiveEntries {
		return playbackAudioEntry{}, fmt.Errorf("too many archive entries")
	}
	files := make(map[string]*zip.File, len(z.File))
	var total uint64
	for _, file := range z.File {
		name, ok := playbackArchivePath(file.Name)
		if !ok || (!file.FileInfo().IsDir() && !file.Mode().IsRegular()) {
			return playbackAudioEntry{}, fmt.Errorf("unsafe archive path or entry type")
		}
		if file.UncompressedSize64 > playbackUnpackedLimit || total > playbackUnpackedLimit-file.UncompressedSize64 {
			return playbackAudioEntry{}, fmt.Errorf("archive expansion exceeds limit")
		}
		total += file.UncompressedSize64
		if file.FileInfo().IsDir() {
			continue
		}
		key := strings.ToLower(name)
		if _, exists := files[key]; exists {
			return playbackAudioEntry{}, fmt.Errorf("ambiguous archive paths")
		}
		files[key] = file
	}
	var selected *zip.File
	scanned := 0
	for _, file := range z.File {
		if !strings.EqualFold(path.Ext(file.Name), ".osu") {
			continue
		}
		if err := ctx.Err(); err != nil {
			return playbackAudioEntry{}, err
		}
		if file.UncompressedSize64 > osuPlaybackMapLimit || file.UncompressedSize64 > uint64(playbackMapScanLimit-scanned) {
			return playbackAudioEntry{}, fmt.Errorf("beatmap scan exceeds limit")
		}
		data, err := readPlaybackZipFile(ctx, file, osuPlaybackMapLimit)
		if err != nil {
			return playbackAudioEntry{}, err
		}
		scanned += len(data)
		if scanned > playbackMapScanLimit {
			return playbackAudioEntry{}, fmt.Errorf("beatmap scan exceeds limit")
		}
		if fmt.Sprintf("%x", md5.Sum(data)) != checksum {
			continue
		}
		filename, err := playbackAudioFilename(data)
		if err != nil {
			return playbackAudioEntry{}, err
		}
		mapPath, _ := playbackArchivePath(file.Name)
		audioPath := path.Join(path.Dir(mapPath), filename)
		candidate := files[strings.ToLower(audioPath)]
		if candidate == nil {
			return playbackAudioEntry{}, audioFailure(http.StatusNotFound, "The matching beatmap archive does not contain its song file.")
		}
		if selected != nil && selected != candidate {
			return playbackAudioEntry{}, fmt.Errorf("ambiguous matching beatmap audio")
		}
		selected = candidate
	}
	if selected == nil {
		return playbackAudioEntry{}, audioFailure(http.StatusConflict, "The archive does not contain this exact beatmap revision.")
	}
	data, err := readPlaybackZipFile(ctx, selected, playbackAudioLimit)
	if err != nil {
		return playbackAudioEntry{}, err
	}
	contentType := playbackAudioType(selected.Name, data)
	if contentType == "" {
		return playbackAudioEntry{}, audioFailure(http.StatusUnsupportedMediaType, "The song uses an unsupported audio format.")
	}
	now := time.Now()
	return playbackAudioEntry{data: data, contentType: contentType, etag: fmt.Sprintf("\"%x\"", sha256.Sum256(data)), expires: now.Add(30 * time.Minute), accessed: now}, nil
}

// Nothing is extracted to disk. Still reject traversal, drive/UNC paths, links
// and duplicate case-insensitive names before resolving the map's audio member.
func playbackArchivePath(name string) (string, bool) {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimSuffix(name, "/")
	if name == "" || strings.HasPrefix(name, "/") || strings.ContainsAny(name, ":\x00") {
		return "", false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return "", false
		}
	}
	return name, true
}

func playbackAudioFilename(data []byte) (string, error) {
	section, filename := "", ""
	scanner := bufio.NewScanner(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})))
	scanner.Buffer(make([]byte, 4096), osuPlaybackMapLimit)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line
			continue
		}
		if section != "[General]" {
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(key) != "AudioFilename" {
			continue
		}
		if filename != "" {
			return "", fmt.Errorf("duplicate audio filename")
		}
		var valid bool
		filename, valid = playbackArchivePath(strings.TrimSpace(value))
		if !valid {
			return "", fmt.Errorf("unsafe audio filename")
		}
	}
	if scanner.Err() != nil || filename == "" {
		return "", fmt.Errorf("missing audio filename")
	}
	return filename, nil
}

type playbackContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r playbackContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func readPlaybackZipFile(ctx context.Context, file *zip.File, limit int) ([]byte, error) {
	if file.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("archive member exceeds limit")
	}
	r, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("archive member could not be opened")
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(playbackContextReader{ctx, r}, int64(limit)+1))
	if err != nil || len(data) > limit {
		return nil, fmt.Errorf("archive member is invalid or too large")
	}
	return data, nil
}

func playbackAudioType(name string, data []byte) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".mp3":
		if bytes.HasPrefix(data, []byte("ID3")) || (len(data) >= 2 && data[0] == 0xff && data[1]&0xe0 == 0xe0) {
			return "audio/mpeg"
		}
	case ".ogg":
		if bytes.HasPrefix(data, []byte("OggS")) {
			return "audio/ogg"
		}
	case ".wav":
		if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
			return "audio/wav"
		}
	case ".flac":
		if bytes.HasPrefix(data, []byte("fLaC")) {
			return "audio/flac"
		}
	}
	return ""
}

func (h *osuPlaybackAudioHandler) cached(key string) (playbackAudioEntry, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	entry, ok := h.cache[key]
	if ok && time.Now().After(entry.expires) {
		delete(h.cache, key)
		h.cacheBytes -= len(entry.data)
		return playbackAudioEntry{}, false
	}
	if ok {
		entry.accessed = time.Now()
		h.cache[key] = entry
	}
	return entry, ok
}

func (h *osuPlaybackAudioHandler) remember(key string, entry playbackAudioEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if previous, exists := h.cache[key]; exists {
		h.cacheBytes -= len(previous.data)
		delete(h.cache, key)
	}
	for len(h.cache) >= playbackAudioCacheEntries || h.cacheBytes+len(entry.data) > playbackAudioCacheLimit {
		oldestKey := ""
		var oldest time.Time
		for key, candidate := range h.cache {
			if oldestKey == "" || candidate.accessed.Before(oldest) {
				oldestKey, oldest = key, candidate.accessed
			}
		}
		if oldestKey == "" {
			break
		}
		h.cacheBytes -= len(h.cache[oldestKey].data)
		delete(h.cache, oldestKey)
	}
	h.cache[key] = entry
	h.cacheBytes += len(entry.data)
}
