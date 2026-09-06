package httpserver

import (
	"bytes"
	"container/list"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/veryCrunchy/aimmod-hub/api/internal/coaching"
	"github.com/veryCrunchy/aimmod-hub/api/internal/seo"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const socialPreviewVersion = "social-v1"

var errPreviewUnavailable = errors.New("preview unavailable")

//go:embed social_assets/aimmod-256.png
var socialBrandPNG []byte

// Noto Sans JP Regular, unmodified static OpenType font; see social_assets/OFL.txt.
//
//go:embed social_assets/NotoSansJP-Regular.otf
var socialJapaneseFont []byte

//go:embed social_assets/OFL.txt
var socialFontLicense string

type socialPreviewStore interface {
	osuMetadataStore
	GetProfileMeta(context.Context, string) (*store.ProfileMeta, error)
	GetScenarioMeta(context.Context, string) (*store.ScenarioMeta, error)
	GetRunMeta(context.Context, string) (*store.RunMeta, error)
}

// Only a local canonical route is accepted; no caller-controlled titles, images or URLs.
func socialPreviewPath(value string) (string, bool) {
	if len(value) == 0 || len(value) > 512 || !utf8.ValidString(value) || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "", false
	}
	if strings.ContainsAny(value, "\\?#%") || strings.IndexFunc(value, unicode.IsControl) >= 0 || path.Clean(value) != value {
		return "", false
	}
	return value, true
}

func resolveSocialPreview(ctx context.Context, route string, st socialPreviewStore) (pageMeta, error) {
	canonical := "https://aimmod.app" + route
	if _, ok := seo.Published.Routes[route]; ok && route != "/search" {
		return resolvePageMeta(ctx, route, canonical, nil), nil
	}
	for _, guide := range seo.Published.Guides {
		if route == "/osu/learn/"+guide.Slug {
			return resolvePageMeta(ctx, route, canonical, nil), nil
		}
	}
	if match := reLearn.FindStringSubmatch(route); match != nil {
		if _, err := coaching.GetLearnEntry(match[1]); err != nil {
			return pageMeta{}, errPreviewUnavailable
		}
		return resolvePageMeta(ctx, route, canonical, nil), nil
	}
	if match := reLearnTopic.FindStringSubmatch(route); match != nil {
		if _, err := coaching.GetLearnTopic(match[1]); err != nil {
			return pageMeta{}, errPreviewUnavailable
		}
		return resolvePageMeta(ctx, route, canonical, nil), nil
	}
	if st == nil {
		return pageMeta{}, errPreviewUnavailable
	}
	if strings.HasPrefix(route, "/osu/replays/") || strings.HasPrefix(route, "/osu/profiles/") {
		meta := resolveOsuDetailMeta(ctx, route, canonical, st)
		if meta.NoIndex {
			return pageMeta{}, errPreviewUnavailable
		}
		return meta, nil
	}
	// These store projections are the same public data used by existing KovaaK's page metadata.
	if match := reProfile.FindStringSubmatch(route); match != nil && route == "/profiles/"+match[1] {
		record, err := st.GetProfileMeta(ctx, match[1])
		if err != nil || record == nil || record.RunCount == 0 {
			return pageMeta{}, errPreviewUnavailable
		}
		name := record.DisplayName
		if name == "" {
			name = record.Handle
		}
		return pageMeta{Title: fmt.Sprintf("%s (@%s)", name, record.Handle), Description: fmt.Sprintf("%d runs across %d scenarios on AimMod Hub.", record.RunCount, record.ScenarioCount), OGType: "profile", Canonical: canonical}, nil
	}
	if match := reScenario.FindStringSubmatch(route); match != nil {
		record, err := st.GetScenarioMeta(ctx, match[1])
		if err != nil || record == nil {
			return pageMeta{}, errPreviewUnavailable
		}
		return pageMeta{Title: record.Name, Description: fmt.Sprintf("%d runs · Best score %.0f · Average accuracy %.1f%%", record.RunCount, record.BestScore, record.AvgAcc), Canonical: canonical}, nil
	}
	if match := reRun.FindStringSubmatch(route); match != nil {
		record, err := st.GetRunMeta(ctx, match[1])
		if err != nil || record == nil {
			return pageMeta{}, errPreviewUnavailable
		}
		name := record.UserDisplayName
		if name == "" {
			name = record.UserHandle
		}
		return pageMeta{Title: record.ScenarioName + " by " + name, Description: fmt.Sprintf("Score %.0f · Accuracy %.1f%%", record.Score, record.Accuracy), Canonical: canonical}, nil
	}
	return pageMeta{}, errPreviewUnavailable
}

