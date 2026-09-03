package ffmpeg

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ulikunitz/xz"
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

// --- Hermetic download/extraction tests ------------------------------------
//
// These exercise the real fetchBundle/extract/verify path with synthetic
// archives served from a local httptest server: no network, no ~/.dntproxy.
// downloadAllFrom takes explicit URLs + dir precisely so the extraction layer
// can be tested end-to-end without touching the default cache dir.

func TestExtractZipBundle(t *testing.T) {
	names := []string{binaryName("ffmpeg"), binaryName("ffprobe")}
	body := makeZip(t, names...)
	srv := archiveServer(t, body)

	dir := t.TempDir()
	if err := downloadAllFrom(context.Background(), dir, srv.URL+"/0.zip", ""); err != nil {
		t.Fatalf("downloadAllFrom: %v", err)
	}
	assertBinaries(t, dir, true, true, true)
}

func TestExtractTarXzBundle(t *testing.T) {
	// BtbN tar.xz archives are only produced for linux, but the extractor
	// filters with binaryName() (plain names on unix), so the same synthetic
	// archive exercises the full flow on every platform.
	names := []string{binaryName("ffmpeg"), binaryName("ffprobe")}
	body := makeTarXz(t, names...)
	srv := archiveServer(t, body)

	dir := t.TempDir()
	if err := downloadAllFrom(context.Background(), dir, srv.URL+"/0.tar.xz", ""); err != nil {
		t.Fatalf("downloadAllFrom: %v", err)
	}
	assertBinaries(t, dir, true, true, true)
}

// TestDarwinSeparateBundles is the regression test for the critical fix:
// evermeet.cx darwin zips are single-binary. The ffmpeg-only bundle is
// fetched first and must NOT be rejected just because ffprobe is absent yet;
// only after the ffprobe-only bundle lands is verification allowed to run.
func TestDarwinSeparateBundles(t *testing.T) {
	ffmpegZip := makeZip(t, binaryName("ffmpeg"))
	ffprobeZip := makeZip(t, binaryName("ffprobe"))
	srv := archiveServer(t, ffmpegZip, ffprobeZip)

	dir := t.TempDir()
	if err := downloadAllFrom(context.Background(), dir, srv.URL+"/0.zip", srv.URL+"/1.zip"); err != nil {
		t.Fatalf("downloadAllFrom with two single-binary bundles: %v", err)
	}
	assertBinaries(t, dir, true, true, true)
}

// TestDownloadAllMissingBundle verifies single verification point rejects a
// combined flow when one binary never arrives.
func TestDownloadAllMissingBundle(t *testing.T) {
	ffmpegZip := makeZip(t, binaryName("ffmpeg"))
	srv := archiveServer(t, ffmpegZip)

	dir := t.TempDir()
	// probeURL points at the same ffmpeg-only zip, so ffprobe never lands.
	err := downloadAllFrom(context.Background(), dir, srv.URL+"/0.zip", srv.URL+"/0.zip")
	if err == nil {
		t.Fatal("expected error when ffprobe is never delivered")
	}
	assertBinaries(t, dir, true, false, true)
}

// TestExtractSingleBinaryZipStillYieldsBinary guards that per-bundle
// extraction was not weakened: a lone single-binary zip must still produce
// its own binary (even though full verification would fail).
func TestExtractSingleBinaryZipStillYieldsBinary(t *testing.T) {
	body := makeZip(t, binaryName("ffmpeg"))
	dir := t.TempDir()
	if err := fetchBundle(context.Background(), archiveURL(t, body, ".zip"), dir); err != nil {
		t.Fatalf("fetchBundle: %v", err)
	}
	// ffprobe legitimately absent; only ffmpeg must exist.
	assertBinaries(t, dir, true, false, false)
}

// TestWriteExecutableAtomic verifies a failed copy never leaves a truncated
// file at the final target and that temp files are cleaned up.
func TestWriteExecutableAtomic(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, binaryName("ffmpeg"))

	failReader := &errorReader{err: io.ErrUnexpectedEOF}
	if err := writeExecutable(target, failReader); err == nil {
		t.Fatal("expected write error from failing reader")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target exists after failed write: %v", err)
	}
	if _, err := os.Stat(target + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind after failed write: %v", err)
	}

	if err := writeExecutable(target, bytes.NewReader(makeBinPayload())); err != nil {
		t.Fatalf("writeExecutable: %v", err)
	}
	assertBinaries(t, dir, true, false, true)
}

// --- helpers ----------------------------------------------------------------

// assertBinaries checks which of ffmpeg/ffprobe exist in dir. wantFF/wantProbe
// select the names to assert on; wantExec checks the 0755 executable bit.
func assertBinaries(t *testing.T, dir string, wantFF, wantProbe, wantExec bool) {
	t.Helper()
	for name, want := range map[string]bool{binaryName("ffmpeg"): wantFF, binaryName("ffprobe"): wantProbe} {
		p := filepath.Join(dir, name)
		info, err := os.Stat(p)
		if err != nil {
			if !want {
				continue
			}
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !want {
			t.Errorf("%s exists but was not expected", name)
			continue
		}
		if info.Size() <= minBinarySize {
			t.Errorf("%s size %d, want > %d", name, info.Size(), minBinarySize)
		}
		if wantExec && runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
			t.Errorf("%s mode %v, want executable bit set", name, info.Mode())
		}
	}
}

// archiveServer serves each archive at a distinct path /0.zip, /1.zip, ...
func archiveServer(t *testing.T, bodies ...[]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var idx int
		if _, err := fmt.Sscanf(r.URL.Path, "/%d", &idx); err != nil || idx < 0 || idx >= len(bodies) {
			http.NotFound(w, r)
			return
		}
		w.Write(bodies[idx])
	}))
	t.Cleanup(srv.Close)
	return srv
}

// archiveURL encodes body into the archive format indicated by suffix and
// returns a locally served URL (for tests that only need one archive).
func archiveURL(t *testing.T, body []byte, suffix string) string {
	t.Helper()
	srv := archiveServer(t, body)
	if suffix == ".tar.xz" {
		return srv.URL + "/0.tar.xz"
	}
	return srv.URL + "/0.zip"
}

func makeZip(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(makeBinPayload()); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func makeTarXz(t *testing.T, names ...string) []byte {
	t.Helper()
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	for _, name := range names {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(makeBinPayload())),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(makeBinPayload()); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	xw, err := xz.NewWriter(&out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := xw.Write(raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := xw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// makeBinPayload returns > 1024 bytes of fake binary data.
func makeBinPayload() []byte {
	payload := bytes.Repeat([]byte{0x7f, 0x45, 0x4c, 0x46, 0x01}, minBinarySize/5+1)
	if len(payload) <= minBinarySize {
		payload = bytes.Repeat([]byte{0xAA}, minBinarySize+1)
	}
	return payload
}

// errorReader fails after a first successful chunk so a partial copy is
// actually written before the error surfaces.
type errorReader struct {
	err error
	n   int
}

func (e *errorReader) Read(p []byte) (int, error) {
	if e.n == 0 {
		e.n = 100
		for i := range p[:100] {
			p[i] = 0xAA
		}
		return 100, nil
	}
	return 0, e.err
}
