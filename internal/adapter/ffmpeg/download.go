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

// fetchBundle downloads one archive (zip or tar.xz) into the cache dir.
func fetchBundle(ctx context.Context, url string) error {
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

	if err := os.MkdirAll(cacheDir(), 0o755); err != nil {
		return err
	}

	switch {
	case strings.HasSuffix(url, ".zip"):
		return extractZip(body)
	case strings.HasSuffix(url, ".tar.xz"):
		return extractTarXz(body)
	default:
		return fmt.Errorf("unsupported archive format: %s", url)
	}
}

func extractZip(body []byte) error {
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return fmt.Errorf("zip: %w", err)
	}
	for _, f := range zr.File {
		base := filepath.Base(f.Name)
		if base != "ffmpeg.exe" && base != "ffprobe.exe" && base != "ffmpeg" && base != "ffprobe" {
			continue
		}
		if f.FileInfo().IsDir() {
			continue
		}
		if err := writeFileFromZip(zr, f, filepath.Join(cacheDir(), base)); err != nil {
			return err
		}
		fmt.Printf("[ffmpeg] Extracted %s\n", base)
	}
	return verifyExtracted()
}

func writeFileFromZip(zr *zip.Reader, f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	return writeExecutable(target, rc)
}

func extractTarXz(body []byte) error {
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
		base := filepath.Base(hdr.Name)
		if base != "ffmpeg" && base != "ffprobe" {
			continue
		}
		if err := writeExecutable(filepath.Join(cacheDir(), base), tr); err != nil {
			return err
		}
		fmt.Printf("[ffmpeg] Extracted %s\n", base)
	}
	return verifyExtracted()
}

func writeExecutable(target string, r io.Reader) error {
	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func verifyExtracted() error {
	for _, name := range []string{binaryName("ffmpeg"), binaryName("ffprobe")} {
		p := filepath.Join(cacheDir(), name)
		if info, err := os.Stat(p); err != nil || info.Size() < 1024 {
			return fmt.Errorf("extraction incomplete: %s missing from archive", name)
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