type previewImage struct {
	key, etag string
	data      []byte
}
type socialPreviewHandler struct {
	resolve func(context.Context, string) (previewContent, error)
	mu      sync.Mutex
	cache   map[string]*list.Element
	lru     *list.List
	bytes   int
}

func newSocialPreviewHandler(st *store.Store) http.Handler {
	return newSocialPreviewRenderer(func(ctx context.Context, route string) (previewContent, error) {
		if st == nil {
			return resolveSocialContent(ctx, route, nil)
		}
		return resolveSocialContent(ctx, route, st)
	})
}

func newSocialPreviewRenderer(resolve func(context.Context, string) (previewContent, error)) *socialPreviewHandler {
	return &socialPreviewHandler{resolve: resolve, cache: map[string]*list.Element{}, lru: list.New()}
}

func (h *socialPreviewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if len(r.URL.RawQuery) > 2048 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(query["path"]) != 1 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	for key, values := range query {
		if key != "path" && (key != "v" || len(values) != 1 || values[0] != "1") {
			http.Error(w, "invalid parameter", http.StatusBadRequest)
			return
		}
	}
	route, ok := socialPreviewPath(query.Get("path"))
	if !ok {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	// This lookup must precede both cached bytes and conditional responses: visibility may change.
	input, err := h.resolve(r.Context(), route)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	keyBytes, _ := json.Marshal(input)
	hash := sha256.Sum256(append([]byte(socialPreviewVersion), keyBytes...))
	key := hex.EncodeToString(hash[:])
	// Serialize cache misses to bound concurrent full-size image allocations and duplicate rendering.
	h.mu.Lock()
	entry := h.cache[key]
	if entry == nil {
		data, renderErr := renderSocialPreview(input)
		if renderErr != nil {
			h.mu.Unlock()
			http.Error(w, "preview unavailable", http.StatusInternalServerError)
			return
		}
		digest := sha256.Sum256(data)
		entry = h.lru.PushFront(previewImage{key: key, etag: `"` + hex.EncodeToString(digest[:]) + `"`, data: data})
		h.cache[key] = entry
		h.bytes += len(data)
		for h.lru.Len() > 128 || h.bytes > 24<<20 {
			oldest := h.lru.Back()
			value := oldest.Value.(previewImage)
			delete(h.cache, value.key)
			h.bytes -= len(value.data)
			h.lru.Remove(oldest)
		}
	} else {
		h.lru.MoveToFront(entry)
	}
	value := entry.Value.(previewImage)
	h.mu.Unlock()
	w.Header().Set("Cache-Control", "private, no-cache, must-revalidate")
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("ETag", value.etag)
	for _, tag := range strings.Split(r.Header.Get("If-None-Match"), ",") {
		tag = strings.TrimSpace(tag)
		if tag == "*" || strings.TrimPrefix(tag, "W/") == value.etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(value.data)))
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(value.data)
}

type previewMetric struct{ Label, Value string }
type previewContent struct {
	Title, Description, Category string
	Metrics                      []previewMetric
}

