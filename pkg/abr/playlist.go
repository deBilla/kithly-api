package abr

import (
	"path/filepath"
	"strings"
)

// StripAudioOnlyVariants removes #EXT-X-STREAM-INF entries that have no
// RESOLUTION attribute. With demuxed audio, ffmpeg advertises the audio
// rendition as its own variant; hls.js treats it as the lowest-bandwidth level
// and will switch to it on a poor connection, leaving viewers with a black
// screen. The audio stays reachable through the #EXT-X-MEDIA group.
func StripAudioOnlyVariants(master string) string {
	lines := strings.SplitAfter(master, "\n")
	var b strings.Builder
	b.Grow(len(master))
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") && !strings.Contains(line, "RESOLUTION=") {
			i++ // Skip the variant URI that follows the tag.
			continue
		}
		b.WriteString(line)
	}
	return b.String()
}

// ContentType returns the MIME type S3 should serve for a packaged file.
// Correct types matter: Safari refuses playlists served as octet-stream and
// some CDNs only compress or cache by type.
func ContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".m4s":
		return "video/iso.segment"
	case ".mp4":
		return "video/mp4"
	case ".ts":
		return "video/MP2T"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	default:
		return "application/octet-stream"
	}
}
