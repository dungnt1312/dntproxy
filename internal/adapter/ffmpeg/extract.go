package ffmpeg

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FrameTimestamps returns evenly spaced sample points across duration,
// offset by half a step so frames avoid the exact boundaries. Durations <= 0
// yield a single frame at 0.
func FrameTimestamps(duration time.Duration, maxFrames int) []time.Duration {
	if maxFrames < 1 {
		maxFrames = 1
	}
	if duration <= 0 {
		return []time.Duration{0}
	}
	step := duration / time.Duration(maxFrames)
	ts := make([]time.Duration, 0, maxFrames)
	for i := 0; i < maxFrames; i++ {
		ts = append(ts, step*time.Duration(i)+step/2)
	}
	return ts
}

// ExtractFrames extracts up to maxFrames evenly spaced JPEG frames from
// videoPath into dir (which must exist), returning frame paths in
// chronological order. Frames whose extraction fails are skipped; an error
// is returned only when no frame could be extracted or probing fails.
func ExtractFrames(ctx context.Context, ffmpegPath, ffprobePath, videoPath, dir string, maxFrames int) ([]string, error) {
	duration, err := probeDuration(ctx, ffprobePath, videoPath)
	if err != nil {
		return nil, err
	}

	timestamps := FrameTimestamps(duration, maxFrames)
	frames := make([]string, 0, len(timestamps))
	for i, ts := range timestamps {
		out := filepath.Join(dir, fmt.Sprintf("frame-%03d.jpg", i))
		args := []string{
			"-hide_banner", "-loglevel", "error",
			"-ss", formatSeconds(ts),
			"-i", videoPath,
			"-frames:v", "1",
			"-vf", "scale='min(1280,iw)':-2",
			"-q:v", "4",
			"-y", out,
		}
		cmd := exec.CommandContext(ctx, ffmpegPath, args...)
		if outBytes, err := cmd.CombinedOutput(); err != nil {
			// Likely past EOF for short videos — skip and keep going.
			fmt.Printf("[ffmpeg] frame %d at %v failed: %v\n%s\n", i, ts, err, outBytes)
			continue
		}
		if info, err := os.Stat(out); err == nil && info.Size() > 0 {
			frames = append(frames, out)
		}
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("no frames extracted from %s", videoPath)
	}
	return frames, nil
}

func probeDuration(ctx context.Context, ffprobePath, videoPath string) (time.Duration, error) {
	out, err := exec.CommandContext(ctx, ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	).Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe duration: %w", err)
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || math.IsNaN(seconds) || seconds <= 0 {
		return 0, fmt.Errorf("ffprobe returned unusable duration %q", strings.TrimSpace(string(out)))
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

func formatSeconds(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', 3, 64)
}