func resolveSocialContent(ctx context.Context, route string, st socialPreviewStore) (previewContent, error) {
	meta, err := resolveSocialPreview(ctx, route, st)
	if err != nil || meta.NoIndex {
		return previewContent{}, errPreviewUnavailable
	}
	content := previewContentFor(route, meta)
	if id, ok := strings.CutPrefix(route, "/osu/replays/"); ok {
		replay, err := st.GetOsuPublicReplay(ctx, id)
		if err != nil || replay.Visibility != store.OsuVisibilityPublic {
			return previewContent{}, errPreviewUnavailable
		}
		pp := "Unavailable"
		if replay.PerformancePoints != nil {
			pp = fmt.Sprintf("%.0f PP", *replay.PerformancePoints)
		}
		mods := strings.Join(replay.Mods, "+")
		if mods == "" {
			mods = "NM"
		}
		content.Description = boundedPreviewText("Played by "+replay.OsuUsername, 1600)
		content.Metrics = []previewMetric{{"ACCURACY", fmt.Sprintf("%.2f%%", replay.Accuracy*100)}, {"PERFORMANCE", pp}, {"COMBO", fmt.Sprintf("%dx", replay.MaxCombo)}, {"MODS", boundedPreviewText(mods, 128)}}
	} else if handle, ok := strings.CutPrefix(route, "/osu/profiles/"); ok {
		profile, err := st.GetOsuPublicProfile(ctx, handle, 1)
		if err != nil || profile.SharedReplayCount <= 0 {
			index, ok := st.(interface {
				GetIndexedOsuProfile(context.Context, string, string) (store.OsuPublicProfile, error)
			})
			if !ok {
				return previewContent{}, errPreviewUnavailable
			}
			profile, err = index.GetIndexedOsuProfile(ctx, handle, "")
			if err != nil {
				return previewContent{}, errPreviewUnavailable
			}
		}
		pp, rank := "Unavailable", "Unranked"
		if profile.PerformancePoints != nil {
			pp = fmt.Sprintf("%.0f PP", *profile.PerformancePoints)
		}
		if profile.GlobalRank != nil {
			rank = fmt.Sprintf("#%d", *profile.GlobalRank)
		}
		content.Metrics = []previewMetric{{"PERFORMANCE", pp}, {"GLOBAL RANK", rank}, {"PUBLIC REPLAYS", strconv.Itoa(profile.SharedReplayCount)}}
	}
	return content, nil
}
func previewContentFor(route string, meta pageMeta) previewContent {
	category := "KOVAAK'S / COMMUNITY"
	if strings.HasPrefix(route, "/osu") || route == "/app/osu" {
		category = "OSU! / COMMUNITY"
	}
	if strings.Contains(route, "/learn") {
		category = strings.Split(category, " / ")[0] + " / KNOWLEDGE BASE"
	}
	if strings.HasPrefix(route, "/app") {
		category = "AIMMOD / DOWNLOADS"
	}
	if strings.Contains(route, "/replays/") || strings.HasPrefix(route, "/runs/") {
		category = strings.Split(category, " / ")[0] + " / REPLAY ANALYSIS"
	}
	if meta.OGType == "profile" {
		category = strings.Split(category, " / ")[0] + " / PLAYER PROFILE"
	}
	title := strings.TrimSuffix(strings.TrimSuffix(meta.Title, " · AimMod Hub"), " · AimMod Learn")
	return previewContent{Title: boundedPreviewText(title, 512), Description: boundedPreviewText(meta.Description, 1600), Category: category}
}

func boundedPreviewText(value string, limit int) string {
	runes := []rune(value)
	if len(runes) > limit {
		runes = runes[:limit]
	}
	return strings.Join(strings.FieldsFunc(string(runes), func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }), " ")
}

func previewFace(ttf []byte, size float64) (font.Face, error) {
	parsed, err := opentype.Parse(ttf)
	if err != nil {
		return nil, err
	}
	primary, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, err
	}
	fallback, err := opentype.Parse(socialJapaneseFont)
	if err != nil {
		primary.Close()
		return nil, err
	}
	japanese, err := opentype.NewFace(fallback, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		primary.Close()
		return nil, err
	}
	return &previewFontFace{primary, japanese}, nil
}

type previewFontFace struct{ primary, japanese font.Face }

func (f *previewFontFace) face(r rune) font.Face {
	if _, ok := f.primary.GlyphAdvance(r); ok {
		return f.primary
	}
	return f.japanese
}
func (f *previewFontFace) Close() error { _ = f.japanese.Close(); return f.primary.Close() }
func (f *previewFontFace) Glyph(dot fixed.Point26_6, r rune) (image.Rectangle, image.Image, image.Point, fixed.Int26_6, bool) {
	return f.face(r).Glyph(dot, r)
}
func (f *previewFontFace) GlyphBounds(r rune) (fixed.Rectangle26_6, fixed.Int26_6, bool) {
	return f.face(r).GlyphBounds(r)
}
func (f *previewFontFace) GlyphAdvance(r rune) (fixed.Int26_6, bool) {
	return f.face(r).GlyphAdvance(r)
}
func (f *previewFontFace) Kern(a, b rune) fixed.Int26_6 {
	if f.face(a) == f.face(b) {
		return f.face(a).Kern(a, b)
	}
	return 0
}
func (f *previewFontFace) Metrics() font.Metrics { return f.primary.Metrics() }

