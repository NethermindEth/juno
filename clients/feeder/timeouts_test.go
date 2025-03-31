package feeder

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTimeouts(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []time.Duration
		wantErr bool
	}{
		{
			name:  "empty input",
			input: []string{},
			want:  []time.Duration{defaultTimeout},
		},
		{
			name:  "single value",
			input: []string{"5s"},
			want:  []time.Duration{5 * time.Second},
		},
		{
			name:  "multiple values",
			input: []string{"5s", "7s", "10s"},
			want:  []time.Duration{5 * time.Second, 7 * time.Second, 10 * time.Second},
		},
		{
			name:    "invalid duration",
			input:   []string{"5s", "invalid", "10s"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimeouts(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetTimeouts(t *testing.T) {
	tests := []struct {
		name       string
		input      []time.Duration
		maxRetries int
		want       Timeouts
	}{
		{
			name:       "empty input",
			input:      []time.Duration{},
			maxRetries: 4,
			want: Timeouts{
				curTimeout: 0,
				timeouts:   []time.Duration{5 * time.Second, 8 * time.Second, 12 * time.Second, 18 * time.Second, 27 * time.Second},
			},
		},
		{
			name:       "single value input",
			input:      []time.Duration{5 * time.Second},
			maxRetries: 4,
			want: Timeouts{
				curTimeout: 0,
				timeouts:   []time.Duration{5 * time.Second, 8 * time.Second, 12 * time.Second, 18 * time.Second, 27 * time.Second},
			},
		},
		{
			name:       "multiple values input",
			input:      []time.Duration{5 * time.Second, 7 * time.Second, 10 * time.Second},
			maxRetries: 4,
			want: Timeouts{
				curTimeout: 0,
				timeouts:   []time.Duration{5 * time.Second, 7 * time.Second, 10 * time.Second, 15 * time.Second, 23 * time.Second},
			},
		},
		{
			name:       "random order input",
			input:      []time.Duration{10 * time.Second, 5 * time.Second, 7 * time.Second},
			maxRetries: 4,
			want: Timeouts{
				curTimeout: 0,
				timeouts:   []time.Duration{5 * time.Second, 7 * time.Second, 10 * time.Second, 15 * time.Second, 23 * time.Second},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTimeouts(tt.input, tt.maxRetries)
			reflect.DeepEqual(tt.want, got)
		})
	}
}
