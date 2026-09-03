package mcp

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var pngBytes = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}

func TestFetchImageAsDataURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer srv.Close()

	got, err := FetchImageAsDataURL(context.Background(), srv.Client(), srv.URL+"/img.png")
	if err != nil {
		t.Fatalf("FetchImageAsDataURL: %v", err)
	}
	want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)
	if got != want {
		t.Fatalf("got %q... want %q...", got[:40], want[:40])
	}
}

func TestFetchImageDataURLPassthrough(t *testing.T) {
	raw := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)
	got, err := FetchImageAsDataURL(context.Background(), http.DefaultClient, raw)
	if err != nil || got != raw {
		t.Fatalf("passthrough broken: %q, %v", got, err)
	}
}

func TestFetchImageRejectsBadScheme(t *testing.T) {
	if _, err := FetchImageAsDataURL(context.Background(), http.DefaultClient, "file:///etc/passwd"); err == nil {
		t.Fatal("file:// scheme must be rejected")
	}
}

func TestFetchImageRejectsOversize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(make([]byte, MaxImageBytes+1))
	}))
	defer srv.Close()

	if _, err := FetchImageAsDataURL(context.Background(), srv.Client(), srv.URL+"/big.png"); err == nil {
		t.Fatal("oversized image must be rejected")
	}
}

func TestDownloadVideo(t *testing.T) {
	payload := strings.Repeat("MPEGDATA", 1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	path, cleanup, err := DownloadVideo(context.Background(), srv.Client(), srv.URL+"/v.mp4")
	if err != nil {
		t.Fatalf("DownloadVideo: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("downloaded file missing: %v", err)
	}
	if filepath.Ext(path) != ".mp4" {
		t.Fatalf("ext = %q, want .mp4", filepath.Ext(path))
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("cleanup must remove the downloaded file")
	}
}

func TestDownloadVideoRejectsOversize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, MaxVideoBytes+1))
	}))
	defer srv.Close()

	_, cleanup, err := DownloadVideo(context.Background(), srv.Client(), srv.URL+"/huge.mp4")
	if err == nil {
		cleanup()
		t.Fatal("oversized video must be rejected")
	}
}

func TestDownloadVideoTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, cleanup, err := DownloadVideo(ctx, srv.Client(), srv.URL+"/slow.mp4"); err == nil {
		cleanup()
		t.Fatal("context deadline must abort the download")
	}
}
