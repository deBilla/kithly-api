package abr

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// MasterPlaylistName is the file players should be pointed at; each rendition
// gets its own sub-directory beside it.
const MasterPlaylistName = "master.m3u8"

// Options are the per-job encoding knobs. Everything else is fixed policy.
type Options struct {
	// Preset is the x264 speed/quality trade-off. Previews favour speed,
	// final compilations favour quality.
	Preset string
	// SegmentSeconds is also the forced keyframe interval, so every rendition
	// cuts at identical timestamps and players can switch rungs at any
	// segment boundary.
	SegmentSeconds int
	AudioBitrateK  int
}

// DefaultOptions returns settings suitable for final card compilations.
func DefaultOptions() Options {
	return Options{Preset: "medium", SegmentSeconds: 4, AudioBitrateK: 128}
}

// BuildArgs returns the ffmpeg arguments that encode src into every rendition
// in one decode pass and package them as CMAF HLS under outDir. Audio, when
// present, is packaged once as a separate rendition group shared by all video
// rungs instead of being duplicated into each one.
func BuildArgs(input, outDir string, src Source, rends []Rendition, opts Options) []string {
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-i", input}

	var filter strings.Builder
	if len(rends) > 1 {
		filter.WriteString(fmt.Sprintf("[0:v]split=%d", len(rends)))
		for i := range rends {
			filter.WriteString(fmt.Sprintf("[s%d]", i))
		}
		filter.WriteString(";")
	}
	for i, r := range rends {
		in := fmt.Sprintf("[s%d]", i)
		if len(rends) == 1 {
			in = "[0:v]"
		}
		scale := fmt.Sprintf("scale=-2:%d", r.ShortSide)
		if src.portrait() {
			scale = fmt.Sprintf("scale=%d:-2", r.ShortSide)
		}
		if i > 0 {
			filter.WriteString(";")
		}
		filter.WriteString(fmt.Sprintf("%s%s[v%d]", in, scale, i))
	}
	args = append(args, "-filter_complex", filter.String())

	for i := range rends {
		args = append(args, "-map", fmt.Sprintf("[v%d]", i))
	}
	if src.HasAudio {
		args = append(args, "-map", "0:a:0")
	}

	seg := strconv.Itoa(opts.SegmentSeconds)
	args = append(args,
		"-c:v", "libx264",
		"-preset", opts.Preset,
		"-profile:v", "high",
		"-pix_fmt", "yuv420p",
		// Scene-cut keyframes would make segment boundaries differ per rung.
		"-sc_threshold", "0",
		"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%s)", seg),
	)
	for i, r := range rends {
		args = append(args,
			fmt.Sprintf("-b:v:%d", i), fmt.Sprintf("%dk", r.VideoBitrateK),
			fmt.Sprintf("-maxrate:v:%d", i), fmt.Sprintf("%dk", r.MaxRateK),
			fmt.Sprintf("-bufsize:v:%d", i), fmt.Sprintf("%dk", r.BufSizeK),
		)
	}
	if src.HasAudio {
		args = append(args,
			"-c:a", "aac",
			"-b:a", fmt.Sprintf("%dk", opts.AudioBitrateK),
			"-ac", "2",
			"-ar", "48000",
		)
	}

	var streamMap []string
	if src.HasAudio {
		streamMap = append(streamMap, "a:0,agroup:audio,name:audio")
	}
	for i, r := range rends {
		entry := fmt.Sprintf("v:%d,name:%s", i, r.Name)
		if src.HasAudio {
			entry = fmt.Sprintf("v:%d,agroup:audio,name:%s", i, r.Name)
		}
		streamMap = append(streamMap, entry)
	}

	args = append(args,
		"-f", "hls",
		"-hls_time", seg,
		"-hls_playlist_type", "vod",
		"-hls_segment_type", "fmp4",
		"-hls_flags", "independent_segments",
		"-hls_fmp4_init_filename", "init.mp4",
		"-master_pl_name", MasterPlaylistName,
		"-hls_segment_filename", filepath.Join(outDir, "%v", "seg_%03d.m4s"),
		"-var_stream_map", strings.Join(streamMap, " "),
		filepath.Join(outDir, "%v", "index.m3u8"),
	)
	return args
}
