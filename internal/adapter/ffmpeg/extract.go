package ffmpeg

import (
	"context"
	"fmt"
	"log"
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

// Frame is one extracted JPEG with the time range of the video it
// represents. Ranges are bounded by the midpoints between consecutive
// samples: the first frame starts at 0 and the last ends at the video
// duration.
type Frame struct {
	Path      string
	Timestamp time.Duration
	Start     time.Duration
	End       time.Duration
}

// frameRanges pairs sample points with the segment each represents:
// [0, mid01], [mid01, mid02], ..., [midN-1N, duration].
func frameRanges(timestamps []time.Duration, duration time.Duration) [][2]time.Duration {
	ranges := make([][2]time.Duration, len(timestamps))
	for i, ts := range timestamps {
		start := time.Duration(0)
		if i > 0 {
			start = (timestamps[i-1] + ts) / 2
		}
		end := duration
		if i < len(timestamps)-1 {
			end = (ts + timestamps[i+1]) / 2
		}
		ranges[i] = [2]time.Duration{start, end}
	}
	return ranges
}

// ExtractFrames extracts up to maxFrames evenly spaced JPEG frames from
// videoPath into dir (which must exist), returning frames in chronological
// order with the time range each represents. Frames whose extraction fails
// are skipped; an error is returned only when no frame could be extracted or
// probing fails.
func ExtractFrames(ctx context.Context, ffmpegPath, ffprobePath, videoPath, dir string, maxFrames int) ([]Frame, error) {
	duration, err := probeDuration(ctx, ffprobePath, videoPath)
	if err != nil {
		return nil, err
	}

	timestamps := FrameTimestamps(duration, maxFrames)
	ranges := frameRanges(timestamps, duration)
	frames := make([]Frame, 0, len(timestamps))
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
			log.Printf("[ffmpeg] frame %d at %v failed: %v\n%s", i, ts, err, outBytes)
			continue
		}
		if info, err := os.Stat(out); err == nil && info.Size() > 0 {
			frames = append(frames, Frame{
				Path:      out,
				Timestamp: ts,
				Start:     ranges[i][0],
				End:       ranges[i][1],
			})
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
