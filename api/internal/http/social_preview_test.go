package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/veryCrunchy/aimmod-hub/api/internal/seo"
	"github.com/veryCrunchy/aimmod-hub/api/internal/store"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
)

type previewStoreStub struct {
	seoStoreStub
	calls int
}

func (s *previewStoreStub) GetOsuPublicReplay(ctx context.Context, id string) (store.OsuPublicReplay, error) {
	s.calls++
	return s.seoStoreStub.GetOsuPublicReplay(ctx, id)
}
func (s *previewStoreStub) GetProfileMeta(context.Context, string) (*store.ProfileMeta, error) {
	return nil, nil
}
func (s *previewStoreStub) GetScenarioMeta(context.Context, string) (*store.ScenarioMeta, error) {
	return nil, nil
}
func (s *previewStoreStub) GetRunMeta(context.Context, string) (*store.RunMeta, error) {
	return nil, nil
}

func previewRequest(h *socialPreviewHandler, route, method, etag string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, "/social-preview.png?path="+url.QueryEscape(route)+"&v=1", nil)
	request.Header.Set("If-None-Match", etag)
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	return response
}

func TestSocialPreviewPNGAndDeterministicCache(t *testing.T) {
	h := newSocialPreviewHandler(nil).(*socialPreviewHandler)
	first := previewRequest(h, "/osu/learn", "GET", "")
	if first.Code != 200 || first.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("bad response %d %s", first.Code, first.Body.String())
	}
	decoded, err := png.Decode(bytes.NewReader(first.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 1200 || decoded.Bounds().Dy() != 630 {
		t.Fatal(decoded.Bounds())
	}
	if decoded.At(70, 3) == decoded.At(1100, 300) {
		t.Fatal("blank image")
	}
	second := previewRequest(h, "/osu/learn", "GET", "")
	if !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) || h.lru.Len() != 1 {
		t.Fatal("non-deterministic cache")
	}
	conditional := previewRequest(h, "/osu/learn", "GET", "W/"+first.Header().Get("ETag"))
	if conditional.Code != 304 || conditional.Body.Len() != 0 {
		t.Fatal("conditional response")
	}
	head := previewRequest(h, "/osu/learn", "HEAD", "")
	if head.Code != 200 || head.Body.Len() != 0 || head.Header().Get("Content-Length") == "" {
		t.Fatal("HEAD response")
	}
}

func TestSocialPreviewChecksVisibilityBeforeCachedBytesAndETag(t *testing.T) {
	st := &previewStoreStub{seoStoreStub: seoStoreStub{replay: store.OsuPublicReplay{Title: "Public title", Visibility: "public", OsuUsername: "Player", Accuracy: .97, MaxCombo: 123}}}
	h := newSocialPreviewRenderer(func(ctx context.Context, route string) (previewContent, error) {
		return resolveSocialContent(ctx, route, st)
	})
	first := previewRequest(h, "/osu/replays/example", "GET", "")
	if first.Code != 200 {
		t.Fatal(first.Code)
	}
	st.replay.Visibility = "unlisted"
	hidden := previewRequest(h, "/osu/replays/example", "GET", first.Header().Get("ETag"))
	if hidden.Code != 404 || hidden.Header().Get("Cache-Control") != "no-store" || hidden.Header().Get("ETag") != "" {
		t.Fatal("unlisted cached image leaked")
	}
	st.replay.Visibility = "private"
	if previewRequest(h, "/osu/replays/example", "HEAD", "*").Code != 404 {
		t.Fatal("private image leaked")
	}
	st.replay.Visibility = "public"
	st.replay.Accuracy = .82
	changed := previewRequest(h, "/osu/replays/example", "GET", first.Header().Get("ETag"))
	if changed.Code != 200 || changed.Header().Get("ETag") == first.Header().Get("ETag") || st.calls < 6 {
		t.Fatal("public state not rechecked")
	}
}

func TestSocialPreviewRejectsUnrecognizedRoutesAndParameters(t *testing.T) {
	h := newSocialPreviewHandler(nil).(*socialPreviewHandler)
	for _, route := range []string{"/admin", "/account", "/link-device", "/search", "/no-such-route", "/osu/learn/unknown", "/learn/unknown", "/learn/topics/unknown", "/profiles/a/extra"} {
		if result := previewRequest(h, route, "GET", ""); result.Code != 404 {
			t.Errorf("%s: %d", route, result.Code)
		}
	}
	for _, route := range []string{"https://example.com", "//example.com", "/../osu", "/osu/", "/osu?q=secret", "/osu#secret", "/osu\\x", "/osu%2fadmin", "/osu\x00", strings.Repeat("x", 513)} {
		if result := previewRequest(h, route, "GET", ""); result.Code != 400 {
			t.Errorf("invalid path %q: %d", route, result.Code)
		}
	}
	for _, query := range []string{"path=/osu&title=secret", "path=/osu&v=2", "path=/osu&v=1&v=1", "path=/osu&path=/app", "path=/osu&url=https://example.com"} {
		recorder := httptest.NewRecorder()
		h.ServeHTTP(recorder, httptest.NewRequest("GET", "/social-preview.png?"+query, nil))
		if recorder.Code != 400 {
			t.Fatal(query, recorder.Code)
		}
	}
	if previewRequest(h, "/osu", "POST", "").Code != 405 {
		t.Fatal("mutation method allowed")
	}
}

