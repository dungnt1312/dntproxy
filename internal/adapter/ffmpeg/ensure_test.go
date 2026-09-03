package ffmpeg

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDownloadURLPlatforms(t *testing.T) {
	cases := []struct {
		goos, goarch      string
		wantFF, wantProbe string
	}{
		{"linux", "amd64", "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linux64-gpl.tar.xz", ""},
		{"linux", "arm64", "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linuxarm64-gpl.tar.xz", ""},
		{"windows", "amd64", "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip", ""},
		{"darwin", "amd64", "https://evermeet.cx/ffmpeg/getrelease/ffmpeg/zip", "https://evermeet.cx/ffmpeg/getrelease/ffprobe/zip"},
		{"darwin", "arm64", "", ""},
		{"solaris", "amd64", "", ""},
	}
	for _, tc := range cases {
		ff, probe := DownloadURL(tc.goos, tc.goarch)
		if ff != tc.wantFF || probe != tc.wantProbe {
			t.Errorf("%s/%s = (%q, %q), want (%q, %q)", tc.goos, tc.goarch, ff, probe, tc.wantFF, tc.wantProbe)
		}
	}
}

func TestLocateEnvOverride(t *testing.T) {
	dir := t.TempDir()
	ff := filepath.Join(dir, "ffmpeg")
	probe := filepath.Join(dir, "ffprobe")
	for _, p := range []string{ff, probe} {
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("DNTPROXY_FFMPEG_PATH", ff)
	t.Setenv("DNTPROXY_FFPROBE_PATH", probe)

	gotFF, gotProbe, err := Locate(context.Background())
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if gotFF != ff || gotProbe != probe {
		t.Fatalf("Locate = (%q, %q), want overrides", gotFF, gotProbe)
	}
}

func TestLocateUsesPathBeforeDownload(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("no system ffmpeg; PATH branch not testable")
	}
	ff, probe, err := Locate(context.Background())
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if ff == "" || probe == "" {
		t.Fatalf("expected both paths, got (%q, %q)", ff, probe)
	}
}
