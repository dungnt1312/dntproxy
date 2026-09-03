package ffmpeg

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestFrameTimestamps(t *testing.T) {
	got := FrameTimestamps(10*time.Second, 4)
	want := []time.Duration{1250 * time.Millisecond, 3750 * time.Millisecond, 6250 * time.Millisecond, 8750 * time.Millisecond}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("FrameTimestamps[%d] = %v, want %v (all: %v)", i, got[i], want[i], got)
		}
	}

	if ts := FrameTimestamps(0, 4); len(ts) != 1 {
		t.Fatalf("zero duration should yield single frame at 0, got %v", ts)
	}
}

// hasFFmpeg reports whether a real ffmpeg is available for integration tests.
func hasFFmpeg(t *testing.T) (string, string) {
	t.Helper()
	ff, probe, err := Locate(context.Background())
	if err != nil {
		t.Skip("ffmpeg/ffprobe not available:", err)
	}
	return ff, probe
}

func TestExtractFramesIntegration(t *testing.T) {
	ff, probe := hasFFmpeg(t)
	ctx := context.Background()

	dir := t.TempDir()
	fixture := filepath.Join(dir, "fixture.mp4")
	gen := exec.CommandContext(ctx, ff,
		"-f", "lavfi", "-i", "testsrc=duration=2:size=320x240:rate=10",
		"-y", fixture)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("cannot generate fixture: %v\n%s", err, out)
	}

	framesDir := t.TempDir()
	frames, err := ExtractFrames(ctx, ff, probe, fixture, framesDir, 4)
	if err != nil {
		t.Fatalf("ExtractFrames: %v", err)
	}
	if len(frames) != 4 {
		t.Fatalf("got %d frames, want 4: %v", len(frames), frames)
	}
	for i, f := range frames {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read frame %d: %v", i, err)
		}
		if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
			t.Fatalf("frame %d is not JPEG: %s (%d bytes)", i, f, len(data))
		}
	}
}

func TestExtractFramesDownscales(t *testing.T) {
	ff, probe := hasFFmpeg(t)
	ctx := context.Background()
	dir := t.TempDir()
	fixture := filepath.Join(dir, "wide.mp4")
	gen := exec.CommandContext(ctx, ff,
		"-f", "lavfi", "-i", "testsrc=duration=1:size=1920x1080:rate=10",
		"-y", fixture)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("cannot generate fixture: %v\n%s", err, out)
	}
	frames, err := ExtractFrames(ctx, ff, probe, fixture, t.TempDir(), 1)
	if err != nil {
		t.Fatalf("ExtractFrames: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("got %d frames, want 1", len(frames))
	}
}
