//go:build integration

package integration_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kithly/kithly-ws-go/pkg/abr"
)

// mustMakeClip renders a synthetic clip so the test needs no fixtures.
func mustMakeClip(t *testing.T, path, size string, withAudio bool) {
	t.Helper()
	args := []string{"-loglevel", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=" + size + ":rate=30"}
	if withAudio {
		args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000")
	}
	args = append(args, "-t", "9", "-c:v", "libx264", "-pix_fmt", "yuv420p")
	if withAudio {
		args = append(args, "-c:a", "aac")
	}
	args = append(args, path)
	out, err := exec.Command("ffmpeg", args...).CombinedOutput()
	require.NoError(t, err, string(out))
}

func TestABRPackager_RealFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	tests := []struct {
		name      string
		size      string
		withAudio bool
		wantDirs  []string
	}{
		{"portrait card with audio", "720x1280", true, []string{"720p", "480p", "360p", "audio"}},
		{"landscape contribution without audio", "854x480", false, []string{"480p", "360p"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "src.mp4")
			mustMakeClip(t, src, tt.size, tt.withAudio)

			out := filepath.Join(dir, "out")
			opts := abr.DefaultOptions()
			opts.Preset = "ultrafast"
			res, err := abr.NewPackager(abr.ExecRunner{}, abr.DefaultLadder).
				Package(context.Background(), src, out, opts)
			require.NoError(t, err)
			assert.Equal(t, tt.withAudio, res.HasAudio)

			for _, d := range tt.wantDirs {
				assert.FileExists(t, filepath.Join(out, d, "index.m3u8"))
				assert.FileExists(t, filepath.Join(out, d, "seg_000.m4s"))
				inits, _ := filepath.Glob(filepath.Join(out, d, "init*.mp4"))
				assert.Len(t, inits, 1, "rendition %s needs exactly one CMAF init segment", d)
			}

			master, err := os.ReadFile(filepath.Join(out, abr.MasterPlaylistName))
			require.NoError(t, err)
			variants := strings.Count(string(master), "#EXT-X-STREAM-INF:")
			assert.Equal(t, len(res.Renditions), variants, "only video variants are advertised:\n%s", master)

			// Every rung must share segment boundaries for seamless switching.
			var first string
			for _, r := range res.Renditions {
				pl, err := os.ReadFile(filepath.Join(out, r.Name, "index.m3u8"))
				require.NoError(t, err)
				var durs []string
				for _, l := range strings.Split(string(pl), "\n") {
					if strings.HasPrefix(l, "#EXTINF:") {
						durs = append(durs, l)
					}
				}
				joined := strings.Join(durs, ",")
				if first == "" {
					first = joined
				}
				assert.Equal(t, first, joined, "rendition %s segment durations differ", r.Name)
			}
		})
	}
}
