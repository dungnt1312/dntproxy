// Package ffmpeg locates or auto-downloads ffmpeg/ffprobe binaries, mirroring
// the tunnel adapter's cloudflared lifecycle (system PATH → env override →
// cached ~/.dntproxy/bin/ → platform download).
package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Env overrides for pre-installed binaries.
const (
	EnvFFmpegPath  = "DNTPROXY_FFMPEG_PATH"
	EnvFFprobePath = "DNTPROXY_FFPROBE_PATH"
)

// Locate returns executable paths for ffmpeg and ffprobe. Resolution order
// per binary: env override → system PATH → cached ~/.dntproxy/bin/ → download.
func Locate(ctx context.Context) (string, string, error) {
	ff, err := resolveOne(ctx, EnvFFmpegPath, "ffmpeg", binaryName("ffmpeg"))
	if err != nil {
		return "", "", fmt.Errorf("ffmpeg: %w", err)
	}
	probe, err := resolveOne(ctx, EnvFFprobePath, "ffprobe", binaryName("ffprobe"))
	if err != nil {
		return "", "", fmt.Errorf("ffprobe: %w", err)
	}
	return ff, probe, nil
}

func resolveOne(ctx context.Context, envKey, lookPath, cachedName string) (string, error) {
	if p := os.Getenv(envKey); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("%s=%q not found", envKey, p)
	}
	if p, err := exec.LookPath(lookPath); err == nil {
		return p, nil
	}

	cached := filepath.Join(cacheDir(), cachedName)
	if info, err := os.Stat(cached); err == nil && info.Size() > 1024 {
		return cached, nil
	}

	if err := downloadAll(ctx); err != nil {
		return "", fmt.Errorf("auto-download failed (or install ffmpeg and retry): %w", err)
	}
	if info, err := os.Stat(cached); err == nil && info.Size() > 1024 {
		return cached, nil
	}
	return "", fmt.Errorf("download completed but %s is missing", cached)
}

func binaryName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func cacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".dntproxy", "bin")
}

// downloadAll fetches the platform bundle once; single-binary bundles
// (darwin) download ffmpeg and ffprobe separately.
func downloadAll(ctx context.Context) error {
	ffURL, probeURL := DownloadURL(runtime.GOOS, runtime.GOARCH)
	if ffURL == "" {
		return fmt.Errorf("unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if err := fetchBundle(ctx, ffURL); err != nil {
		return err
	}
	if probeURL != "" {
		if err := fetchBundle(ctx, probeURL); err != nil {
			return err
		}
	}
	return nil
}
