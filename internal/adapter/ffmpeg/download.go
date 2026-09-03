package ffmpeg

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

// fetchBundle downloads one archive (zip or tar.xz) into dir. Extraction is
// intentionally not verified here — single-binary bundles (darwin evermeet
// zips) legitimately contain only one of the two binaries, and verification
// happens once in downloadAllFrom after every bundle lands.
func fetchBundle(ctx context.Context, url, dir string) error {
	fmt.Printf("[ffmpeg] Downloading %s\n", url)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 300<<20))
	if err != nil {
		return fmt.Errorf("read download: %w", err)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	switch {
	case strings.HasSuffix(url, ".zip"):
		return extractZip(body, dir)
	case strings.HasSuffix(url, ".tar.xz"):
		return extractTarXz(body, dir)
	default:
		return fmt.Errorf("unsupported archive format: %s", url)
	}
}

func extractZip(body []byte, dir string) error {
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return fmt.Errorf("zip: %w", err)
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(f.Name)
		if base != binaryName("ffmpeg") && base != binaryName("ffprobe") {
			continue
		}
		if err := writeFileFromZip(f, filepath.Join(dir, base)); err != nil {
			return err
		}
		fmt.Printf("[ffmpeg] Extracted %s\n", base)
	}
	return nil
}

func writeFileFromZip(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	return writeExecutable(target, rc)
}

func extractTarXz(body []byte, dir string) error {
	xr, err := xz.NewReader(bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("xz: %w", err)
	}
	tr := tar.NewReader(xr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		if !hdr.FileInfo().Mode().IsRegular() {
			continue
		}
		base := filepath.Base(hdr.Name)
		if base != binaryName("ffmpeg") && base != binaryName("ffprobe") {
			continue
		}
		if err := writeExecutable(filepath.Join(dir, base), tr); err != nil {
			return err
		}
		fmt.Printf("[ffmpeg] Extracted %s\n", base)
	}
	return nil
}

// writeExecutable streams r into a temp file next to target and renames it into
// place once the copy succeeds, so an interrupted or failed write can never
// leave a truncated file at target (which the cache-hit guard would otherwise
// serve forever).
func writeExecutable(target string, r io.Reader) error {
	tmp := target + ".tmp"
	_ = os.Remove(tmp)
	defer os.Remove(tmp)

	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp, target)
}

// verifyExtracted reports an error if either expected binary is missing or
// undersized in dir. It is called once after all platform bundles have been
// fetched, never per-bundle: single-binary bundles (darwin evermeet zips)
// legitimately contain only ffmpeg or only ffprobe.
func verifyExtracted(dir string) error {
	for _, name := range []string{binaryName("ffmpeg"), binaryName("ffprobe")} {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err != nil || info.Size() < minBinarySize {
			return fmt.Errorf("%s missing from cache", name)
		}
	}
	return nil
}

// DownloadURL returns archive URLs for the platform. linux uses a combined
// BtbN tar.xz (ffprobeURL empty); windows uses a gyan.dev zip (both binaries
// inside); darwin uses evermeet.cx single-binary zips. Empty ffmpegURL means
// unsupported platform.
func DownloadURL(goos, goarch string) (ffmpegURL, ffprobeURL string) {
	if goarch != "amd64" && goarch != "arm64" {
		return "", ""
	}
	switch goos {
	case "linux":
		if goarch == "amd64" {
			return "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linux64-gpl.tar.xz", ""
		}
		return "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linuxarm64-gpl.tar.xz", ""
	case "windows":
		if goarch == "amd64" {
			return "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip", ""
		}
		return "", ""
	case "darwin":
		if goarch == "amd64" {
			return "https://evermeet.cx/ffmpeg/getrelease/ffmpeg/zip", "https://evermeet.cx/ffmpeg/getrelease/ffprobe/zip"
		}
		return "", ""
	default:
		return "", ""
	}
}
