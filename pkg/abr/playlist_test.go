package abr_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kithly/kithly-ws-go/pkg/abr"
)

// ffmpegMaster is what ffmpeg 8 actually emits for a demuxed-audio CMAF ladder.
const ffmpegMaster = `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="group_audio",NAME="audio_0",DEFAULT=YES,CHANNELS="2",URI="audio/index.m3u8"
#EXT-X-STREAM-INF:BANDWIDTH=121304,AVERAGE-BANDWIDTH=104318,CODECS="mp4a.40.2",AUDIO="group_audio"
audio/index.m3u8

#EXT-X-STREAM-INF:BANDWIDTH=2838892,AVERAGE-BANDWIDTH=2773405,RESOLUTION=720x1280,CODECS="avc1.64001f,mp4a.40.2",AUDIO="group_audio"
720p/index.m3u8

#EXT-X-STREAM-INF:BANDWIDTH=832278,AVERAGE-BANDWIDTH=786952,RESOLUTION=360x640,CODECS="avc1.64001e,mp4a.40.2",AUDIO="group_audio"
360p/index.m3u8
`

func TestStripAudioOnlyVariants(t *testing.T) {
	want := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="group_audio",NAME="audio_0",DEFAULT=YES,CHANNELS="2",URI="audio/index.m3u8"

#EXT-X-STREAM-INF:BANDWIDTH=2838892,AVERAGE-BANDWIDTH=2773405,RESOLUTION=720x1280,CODECS="avc1.64001f,mp4a.40.2",AUDIO="group_audio"
720p/index.m3u8

#EXT-X-STREAM-INF:BANDWIDTH=832278,AVERAGE-BANDWIDTH=786952,RESOLUTION=360x640,CODECS="avc1.64001e,mp4a.40.2",AUDIO="group_audio"
360p/index.m3u8
`
	assert.Equal(t, want, abr.StripAudioOnlyVariants(ffmpegMaster))
}

func TestStripAudioOnlyVariants_NoAudioIsUnchanged(t *testing.T) {
	in := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-STREAM-INF:BANDWIDTH=832278,RESOLUTION=360x640,CODECS="avc1.64001e"
360p/index.m3u8
`
	assert.Equal(t, in, abr.StripAudioOnlyVariants(in))
}

func TestStripAudioOnlyVariants_CRLF(t *testing.T) {
	in := "#EXTM3U\r\n#EXT-X-STREAM-INF:BANDWIDTH=1,CODECS=\"mp4a.40.2\"\r\naudio/index.m3u8\r\n#EXT-X-STREAM-INF:BANDWIDTH=2,RESOLUTION=360x640\r\n360p/index.m3u8\r\n"
	want := "#EXTM3U\r\n#EXT-X-STREAM-INF:BANDWIDTH=2,RESOLUTION=360x640\r\n360p/index.m3u8\r\n"
	assert.Equal(t, want, abr.StripAudioOnlyVariants(in))
}

func TestContentType(t *testing.T) {
	tests := []struct {
		file string
		want string
	}{
		{"master.m3u8", "application/vnd.apple.mpegurl"},
		{"720p/index.m3u8", "application/vnd.apple.mpegurl"},
		{"720p/seg_000.m4s", "video/iso.segment"},
		{"720p/init_1.mp4", "video/mp4"},
		{"legacy/segment_000.ts", "video/MP2T"},
		{"thumbnail.jpg", "image/jpeg"},
		{"unknown.bin", "application/octet-stream"},
		{"UPPER.M4S", "video/iso.segment"},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			assert.Equal(t, tt.want, abr.ContentType(tt.file))
		})
	}
}
