package abr

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Runner executes an external command and returns its stdout. It exists so
// the packaging flow can be tested without ffmpeg installed.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ExecRunner runs commands on the host. Stderr is folded into the returned
// error because that is where ffmpeg explains why it failed.
type ExecRunner struct{}

// Run implements Runner.
func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// Result describes a packaged ladder. MasterPlaylist is relative to the
// output directory.
type Result struct {
	MasterPlaylist string
	Renditions     []Rendition
	HasAudio       bool
}

// Packager produces a CMAF ABR ladder from a local MP4.
type Packager interface {
	Package(ctx context.Context, input, outDir string, opts Options) (*Result, error)
}

type packager struct {
	runner Runner
	ladder []Rendition
}

// NewPackager returns a Packager that encodes against ladder, which must be
// ordered from highest to lowest quality.
func NewPackager(runner Runner, ladder []Rendition) Packager {
	return &packager{runner: runner, ladder: ladder}
}

// Package probes input, encodes the renditions that fit its resolution and
// writes the playlists and segments under outDir.
func (p *packager) Package(ctx context.Context, input, outDir string, opts Options) (*Result, error) {
	probe, err := p.runner.Run(ctx, "ffprobe", append(append([]string(nil), ProbeArgs...), input)...)
	if err != nil {
		return nil, fmt.Errorf("abr.Package: probe: %w", err)
	}
	src, err := ParseProbe(probe)
	if err != nil {
		return nil, fmt.Errorf("abr.Package: %w", err)
	}
	rends, err := SelectRenditions(src, p.ladder)
	if err != nil {
		return nil, fmt.Errorf("abr.Package: %w", err)
	}

	if _, err := p.runner.Run(ctx, "ffmpeg", BuildArgs(input, outDir, src, rends, opts)...); err != nil {
		return nil, fmt.Errorf("abr.Package: encode: %w", err)
	}

	masterPath := filepath.Join(outDir, MasterPlaylistName)
	master, err := os.ReadFile(masterPath)
	if err != nil {
		return nil, fmt.Errorf("abr.Package: read master playlist: %w", err)
	}
	if err := os.WriteFile(masterPath, []byte(StripAudioOnlyVariants(string(master))), 0o644); err != nil {
		return nil, fmt.Errorf("abr.Package: rewrite master playlist: %w", err)
	}

	return &Result{MasterPlaylist: MasterPlaylistName, Renditions: rends, HasAudio: src.HasAudio}, nil
}
