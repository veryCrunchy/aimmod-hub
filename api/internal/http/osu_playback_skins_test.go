package httpserver

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func skinTestArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, data := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = file.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestSkinGameplayFilter(t *testing.T) {
	packed, err := prepareOsuSkin(skinTestArchive(t, map[string]string{
		"Skin.ini":         "[Fonts]\r\nHitCirclePrefix: Assets/default/default\r\n",
		"hitcircle@2x.png": "circle", "Assets/default/default-1@2x.png": "number",
		"normal-hitnormal.wav": "sound", "followpoint-0.png": "point", "spinner-circle.png": "spinner",
		"menu-background.png": "unused", "example.osr": "unused", "../cursor.png": "unsafe",
	}))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(packed), int64(len(packed)))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 6 {
		t.Fatalf("got %d gameplay files", len(reader.File))
	}
	for _, file := range reader.File {
		if file.Name == "menu-background.png" || file.Name == "example.osr" || file.Name == "../cursor.png" {
			t.Fatal("unrelated file retained")
		}
	}
}

func TestSkinRouteCacheAndSourceFailure(t *testing.T) {
	h := newOsuSkinHandler()
	h.cache["yugen"] = skinTestArchive(t, map[string]string{"skin.ini": "[General]"})
	h.client.Transport = playbackRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader([]byte("changed release")))}, nil
	})
	for _, route := range []string{"unknown", "../yugen", "yugen?url=https://example.com"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", osuSkinPrefix+route, nil))
		if w.Code != 404 {
			t.Fatalf("%s: %d", route, w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", osuSkinPrefix+"yugen", nil))
	if w.Code != 200 || w.Header().Get("ETag") == "" {
		t.Fatal("cached skin not served")
	}
	req := httptest.NewRequest("GET", osuSkinPrefix+"yugen", nil)
	req.Header.Set("If-None-Match", w.Header().Get("ETag"))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 304 {
		t.Fatalf("cache validator: %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", osuSkinPrefix+"rafis", nil))
	if w.Code != 502 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("changed release was accepted or cached")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", osuSkinPrefix+"yugen", nil))
	if w.Code != 405 {
		t.Fatal("unexpected method accepted")
	}
}

// Optional verification against downloaded public skin releases. No artwork or
// local paths are stored in fixtures or published with the application.
func TestDownloadedPresetSkins(t *testing.T) {
	directory := os.Getenv("AIMMOD_TEST_SKIN_ARCHIVE_DIR")
	if directory == "" {
		t.Skip("set AIMMOD_TEST_SKIN_ARCHIVE_DIR for downloaded release verification")
	}
	for id := range osuSkinSources {
		t.Run(id, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(directory, id+".osk"))
			if err != nil {
				t.Fatal(err)
			}
			packed, err := prepareOsuSkin(data)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("gameplay archive: %d bytes", len(packed))
			if out := os.Getenv("AIMMOD_TEST_SKIN_OUTPUT_DIR"); out != "" {
				if err := os.WriteFile(filepath.Join(out, id+".osk"), packed, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
