// Package abr turns a single MP4 into an adaptive-bitrate HLS ladder packaged
// as CMAF (fragmented MP4 segments). Players pick the rung that fits the
// viewer's bandwidth, so a contributor on a weak mobile connection still gets
// smooth playback while good connections get full quality.
//
// The package owns the encoding policy (ladder, GOP alignment, packaging
// flags) and is independent of storage and queueing so it can be unit tested
// without ffmpeg installed.
package abr

import (
	"errors"
	"fmt"
)

// Rendition is one rung of the ABR ladder. Rungs are expressed by their short
// side rather than height because Kithly content is mostly portrait (cards are
// rendered 1080×1920) while raw contributions can be either orientation.
type Rendition struct {
	Name          string
	ShortSide     int
	VideoBitrateK int
	MaxRateK      int
	BufSizeK      int
}

// DefaultLadder is ordered from highest to lowest quality; SelectRenditions
// relies on that order. Bitrates follow common H.264 30fps guidance, with
// maxrate ~107% and bufsize ~150% of target so segment sizes stay predictable
// for the player's bandwidth estimator.
var DefaultLadder = []Rendition{
	{Name: "1080p", ShortSide: 1080, VideoBitrateK: 5000, MaxRateK: 5350, BufSizeK: 7500},
	{Name: "720p", ShortSide: 720, VideoBitrateK: 2800, MaxRateK: 3000, BufSizeK: 4200},
	{Name: "480p", ShortSide: 480, VideoBitrateK: 1400, MaxRateK: 1500, BufSizeK: 2100},
	{Name: "360p", ShortSide: 360, VideoBitrateK: 800, MaxRateK: 856, BufSizeK: 1200},
}

// minShortSide guards against degenerate sources that x264 cannot encode.
const minShortSide = 16

// Source describes the display geometry of the input after rotation is
// applied, plus whether there is an audio track to package.
type Source struct {
	Width    int
	Height   int
	HasAudio bool
}

func (s Source) portrait() bool { return s.Width <= s.Height }

func (s Source) shortSide() int {
	if s.portrait() {
		return s.Width
	}
	return s.Height
}

// SelectRenditions returns the rungs of ladder that do not exceed the source
// resolution — upscaling only burns bandwidth without adding detail. A source
// smaller than every rung gets a single rendition at its own size using the
// lowest rung's bitrates, so tiny uploads still play.
func SelectRenditions(src Source, ladder []Rendition) ([]Rendition, error) {
	if src.Width <= 0 || src.Height <= 0 {
		return nil, fmt.Errorf("abr.SelectRenditions: invalid source size %dx%d", src.Width, src.Height)
	}
	if len(ladder) == 0 {
		return nil, errors.New("abr.SelectRenditions: empty ladder")
	}

	short := src.shortSide()
	var out []Rendition
	for _, r := range ladder {
		if r.ShortSide <= short {
			out = append(out, r)
		}
	}
	if len(out) > 0 {
		return out, nil
	}

	even := short &^ 1
	if even < minShortSide {
		return nil, fmt.Errorf("abr.SelectRenditions: source %dx%d too small", src.Width, src.Height)
	}
	r := ladder[len(ladder)-1]
	r.ShortSide = even
	r.Name = fmt.Sprintf("%dp", even)
	return []Rendition{r}, nil
}
