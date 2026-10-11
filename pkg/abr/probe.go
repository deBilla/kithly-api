package abr

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
)

// ProbeArgs are the ffprobe arguments whose JSON output ParseProbe expects.
// The input path is appended by the caller.
var ProbeArgs = []string{"-v", "error", "-show_streams", "-of", "json"}

type probeOutput struct {
	Streams []struct {
		CodecType    string            `json:"codec_type"`
		Width        int               `json:"width"`
		Height       int               `json:"height"`
		Tags         map[string]string `json:"tags"`
		SideDataList []struct {
			Rotation float64 `json:"rotation"`
		} `json:"side_data_list"`
	} `json:"streams"`
}

// ParseProbe extracts the display geometry from ffprobe JSON. Phone footage is
// commonly stored as landscape pixels with a 90° rotation flag; ffmpeg
// autorotates on decode, so the reported width and height are swapped here to
// keep the ladder and scale filters in display orientation.
func ParseProbe(data []byte) (Source, error) {
	var p probeOutput
	if err := json.Unmarshal(data, &p); err != nil {
		return Source{}, fmt.Errorf("abr.ParseProbe: %w", err)
	}

	var src Source
	foundVideo := false
	for _, s := range p.Streams {
		switch s.CodecType {
		case "audio":
			src.HasAudio = true
		case "video":
			if foundVideo {
				continue
			}
			foundVideo = true
			src.Width, src.Height = s.Width, s.Height

			rotation := 0.0
			for _, sd := range s.SideDataList {
				if sd.Rotation != 0 {
					rotation = sd.Rotation
				}
			}
			if rotation == 0 {
				if v, err := strconv.ParseFloat(s.Tags["rotate"], 64); err == nil {
					rotation = v
				}
			}
			if int(math.Abs(rotation))%180 == 90 {
				src.Width, src.Height = src.Height, src.Width
			}
		}
	}

	if !foundVideo {
		return Source{}, errors.New("abr.ParseProbe: no video stream")
	}
	if src.Width <= 0 || src.Height <= 0 {
		return Source{}, errors.New("abr.ParseProbe: video stream has no dimensions")
	}
	return src, nil
}
