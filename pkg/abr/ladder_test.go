package abr_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kithly/kithly-ws-go/pkg/abr"
)

func names(rs []abr.Rendition) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Name
	}
	return out
}

func TestSelectRenditions(t *testing.T) {
	tests := []struct {
		name      string
		src       abr.Source
		wantNames []string
		wantShort []int
	}{
		{"portrait 1080p final", abr.Source{Width: 1080, Height: 1920}, []string{"1080p", "720p", "480p", "360p"}, []int{1080, 720, 480, 360}},
		{"landscape 1080p", abr.Source{Width: 1920, Height: 1080}, []string{"1080p", "720p", "480p", "360p"}, []int{1080, 720, 480, 360}},
		{"portrait 720p preview", abr.Source{Width: 720, Height: 1280}, []string{"720p", "480p", "360p"}, []int{720, 480, 360}},
		{"between rungs never upscales", abr.Source{Width: 1000, Height: 1778}, []string{"720p", "480p", "360p"}, []int{720, 480, 360}},
		{"exactly lowest rung", abr.Source{Width: 640, Height: 360}, []string{"360p"}, []int{360}},
		{"below lowest rung keeps source size", abr.Source{Width: 320, Height: 240}, []string{"240p"}, []int{240}},
		{"odd source rounded down to even", abr.Source{Width: 321, Height: 241}, []string{"240p"}, []int{240}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := abr.SelectRenditions(tt.src, abr.DefaultLadder)
			require.NoError(t, err)
			assert.Equal(t, tt.wantNames, names(got))
			shorts := make([]int, len(got))
			for i, r := range got {
				shorts[i] = r.ShortSide
			}
			assert.Equal(t, tt.wantShort, shorts)
		})
	}
}

func TestSelectRenditions_BelowLadderUsesLowestBitrates(t *testing.T) {
	got, err := abr.SelectRenditions(abr.Source{Width: 320, Height: 240}, abr.DefaultLadder)
	require.NoError(t, err)
	require.Len(t, got, 1)
	lowest := abr.DefaultLadder[len(abr.DefaultLadder)-1]
	assert.Equal(t, lowest.VideoBitrateK, got[0].VideoBitrateK)
	assert.Equal(t, lowest.MaxRateK, got[0].MaxRateK)
	assert.Equal(t, lowest.BufSizeK, got[0].BufSizeK)
}

func TestSelectRenditions_Errors(t *testing.T) {
	tests := []struct {
		name   string
		src    abr.Source
		ladder []abr.Rendition
	}{
		{"zero width", abr.Source{Width: 0, Height: 1920}, abr.DefaultLadder},
		{"negative height", abr.Source{Width: 1080, Height: -1}, abr.DefaultLadder},
		{"empty ladder", abr.Source{Width: 1080, Height: 1920}, nil},
		{"too small to encode", abr.Source{Width: 1, Height: 1}, abr.DefaultLadder},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := abr.SelectRenditions(tt.src, tt.ladder)
			assert.Error(t, err)
		})
	}
}

func TestSelectRenditions_DoesNotMutateLadder(t *testing.T) {
	ladder := append([]abr.Rendition(nil), abr.DefaultLadder...)
	_, err := abr.SelectRenditions(abr.Source{Width: 320, Height: 240}, ladder)
	require.NoError(t, err)
	assert.Equal(t, abr.DefaultLadder, ladder)
}