func previewLines(text string, face font.Face, width, maxLines int) []string {
	// Keep unsupported scripts/emoji readable without missing-glyph rectangles.
	var supported strings.Builder
	missing := false
	for _, r := range text {
		_, ok := face.GlyphAdvance(r)
		if !ok {
			if !missing {
				supported.WriteString("...")
			}
			missing = true
			continue
		}
		supported.WriteRune(r)
		missing = false
	}
	text = supported.String()
	var lines []string
	current := ""
	for _, word := range strings.Fields(text) {
		next := word
		if current != "" {
			next = current + " " + word
		}
		if font.MeasureString(face, next).Ceil() <= width {
			current = next
			continue
		}
		if current != "" {
			lines = append(lines, current)
			current = ""
		}
		// Long unbroken titles must also fit; never let a single token cross the right margin.
		for _, r := range word {
			if font.MeasureString(face, current+string(r)).Ceil() > width && current != "" {
				lines = append(lines, current)
				current = ""
			}
			current += string(r)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		last := []rune(lines[maxLines-1])
		for len(last) > 0 && font.MeasureString(face, string(last)+"...").Ceil() > width {
			last = last[:len(last)-1]
		}
		lines[maxLines-1] = string(last) + "..."
	}
	return lines
}

func renderSocialPreview(content previewContent) ([]byte, error) {
	title, err := previewFace(gobold.TTF, 56)
	if err != nil {
		return nil, err
	}
	defer title.Close()
	body, err := previewFace(goregular.TTF, 27)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	label, err := previewFace(gobold.TTF, 20)
	if err != nil {
		return nil, err
	}
	defer label.Close()
	brand, err := previewFace(gobold.TTF, 32)
	if err != nil {
		return nil, err
	}
	defer brand.Close()
	canvas := image.NewRGBA(image.Rect(0, 0, 1200, 630))
	background, white, muted, mint := color.RGBA{16, 17, 19, 255}, color.RGBA{246, 248, 249, 255}, color.RGBA{180, 190, 195, 255}, color.RGBA{40, 218, 171, 255}
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(0, 0, 1200, 8), image.NewUniform(mint), image.Point{}, draw.Src)
	write := func(face font.Face, value string, x, y int, ink color.Color) {
		d := font.Drawer{Dst: canvas, Src: image.NewUniform(ink), Face: face, Dot: fixed.P(x, y)}
		d.DrawString(value)
	}
	logo, err := png.Decode(bytes.NewReader(socialBrandPNG))
	if err != nil {
		return nil, err
	}
	xdraw.CatmullRom.Scale(canvas, image.Rect(58, 26, 118, 86), logo, logo.Bounds(), draw.Over, nil)
	write(brand, "AimMod", 132, 72, white)
	write(label, content.Category, 64, 139, mint)
	for i, line := range previewLines(content.Title, title, 1072, 3) {
		write(title, line, 64, 218+i*68, white)
	}
	if len(content.Metrics) == 0 {
		for i, line := range previewLines(content.Description, body, 1040, 3) {
			write(body, line, 64, 423+i*36, muted)
		}
	} else {
		for i, line := range previewLines(content.Description, body, 1040, 2) {
			write(body, line, 64, 402+i*34, muted)
		}
		for i, metric := range content.Metrics {
			x := 64 + i*268
			write(label, metric.Label, x, 482, muted)
			for _, line := range previewLines(metric.Value, brand, 246, 1) {
				write(brand, line, x, 523, white)
			}
		}
	}
	draw.Draw(canvas, image.Rect(64, 548, 1136, 550), image.NewUniform(color.RGBA{53, 61, 66, 255}), image.Point{}, draw.Src)
	write(label, "aimmod.app", 64, 590, mint)
	write(label, "ANALYSIS / PRACTICE / PROGRESS", 740, 590, muted)
	var out bytes.Buffer
	err = png.Encode(&out, canvas)
	return out.Bytes(), err
}
