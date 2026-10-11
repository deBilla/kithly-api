package abr_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kithly/kithly-ws-go/pkg/abr"
)

// flagValue returns the argument following the first occurrence of flag.
func flagValue(t *testing.T, args []string, flag string) string {
	t.Helper()
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	t.Fatalf("flag %q not found in %v", flag, args)
	return ""
}

// flagValues returns every argument following an occurrence of flag.
func flagValues(args []string, flag string) []string {
	var out []string
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			out = append(out, args[i+1])
		}
	}
	return out
}

func twoRungs() []abr.Rendition {
	return []abr.Rendition{
		{Name: "720p", ShortSide: 720, VideoBitrateK: 2800, MaxRateK: 3000, BufSizeK: 4200},
		{Name: "360p", ShortSide: 360, VideoBitrateK: 800, MaxRateK: 856, BufSizeK: 1200},
	}
}

func TestBuildArgs_PortraitWithAudio(t *testing.T) {
	src := abr.Source{Width: 720, Height: 1280, HasAudio: true}
	args := abr.BuildArgs("in.mp4", "/out", src, twoRungs(), abr.DefaultOptions())

	assert.Equal(t, "in.mp4", flagValue(t, args, "-i"))
	assert.Equal(t,
		"[0:v]split=2[s0][s1];[s0]scale=720:-2[v0];[s1]scale=360:-2[v1]",
		flagValue(t, args, "-filter_complex"))
	assert.Equal(t, []string{"[v0]", "[v1]", "0:a:0"}, flagValues(args, "-map"))

	assert.Equal(t, "2800k", flagValue(t, args, "-b:v:0"))
	assert.Equal(t, "3000k", flagValue(t, args, "-maxrate:v:0"))
	assert.Equal(t, "4200k", flagValue(t, args, "-bufsize:v:0"))
	assert.Equal(t, "800k", flagValue(t, args, "-b:v:1"))
	assert.Equal(t, "856k", flagValue(t, args, "-maxrate:v:1"))
	assert.Equal(t, "1200k", flagValue(t, args, "-bufsize:v:1"))

	assert.Equal(t, "aac", flagValue(t, args, "-c:a"))
	assert.Equal(t, "128k", flagValue(t, args, "-b:a"))

	assert.Equal(t,
		"a:0,agroup:audio,name:audio v:0,agroup:audio,name:720p v:1,agroup:audio,name:360p",
		flagValue(t, args, "-var_stream_map"))
	assert.Equal(t, filepath.Join("/out", "%v", "index.m3u8"), args[len(args)-1])
}

func TestBuildArgs_LandscapeScalesHeight(t *testing.T) {
	src := abr.Source{Width: 1280, Height: 720}
	args := abr.BuildArgs("in.mp4", "/out", src, twoRungs(), abr.DefaultOptions())
	assert.Equal(t,
		"[0:v]split=2[s0][s1];[s0]scale=-2:720[v0];[s1]scale=-2:360[v1]",
		flagValue(t, args, "-filter_complex"))
}

func TestBuildArgs_NoAudio(t *testing.T) {
	src := abr.Source{Width: 720, Height: 1280, HasAudio: false}
	args := abr.BuildArgs("in.mp4", "/out", src, twoRungs(), abr.DefaultOptions())

	assert.Equal(t, []string{"[v0]", "[v1]"}, flagValues(args, "-map"))
	assert.NotContains(t, args, "-c:a")
	assert.Equal(t, "v:0,name:720p v:1,name:360p", flagValue(t, args, "-var_stream_map"))
}

func TestBuildArgs_SingleRenditionSkipsSplit(t *testing.T) {
	src := abr.Source{Width: 640, Height: 360, HasAudio: true}
	rungs := twoRungs()[1:]
	args := abr.BuildArgs("in.mp4", "/out", src, rungs, abr.DefaultOptions())
	assert.Equal(t, "[0:v]scale=-2:360[v0]", flagValue(t, args, "-filter_complex"))
}

func TestBuildArgs_CMAFPackaging(t *testing.T) {
	opts := abr.DefaultOptions()
	opts.Preset = "veryfast"
	opts.SegmentSeconds = 6
	args := abr.BuildArgs("in.mp4", "/out", abr.Source{Width: 720, Height: 1280}, twoRungs(), opts)

	want := map[string]string{
		"-c:v":                    "libx264",
		"-preset":                 "veryfast",
		"-pix_fmt":                "yuv420p",
		"-sc_threshold":           "0",
		"-force_key_frames":       "expr:gte(t,n_forced*6)",
		"-f":                      "hls",
		"-hls_time":               "6",
		"-hls_playlist_type":      "vod",
		"-hls_segment_type":       "fmp4",
		"-hls_flags":              "independent_segments",
		"-hls_fmp4_init_filename": "init.mp4",
		"-master_pl_name":         abr.MasterPlaylistName,
		"-hls_segment_filename":   filepath.Join("/out", "%v", "seg_%03d.m4s"),
	}
	for flag, v := range want {
		assert.Equal(t, v, flagValue(t, args, flag), flag)
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := abr.DefaultOptions()
	require.Positive(t, opts.SegmentSeconds)
	require.Positive(t, opts.AudioBitrateK)
	require.NotEmpty(t, opts.Preset)
}
