package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestImageExtFromFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, b []byte) string {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, b, 0o600))
		return p
	}
	assert.Equal(t, ".jpg", imageExtFromFile(write("a", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0})))
	assert.Equal(t, ".png", imageExtFromFile(write("b", []byte{0x89, 'P', 'N', 'G', '\r', '\n'})))
	assert.Equal(t, ".gif", imageExtFromFile(write("c", []byte("GIF89a"))))
	assert.Equal(t, ".webp", imageExtFromFile(write("d", []byte("RIFF\x00\x00\x00\x00WEBPVP8 "))))
	assert.Equal(t, "", imageExtFromFile(write("e", []byte("\x00\x00\x00\x18ftypmp42")))) // MP4
	assert.Equal(t, "", imageExtFromFile(write("f", nil)))
	assert.Equal(t, "", imageExtFromFile(filepath.Join(dir, "missing")))
}

// TestCompile_ExtensionlessImageContribution reproduces production: uploads
// are stored at keys without an extension, so a text card JPEG was downloaded
// as input_N.mp4, treated as a one-frame video and dropped from the output.
func TestCompile_ExtensionlessImageContribution(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	cover := filepath.Join(dir, "cover.jpg")
	mustFFmpeg(t, "-f", "lavfi", "-i", "color=c=red:s=1080x1920", "-frames:v", "1", cover)
	// A blue text card, saved the way the worker saves an extensionless key.
	card := filepath.Join(dir, "input_1.mp4")
	mustFFmpeg(t, "-f", "lavfi", "-i", "color=c=blue:s=1080x1920", "-frames:v", "1", "-f", "mjpeg", card)

	const key = "contributions/card/abc/original_def"
	path, isImg, err := resolveStillImage(key, card)
	require.NoError(t, err)
	require.True(t, isImg)
	assert.Equal(t, filepath.Join(dir, "input_1.jpg"), path)

	p := &videoProcessor{logger: zap.NewNop()}
	out := filepath.Join(dir, "out.mp4")
	require.NoError(t, p.compile([]compInput{
		{path: cover, isImage: true, duration: coverDurationSeconds},
		{path: path, isImage: isImg, duration: 3},
	}, "", out, true))

	assert.InDelta(t, coverDurationSeconds+3, probeDuration(t, out), 0.3)
	assert.Equal(t, "red", dominantColour(t, out, 1.0))
	assert.Equal(t, "blue", dominantColour(t, out, coverDurationSeconds+1.5))
}

func TestResolveStillImage_VideoUntouched(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	clip := filepath.Join(dir, "input_0.mp4")
	mustFFmpeg(t, "-f", "lavfi", "-i", "color=c=blue:s=320x240:d=1", "-c:v", "libx264", "-pix_fmt", "yuv420p", clip)
	path, isImg, err := resolveStillImage("contributions/card/abc/original_def", clip)
	require.NoError(t, err)
	assert.False(t, isImg)
	assert.Equal(t, clip, path)
}
