package httpserver

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const osuSkinPrefix = "/api/osu/v1/playback/skins/"

// Fixed, checksum-pinned public releases; this endpoint never accepts a URL.
// Source and creator credits are also exposed in the replay skin picker.
var osuSkinSources = map[string]struct{ URL, SHA256 string }{
	"whitecat21": {"https://media.githubusercontent.com/media/azekyoo/skins/a8cdd2a2ed33dea45d8756d62f9d92426b643979/-%20%20%20%20%20%20%20%20%20%E3%80%8ACK%E3%80%8B%20WhiteCat%202.1%20~%20old.osk", "337bc0db625d885610282558f92d4c8cb6a8c63012101918bdb723e90ee00d7b"},
	"flyingtuna": {"https://media.githubusercontent.com/media/azekyoo/skins/a8cdd2a2ed33dea45d8756d62f9d92426b643979/-%20%20%20%20%20%20%20%20%23%20re%3BowoTuna%20v1.1%20%E3%80%8ESelyu%E3%80%8F%20%23%20%20%20%20%20%20%20%20-.osk", "9e0c89eaf172baf7dfd0813212259f6594d5e1e31df010fe6c00b8e5d98d0609"},

	"yugen":    {"https://tetsui.s-ul.eu/t3yyAk3g2QugMs9O", "27032e2fb6d6a7b617558f0bf632bcbecf6e1e0ec988da7f16706e75062798cb"},
	"whitecat": {"https://raw.githubusercontent.com/praeludiumOrbis/whitecat-skins/b926fa5c5ac818596dea456abf342b9fc2aa43dc/-%20%20%20%20%20%20%20%20%23%20WhiteCat%20%281.0%29%20%E3%80%8ENM%E3%80%8F%20%23-/-%20%20%20%20%20%20%20%20%23%20WhiteCat%20%281.0%29%20%E3%80%8ENM%E3%80%8F%20%23-.osk", "6b89cce6249beaf56a8799d32b8d268f54dbfeb0782919ee62e53b9be87dd4d3"},
	"rafis":    {"https://raw.githubusercontent.com/praeludiumOrbis/whitecat-skins/b926fa5c5ac818596dea456abf342b9fc2aa43dc/Rafis%202018-03-26%20HDDT/Rafis%202018-03-26%20HDDT.osk", "785b299029c06cf828deaf65ad0ec88d154fbacd935ac47577bb30dd999512d9"},
}

type osuSkinHandler struct {
	client   *http.Client
	requests singleflight.Group
	capacity chan struct{}
	mu       sync.Mutex
	cache    map[string][]byte
}

func newOsuSkinHandler() *osuSkinHandler {
	return &osuSkinHandler{client: &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, capacity: make(chan struct{}, 2), cache: make(map[string][]byte)}
}

func (h *osuSkinHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, osuSkinPrefix)
	source, ok := osuSkinSources[id]
	if !strings.HasPrefix(r.URL.Path, osuSkinPrefix) || !ok || r.URL.RawQuery != "" {
		http.NotFound(w, r)
		return
	}
	result := h.requests.DoChan(id, func() (any, error) {
		h.mu.Lock()
		cached := h.cache[id]
		h.mu.Unlock()
		if cached != nil {
			return cached, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		select {
		case h.capacity <- struct{}{}:
			defer func() { <-h.capacity }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := h.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("skin source unavailable")
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, (64<<20)+1))
		if err != nil || len(data) > 64<<20 {
			return nil, fmt.Errorf("skin archive exceeds limit")
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != source.SHA256 {
			return nil, fmt.Errorf("skin release checksum changed")
		}
		packed, err := prepareOsuSkin(data)
		if err != nil {
			return nil, err
		}
		h.mu.Lock()
		h.cache[id] = packed
		h.mu.Unlock()
		return packed, nil
	})
	select {
	case <-r.Context().Done():
		return
	case result := <-result:
		if result.Err != nil {
			http.Error(w, "This skin is temporarily unavailable.", http.StatusBadGateway)
			return
		}
		data := result.Val.([]byte)
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("ETag", fmt.Sprintf(`"%x"`, sha256.Sum256(data)))
		http.ServeContent(w, r, id+".osk", time.Time{}, bytes.NewReader(data))
	}
}

var osuGameplayImage = regexp.MustCompile(`^(hitcircle|hitcircleoverlay|approachcircle|slider[b0-9a-z-]*|reversearrow|followpoint(-[0-9]+)?|cursor[a-z]*|hit(0|50|100|300)[a-z0-9-]*|spinner-[a-z0-9-]+|inputoverlay-[a-z0-9-]+|selection-mod-[a-z0-9-]+)(@2x)?\.png$`)
var osuGameplaySound = regexp.MustCompile(`^((normal|soft|drum)-(hit(normal|whistle|finish|clap)|slider(slide|tick|whistle))|combobreak|spinnerspin|spinnerbonus)\.(wav|ogg|mp3)$`)
var osuFontSetting = regexp.MustCompile(`(?im)^\s*(HitCirclePrefix|ScorePrefix|ComboPrefix)\s*:\s*([^\r\n]+)`)

// Preserve original gameplay bytes and configured font paths, without menu
// artwork, videos, replays or unrelated files from the source skin archive.
func prepareOsuSkin(data []byte) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(reader.File) > 4096 {
		return nil, fmt.Errorf("invalid skin archive")
	}
	files := make(map[string]*zip.File)
	for _, file := range reader.File {
		name := strings.ToLower(strings.ReplaceAll(file.Name, `\`, "/"))
		if path.Clean(name) != name || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") || strings.Contains(name, ":") {
			continue
		}
		if _, exists := files[name]; exists {
			return nil, fmt.Errorf("duplicate skin asset")
		}
		files[name] = file
	}
	iniFile := files["skin.ini"]
	if iniFile == nil || iniFile.UncompressedSize64 > 64<<10 {
		return nil, fmt.Errorf("missing skin settings")
	}
	read := func(file *zip.File, max int64) ([]byte, error) {
		if file.UncompressedSize64 > uint64(max) {
			return nil, fmt.Errorf("skin asset exceeds limit")
		}
		stream, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer stream.Close()
		data, err := io.ReadAll(io.LimitReader(stream, max+1))
		if len(data) > int(max) {
			return nil, fmt.Errorf("skin asset exceeds limit")
		}
		return data, err
	}
	ini, err := read(iniFile, 64<<10)
	if err != nil {
		return nil, err
	}
	prefixes := []string{"default", "score", "combo", "scoreentry"}
	for _, match := range osuFontSetting.FindAllSubmatch(ini, -1) {
		prefixes = append(prefixes, strings.ToLower(strings.TrimSpace(strings.SplitN(string(match[2]), "//", 2)[0])))
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	total := 0
	// Archive order is deterministic, making HTTP ETags stable across restarts.
	for _, file := range reader.File {
		name := strings.ToLower(strings.ReplaceAll(file.Name, `\`, "/"))
		if files[name] != file {
			continue
		}
		keep := name == "skin.ini" || osuGameplayImage.MatchString(name) || osuGameplaySound.MatchString(name)
		for _, prefix := range prefixes {
			if strings.HasPrefix(name, prefix+"-") && strings.HasSuffix(name, ".png") {
				keep = true
			}
		}
		if !keep {
			continue
		}
		asset, err := read(file, 8<<20)
		if err != nil {
			return nil, err
		}
		total += len(asset)
		if total > 32<<20 {
			return nil, fmt.Errorf("skin gameplay exceeds limit")
		}
		entry, err := writer.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err = entry.Write(asset); err != nil {
			return nil, err
		}
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
