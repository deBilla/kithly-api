package abr_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kithly/kithly-ws-go/pkg/abr"
)

type mockRunner struct {
	mock.Mock
}

func (m *mockRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	called := m.Called(ctx, name, args)
	out, _ := called.Get(0).([]byte)
	return out, called.Error(1)
}

const probeJSON = `{"streams":[{"codec_type":"video","width":720,"height":1280},{"codec_type":"audio"}]}`

func TestPackager_Package(t *testing.T) {
	ctx := context.Background()
	outDir := t.TempDir()
	r := &mockRunner{}

	r.On("Run", ctx, "ffprobe", mock.Anything).Return([]byte(probeJSON), nil)
	r.On("Run", ctx, "ffmpeg", mock.Anything).
		Run(func(args mock.Arguments) {
			// Simulate ffmpeg writing its master playlist.
			require.NoError(t, os.WriteFile(filepath.Join(outDir, abr.MasterPlaylistName), []byte(ffmpegMaster), 0o644))
		}).
		Return([]byte(nil), nil)

	p := abr.NewPackager(r, abr.DefaultLadder)
	res, err := p.Package(ctx, "in.mp4", outDir, abr.DefaultOptions())
	require.NoError(t, err)

	assert.Equal(t, []string{"720p", "480p", "360p"}, names(res.Renditions))
	assert.True(t, res.HasAudio)

	ffprobeArgs := r.Calls[0].Arguments.Get(2).([]string)
	assert.Equal(t, "in.mp4", ffprobeArgs[len(ffprobeArgs)-1])

	ffmpegArgs := r.Calls[1].Arguments.Get(2).([]string)
	assert.Equal(t, "in.mp4", flagValue(t, ffmpegArgs, "-i"))
	assert.Equal(t, filepath.Join(outDir, "%v", "index.m3u8"), ffmpegArgs[len(ffmpegArgs)-1])

	master, err := os.ReadFile(filepath.Join(outDir, abr.MasterPlaylistName))
	require.NoError(t, err)
	assert.NotContains(t, string(master), "\naudio/index.m3u8", "audio-only variant must be stripped")
	assert.Contains(t, string(master), `URI="audio/index.m3u8"`, "audio rendition group must remain")
	r.AssertExpectations(t)
}

func TestPackager_Errors(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")

	tests := []struct {
		name  string
		setup func(r *mockRunner, outDir string)
	}{
		{
			name: "ffprobe fails",
			setup: func(r *mockRunner, _ string) {
				r.On("Run", ctx, "ffprobe", mock.Anything).Return([]byte(nil), boom)
			},
		},
		{
			name: "probe has no video",
			setup: func(r *mockRunner, _ string) {
				r.On("Run", ctx, "ffprobe", mock.Anything).Return([]byte(`{"streams":[]}`), nil)
			},
		},
		{
			name: "ffmpeg fails",
			setup: func(r *mockRunner, _ string) {
				r.On("Run", ctx, "ffprobe", mock.Anything).Return([]byte(probeJSON), nil)
				r.On("Run", ctx, "ffmpeg", mock.Anything).Return([]byte(nil), boom)
			},
		},
		{
			name: "ffmpeg wrote no master playlist",
			setup: func(r *mockRunner, _ string) {
				r.On("Run", ctx, "ffprobe", mock.Anything).Return([]byte(probeJSON), nil)
				r.On("Run", ctx, "ffmpeg", mock.Anything).Return([]byte(nil), nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outDir := t.TempDir()
			r := &mockRunner{}
			tt.setup(r, outDir)
			_, err := abr.NewPackager(r, abr.DefaultLadder).Package(ctx, "in.mp4", outDir, abr.DefaultOptions())
			assert.Error(t, err)
		})
	}
}

func TestPackager_FfmpegErrorIsWrapped(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	r := &mockRunner{}
	r.On("Run", ctx, "ffprobe", mock.Anything).Return([]byte(probeJSON), nil)
	r.On("Run", ctx, "ffmpeg", mock.Anything).Return([]byte(nil), boom)

	_, err := abr.NewPackager(r, abr.DefaultLadder).Package(ctx, "in.mp4", t.TempDir(), abr.DefaultOptions())
	assert.ErrorIs(t, err, boom)
}