func TestSocialPreviewMetricsUseActualNullableValues(t *testing.T) {
	st := &previewStoreStub{seoStoreStub: seoStoreStub{replay: store.OsuPublicReplay{Visibility: "public", OsuUsername: "Actual player", Accuracy: .975, MaxCombo: 432, Mods: []string{"HD", "HR"}}, profile: store.OsuPublicProfile{OsuUsername: "Player", SharedReplayCount: 7}}}
	content, err := resolveSocialContent(context.Background(), "/osu/replays/example", st)
	if err != nil {
		t.Fatal(err)
	}
	if content.Metrics[0].Value != "97.50%" || content.Metrics[1].Value != "Unavailable" || content.Metrics[2].Value != "432x" || content.Metrics[3].Value != "HD+HR" || !strings.Contains(content.Description, "Actual player") {
		t.Fatalf("%+v", content)
	}
	profile, err := resolveSocialContent(context.Background(), "/osu/profiles/player", st)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Metrics[0].Value != "Unavailable" || profile.Metrics[1].Value != "Unranked" || profile.Metrics[2].Value != "7" {
		t.Fatalf("%+v", profile)
	}
	st.profile.SharedReplayCount = 0
	if _, err := resolveSocialContent(context.Background(), "/osu/profiles/player", st); err == nil {
		t.Fatal("private profile accepted")
	}
}

func TestSocialPreviewTextFitsAndJapaneseGlyphsExist(t *testing.T) {
	face, err := previewFace(gobold.TTF, 56)
	if err != nil {
		t.Fatal(err)
	}
	defer face.Close()
	for _, r := range "初音ミク日本語の楽曲" {
		if _, ok := face.GlyphAdvance(r); !ok {
			t.Fatalf("missing Japanese glyph %c", r)
		}
	}
	for _, text := range []string{strings.Repeat("Long title ", 200), strings.Repeat("W", 1000), strings.Repeat("初音ミク", 100)} {
		lines := previewLines(text, face, 1072, 3)
		if len(lines) > 3 {
			t.Fatal("too many lines")
		}
		for _, line := range lines {
			if font.MeasureString(face, line).Ceil() > 1072 {
				t.Fatal("overflow", line)
			}
		}
	}
	if !strings.Contains(socialFontLicense, "SIL OPEN FONT LICENSE") {
		t.Fatal("font license missing")
	}
}

func TestSocialPreviewCacheIsBounded(t *testing.T) {
	h := newSocialPreviewHandler(nil).(*socialPreviewHandler)
	for i := 0; i < 128; i++ {
		key := fmt.Sprint(i)
		h.cache[key] = h.lru.PushFront(previewImage{key: key, data: []byte{1}})
		h.bytes++
	}
	if previewRequest(h, "/osu", "GET", "").Code != 200 {
		t.Fatal("render failed")
	}
	if h.lru.Len() != 128 || len(h.cache) != 128 || h.cache["0"] != nil || h.bytes > 24<<20 {
		t.Fatal("cache unbounded")
	}
}

func TestSocialPreviewArtifacts(t *testing.T) {
	dir := os.Getenv("SOCIAL_PREVIEW_OUTPUT_DIR")
	if dir == "" {
		t.Skip("set SOCIAL_PREVIEW_OUTPUT_DIR to export review artifacts")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	examples := map[string]previewContent{}
	for name, route := range map[string]string{"osu-catalog": "/osu/beatmaps", "osu-guide": "/osu/learn/" + seo.Published.Guides[0].Slug, "kovaaks-guide": "/learn/clicking-precision-balance"} {
		value, err := resolveSocialContent(context.Background(), route, nil)
		if err != nil {
			t.Fatal(err)
		}
		examples[name] = value
	}
	examples["japanese-title"] = previewContent{Title: "初音ミク - 日本語の楽曲とリプレイ分析", Description: "Japanese glyph rendering specimen / osu! replay analysis", Category: "OSU! / TYPOGRAPHY SPECIMEN"}
	if fixture := os.Getenv("SOCIAL_PREVIEW_REPLAY_FIXTURE"); fixture != "" {
		data, err := os.ReadFile(fixture)
		if err != nil {
			t.Fatal(err)
		}
		var replay store.OsuPublicReplay
		if err = json.Unmarshal(data, &replay); err != nil {
			t.Fatal(err)
		}
		content, err := resolveSocialContent(context.Background(), "/osu/replays/"+replay.ShareID, &previewStoreStub{seoStoreStub: seoStoreStub{replay: replay}})
		if err != nil {
			t.Fatal(err)
		}
		examples["public-replay"] = content
	}
	for name, content := range examples {
		image, err := renderSocialPreview(content)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, name+".png"), image, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
