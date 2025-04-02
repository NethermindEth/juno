package feeder

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeoutString(t *testing.T) {
	tests := []*struct {
		name  string
		input Timeouts
		want  string
	}{
		{
			name: "empty timeouts",
			input: Timeouts{
				timeouts: []time.Duration{},
			},
			want: "",
		},
		{
			name: "single timeout",
			input: Timeouts{
				timeouts: []time.Duration{5 * time.Second},
			},
			want: "5s",
		},
		{
			name: "multiple timeouts",
			input: Timeouts{
				timeouts: []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second},
			},
			want: "5s,10s,20s",
		},
		{
			name: "mixed duration units",
			input: Timeouts{
				timeouts: []time.Duration{5 * time.Second, 2 * time.Minute, 1 * time.Hour},
			},
			want: "5s,2m0s,1h0m0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseTimeouts(t *testing.T) {
	type want struct {
		timeouts []time.Duration
		fixed    bool
	}

	tests := []struct {
		name    string
		input   string
		want    want
		wantErr bool
	}{
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:  "single value",
			input: "5s",
			want:  want{timeouts: []time.Duration{5 * time.Second}, fixed: false},
		},
		{
			name:    "single value with trailing comma",
			input:   "5s,",
			want:    want{timeouts: []time.Duration{5 * time.Second}, fixed: true},
			wantErr: false,
		},
		{
			name:  "multiple values",
			input: "5s,7s,10s",
			want:  want{timeouts: []time.Duration{5 * time.Second, 7 * time.Second, 10 * time.Second}, fixed: false},
		},
		{
			name:    "multiple values with trailing comma",
			input:   "5s,7s,10s,",
			want:    want{timeouts: []time.Duration{5 * time.Second, 7 * time.Second, 10 * time.Second}, fixed: false},
			wantErr: false,
		},
		{
			name:    "invalid duration",
			input:   "5s,invalid,10s",
			wantErr: true,
		},
		{
			name:    "empty timeouts",
			input:   "",
			wantErr: true,
		},
		{
			name:    "random order input",
			input:   "10s,5s,7s",
			wantErr: true,
		},
		{
			name:    "random order input with trailing comma",
			input:   "10s,5s,7s,",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, fixed, err := ParseTimeouts(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want.timeouts, got)
			assert.Equal(t, tt.want.fixed, fixed)
		})
	}
}

func TestGetDynamicTimeouts(t *testing.T) {
	input := 5 * time.Second
	want := Timeouts{
		curTimeout: 0,
		timeouts: []time.Duration{
			5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second, 80 * time.Second,
			120 * time.Second, 144 * time.Second, 173 * time.Second, 208 * time.Second, 250 * time.Second,
			300 * time.Second, 360 * time.Second, 432 * time.Second, 519 * time.Second, 623 * time.Second,
			748 * time.Second, 898 * time.Second, 1078 * time.Second, 1294 * time.Second, 1553 * time.Second,
			1864 * time.Second, 2237 * time.Second, 2685 * time.Second, 3222 * time.Second, 3867 * time.Second,
			4641 * time.Second, 5570 * time.Second, 6684 * time.Second, 8021 * time.Second, 9626 * time.Second,
		},
		mu: sync.RWMutex{},
	}

	got := getDynamicTimeouts(input)
	assert.Equal(t, want.curTimeout, got.curTimeout)
	assert.Equal(t, want.timeouts, got.timeouts)
}
