package abr_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kithly/kithly-ws-go/pkg/abr"
)

func TestParseProbe(t *testing.T) {
	tests := []struct {
		name string
		json string
		want abr.Source
	}{
		{
			name: "video and audio",
			json: `{"streams":[{"codec_type":"video","width":1080,"height":1920},{"codec_type":"audio"}]}`,
			want: abr.Source{Width: 1080, Height: 1920, HasAudio: true},
		},
		{
			name: "video only",
			json: `{"streams":[{"codec_type":"video","width":1920,"height":1080}]}`,
			want: abr.Source{Width: 1920, Height: 1080, HasAudio: false},
		},
		{
			// Phones store portrait video as landscape pixels plus a rotation
			// side-data entry; ffmpeg autorotates, so the ladder must too.
			name: "rotation side data swaps dimensions",
			json: `{"streams":[{"codec_type":"video","width":1920,"height":1080,"side_data_list":[{"side_data_type":"Display Matrix","rotation":-90}]}]}`,
			want: abr.Source{Width: 1080, Height: 1920},
		},
		{
			name: "legacy rotate tag swaps dimensions",
			json: `{"streams":[{"codec_type":"video","width":1920,"height":1080,"tags":{"rotate":"270"}}]}`,
			want: abr.Source{Width: 1080, Height: 1920},
		},
		{
			name: "180 rotation keeps dimensions",
			json: `{"streams":[{"codec_type":"video","width":1920,"height":1080,"side_data_list":[{"rotation":180}]}]}`,
			want: abr.Source{Width: 1920, Height: 1080},
		},
		{
			name: "first video stream wins",
			json: `{"streams":[{"codec_type":"video","width":720,"height":1280},{"codec_type":"video","width":64,"height":64}]}`,
			want: abr.Source{Width: 720, Height: 1280},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := abr.ParseProbe([]byte(tt.json))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseProbe_Errors(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"invalid json", `{`},
		{"no video stream", `{"streams":[{"codec_type":"audio"}]}`},
		{"video without dimensions", `{"streams":[{"codec_type":"video"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := abr.ParseProbe([]byte(tt.json))
			assert.Error(t, err)
		})
	}
}
