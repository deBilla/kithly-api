package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kithly/kithly-ws-go/internal/modules/transcoderclient"
)

func TestEncodeOptions(t *testing.T) {
	tests := []struct {
		jobType    string
		wantPreset string
	}{
		{transcoderclient.JobTypeCardPreview, "veryfast"},
		{transcoderclient.JobTypeContributionIngest, "fast"},
		{transcoderclient.JobTypeCardCompilation, "medium"},
		{"unknown", "medium"},
	}
	for _, tt := range tests {
		t.Run(tt.jobType, func(t *testing.T) {
			opts := encodeOptions(tt.jobType)
			assert.Equal(t, tt.wantPreset, opts.Preset)
			assert.Positive(t, opts.SegmentSeconds)
		})
	}
}
