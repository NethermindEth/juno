package main

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDBMaxHandlesForFDLimit(t *testing.T) {
	tests := []struct {
		name    string
		fdLimit uint64
		want    int
	}{
		{name: "zero limit clamps to floor", fdLimit: 0, want: defaultDBMaxHandlesFloor},
		{name: "half below floor clamps to floor", fdLimit: 2047, want: defaultDBMaxHandlesFloor},
		{name: "half equals floor", fdLimit: 2048, want: defaultDBMaxHandlesFloor},
		{name: "half above floor", fdLimit: 4096, want: 2048},
		{name: "large limit", fdLimit: 1048576, want: 524288},
		{
			name:    "half equals ceiling",
			fdLimit: 2 * defaultDBMaxHandlesCeiling,
			want:    defaultDBMaxHandlesCeiling,
		},
		{
			name:    "half above ceiling",
			fdLimit: 4 * defaultDBMaxHandlesCeiling,
			want:    defaultDBMaxHandlesCeiling,
		},
		{
			name:    "runtime-raised unlimited rlimit",
			fdLimit: math.MaxUint64 - 1,
			want:    defaultDBMaxHandlesCeiling,
		},
		{name: "unlimited rlimit", fdLimit: math.MaxUint64, want: defaultDBMaxHandlesCeiling},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, dbMaxHandlesForFDLimit(tt.fdLimit))
		})
	}
}
