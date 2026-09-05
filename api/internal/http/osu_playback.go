package httpserver

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	osuPlaybackMapLimit     = 4 << 20
	osuPlaybackCacheLimit   = 64 << 20
	osuPlaybackCacheEntries = 128
	osuPlaybackMapPrefix    = "/api/osu/v1/playback/beatmaps/"
)

type osuPlaybackMap struct {
	data     []byte
	etag     string
	expires  time.Time
	accessed time.Time
}

type osuPlaybackHandler struct {
	client     *http.Client
	mu         sync.Mutex
	cache      map[string]osuPlaybackMap
	cacheBytes int
	requests   singleflight.Group
	capacity   chan struct{}
}

func newOsuPlaybackHandler() *osuPlaybackHandler {
	return &osuPlaybackHandler{
		client: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }},
		cache:  make(map[string]osuPlaybackMap), capacity: make(chan struct{}, 4),
	}
}

func (h *osuPlaybackHandler) register(mux *http.ServeMux, origin string) {
	mux.Handle(osuPlaybackMapPrefix, withCORS(origin, h))
}

func (h *osuPlaybackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, osuPlaybackMapPrefix), "/")
	if !strings.HasPrefix(r.URL.Path, osuPlaybackMapPrefix) || len(parts) != 2 || parts[1] != "file" {
		http.NotFound(w, r)
		return
	}
	id, err := strconv.ParseUint(parts[0], 10, 31)
	if err != nil || id == 0 || strconv.FormatUint(id, 10) != parts[0] {
		http.Error(w, "a positive beatmap difficulty ID is required", http.StatusBadRequest)
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	checksum := ""
	if values, exists := query["checksum"]; exists && len(values) == 1 {
		checksum = strings.ToLower(values[0])
	}
	decoded, checksumErr := hex.DecodeString(checksum)
	if err != nil || (len(query) != 0 && (len(query) != 1 || checksumErr != nil || len(decoded) != md5.Size)) {
		http.Error(w, "only an optional 32-hex checksum is accepted", http.StatusBadRequest)
		return
	}
	key := parts[0]
	if checksum != "" {
		key += ":" + checksum
	}
	if cached, ok := h.cached(key); ok {
		writePlaybackMap(w, r, cached)
		return
	}
	// Coalesce the same difficulty while bounding simultaneous upstream requests.
	result := h.requests.DoChan(key, func() (any, error) {
		if cached, ok := h.cached(key); ok {
			return cached, nil
		}
		select {
		case h.capacity <- struct{}{}:
			defer func() { <-h.capacity }()
		default:
			return nil, &playbackFetchError{http.StatusServiceUnavailable, "Beatmap downloads are busy. Try again shortly."}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://osu.ppy.sh/osu/"+parts[0], nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("User-Agent", "AimMod-Hub/1.0 (beatmap playback and performance)")
		response, err := h.client.Do(request)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		if response.StatusCode == http.StatusNotFound {
			return nil, &playbackFetchError{http.StatusNotFound, "This beatmap difficulty is not available."}
		}
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("official beatmap returned HTTP %d", response.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, osuPlaybackMapLimit+1))
		if err != nil {
			return nil, err
		}
		if len(data) > osuPlaybackMapLimit {
			return nil, &playbackFetchError{http.StatusBadGateway, "The beatmap exceeds the playback size limit."}
		}
		if !bytes.HasPrefix(bytes.TrimSpace(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})), []byte("osu file format v")) {
			return nil, fmt.Errorf("official endpoint did not return an osu beatmap")
		}
		// osu identifies exact difficulty revisions by MD5, not by numeric ID.
		if checksum != "" && fmt.Sprintf("%x", md5.Sum(data)) != checksum {
			return nil, &playbackFetchError{http.StatusConflict, "The beatmap file does not match the requested revision. Refresh the beatmap details and try again."}
		}
		now := time.Now()
		entry := osuPlaybackMap{data: data, etag: fmt.Sprintf("\"%x\"", sha256.Sum256(data)), expires: now.Add(6 * time.Hour), accessed: now}
		h.remember(key, entry)
		return entry, nil
	})
	select {
	case <-r.Context().Done():
		return
	case response := <-result:
		if response.Err != nil {
			status, message := http.StatusBadGateway, "The beatmap could not be loaded from osu!. Try again shortly."
			if failure, ok := response.Err.(*playbackFetchError); ok {
				status, message = failure.status, failure.message
			}
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, message, status)
			return
		}
		writePlaybackMap(w, r, response.Val.(osuPlaybackMap))
	}
}

func writePlaybackMap(w http.ResponseWriter, r *http.Request, entry osuPlaybackMap) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("ETag", entry.etag)
	if r.Header.Get("If-None-Match") == entry.etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(entry.data)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(entry.data)
	}
}

func (h *osuPlaybackHandler) cached(key string) (osuPlaybackMap, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	entry, exists := h.cache[key]
	if !exists {
		return entry, false
	}
	if time.Now().After(entry.expires) {
		delete(h.cache, key)
		h.cacheBytes -= len(entry.data)
		return entry, false
	}
	entry.accessed = time.Now()
	h.cache[key] = entry
	return entry, true
}

func (h *osuPlaybackHandler) remember(key string, entry osuPlaybackMap) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if previous, exists := h.cache[key]; exists {
		h.cacheBytes -= len(previous.data)
		delete(h.cache, key)
	}
	for len(h.cache) >= osuPlaybackCacheEntries || h.cacheBytes+len(entry.data) > osuPlaybackCacheLimit {
		oldestKey := ""
		var oldest time.Time
		for candidate, value := range h.cache {
			if oldestKey == "" || value.accessed.Before(oldest) {
				oldestKey, oldest = candidate, value.accessed
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

type playbackFetchError struct {
	status  int
	message string
}

func (e *playbackFetchError) Error() string { return e.message }
