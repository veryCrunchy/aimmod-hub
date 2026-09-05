package osu

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
)

func TestOfficialReplayDownloadsWithoutAssumingUserPermissionAndRemovesSpool(t *testing.T) {
	payload := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	calls := 0
	s := scoreTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/v2/scores/42/download" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(payload)
	})
	for i := 0; i < 2; i++ {
		result, err := s.DownloadPublicReplay(context.Background(), 42)
		if err != nil || result.Status != "available" {
			t.Fatalf("%+v %v", result, err)
		}
		path := result.Body.(*temporaryReplay).Name()
		data, err := io.ReadAll(result.Body)
		if err != nil || !bytes.Equal(data, payload) {
			t.Fatalf("body %v %v", data, err)
		}
		if err = result.Body.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err = os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("temporary replay retained")
		}
	}
	if calls != 2 {
		t.Fatal("binary/permission was cached")
	}
}

func TestOfficialReplayRejectsFailuresAndNonBinaryResponses(t *testing.T) {
	for _, tc := range []struct {
		code        int
		contentType string
		want        string
	}{
		{401, "", "authentication_failed"}, {403, "", "permission_denied"}, {404, "", "not_found"}, {429, "", "rate_limited"},
		{302, "", "redirect_rejected"}, {200, "text/html", "invalid_response"}, {200, "application/json", "invalid_response"},
	} {
		t.Run(tc.want+strconv.Itoa(tc.code), func(t *testing.T) {
			s := scoreTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.Header().Set("Location", "https://untrusted.example/file")
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte("not a replay"))
			})
			result, err := s.DownloadPublicReplay(context.Background(), 42)
			if err != nil || result.Status != tc.want || result.Body != nil {
				t.Fatalf("%+v %v", result, err)
			}
		})
	}
}

func TestOfficialReplayRejectsDeclaredAndStreamedOversize(t *testing.T) {
	for _, declared := range []bool{true, false} {
		t.Run(strconv.FormatBool(declared), func(t *testing.T) {
			s := scoreTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/octet-stream")
				if declared {
					w.Header().Set("Content-Length", strconv.Itoa(maximumOfficialReplayBytes+1))
					return
				}
				block := make([]byte, 1024*1024)
				for i := 0; i < 65; i++ {
					if _, err := w.Write(block); err != nil {
						return
					}
				}
			})
			result, err := s.DownloadPublicReplay(context.Background(), 42)
			if err != nil || result.Status != "too_large" || result.Body != nil {
				t.Fatalf("%+v %v", result, err)
			}
		})
	}
}

func TestOfficialReplayCancelledRequestNeverDownloads(t *testing.T) {
	s := scoreTestServer(t, func(http.ResponseWriter, *http.Request) { t.Error("unexpected transport") })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.DownloadPublicReplay(ctx, 42); err != context.Canceled {
		t.Fatalf("%v", err)
	}
}
