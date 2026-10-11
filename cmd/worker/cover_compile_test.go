package main

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestCompileWithCover runs the real FFmpeg graph with a cover frame in front
// of a voiced clip, checking the output length and that the cover comes first.
// Skipped when ffmpeg is not installed.
func TestCompileWithCover(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()

	cover := filepath.Join(dir, "cover.jpg")
	clip := filepath.Join(dir, "clip.mp4")
	// A solid red 1080x1920 cover, and a 2s blue clip with a tone (portrait, like phone video).
	mustFFmpeg(t, "-f", "lavfi", "-i", "color=c=red:s=1080x1920", "-frames:v", "1", cover)
	mustFFmpeg(t, "-f", "lavfi", "-i", "color=c=blue:s=720x1280:d=2", "-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		"-shortest", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", clip)

	inputs := []compInput{
		{path: cover, isImage: true, duration: coverDurationSeconds},
		{path: clip, hasAudio: true, duration: 2},
	}

	tests := []struct {
		name      string
		isPreview bool
		assetDir  string
		wantDur   float64
	}{
		{"preview", true, "", coverDurationSeconds + 2},
		{"final with outro", false, "assets", coverDurationSeconds + 2 + 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &videoProcessor{logger: zap.NewNop(), assetDir: tt.assetDir}
			out := filepath.Join(dir, tt.name+".mp4")
			require.NoError(t, p.compile(inputs, "", out, tt.isPreview))

			assert.InDelta(t, tt.wantDur, probeDuration(t, out), 0.3)
			// First second is the red cover; after it, the blue clip.
			assert.Equal(t, "red", dominantColour(t, out, 1.0))
			assert.Equal(t, "blue", dominantColour(t, out, coverDurationSeconds+1.0))
		})
	}
}

func mustFFmpeg(t *testing.T, args ...string) {
	t.Helper()
	out, err := exec.Command("ffmpeg", append([]string{"-y", "-loglevel", "error"}, args...)...).CombinedOutput()
	require.NoError(t, err, string(out))
}

func probeDuration(t *testing.T, path string) float64 {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", path).Output()
	require.NoError(t, err)
	d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	require.NoError(t, err)
	return d
}

// dominantColour samples the centre pixel of the frame at ts seconds.
func dominantColour(t *testing.T, path string, ts float64) string {
	t.Helper()
	out, err := exec.Command("ffmpeg", "-loglevel", "error", "-ss", strconv.FormatFloat(ts, 'f', 2, 64), "-i", path,
		"-frames:v", "1", "-vf", "crop=2:2:iw/2:ih/2,scale=1:1", "-f", "rawvideo", "-pix_fmt", "rgb24", "-").Output()
	require.NoError(t, err)
	require.Len(t, out, 3)
	r, g, b := int(out[0]), int(out[1]), int(out[2])
	switch {
	case r > 150 && g < 80 && b < 80:
		return "red"
	case b > 150 && r < 80 && g < 80:
		return "blue"
	default:
		return "other(" + strconv.Itoa(r) + "," + strconv.Itoa(g) + "," + strconv.Itoa(b) + ")"
	}
}
