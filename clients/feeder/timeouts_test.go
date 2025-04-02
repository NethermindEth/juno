package feeder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/utils"
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
		{
			name:    "max amount of timeouts exceeded",
			input:   "1s,2s,3s,4s,5s,6s,7s,8s,9s,10s,11s,12s,13s,14s,15s,16s,17s,18s,19s,20s,21s,22s,23s,24s,25s,26s,27s,28s,29s,30s,31s",
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

//nolint:dupl
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

func setupTimeoutTest(t *testing.T, ctx context.Context, method, path string, client *Client) *httptest.ResponseRecorder {
	req, err := http.NewRequestWithContext(ctx, method, path, http.NoBody)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HTTPTimeoutsSettings(w, r, client)
	})

	handler.ServeHTTP(rr, req)
	return rr
}

//nolint:dupl
func TestHTTPTimeoutsSettings(t *testing.T) {
	timeouts, fixed, err := ParseTimeouts(DefaultTimeouts)
	require.NoError(t, err)
	client := NewTestClient(t, &utils.Mainnet).WithMaxRetries(4).WithTimeouts(timeouts, fixed)
	ctx := t.Context()

	t.Run("GET current timeouts", func(t *testing.T) {
		timeouts, fixed, err := ParseTimeouts(DefaultTimeouts)
		require.NoError(t, err)
		client := NewTestClient(t, &utils.Mainnet).WithMaxRetries(4).WithTimeouts(timeouts, fixed)
		ctx := t.Context()
		rr := setupTimeoutTest(t, ctx, http.MethodGet, "/feeder/timeouts", client)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "5s\n", rr.Body.String())
	})

	t.Run("GET current timeouts single value", func(t *testing.T) {
		timeouts, fixed, err := ParseTimeouts("5s")
		require.NoError(t, err)
		client := NewTestClient(t, &utils.Mainnet).WithMaxRetries(4).WithTimeouts(timeouts, fixed)
		ctx := t.Context()
		rr := setupTimeoutTest(t, ctx, http.MethodGet, "/feeder/timeouts", client)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "5s,10s,20s,40s,1m20s,2m0s,2m24s,2m53s,3m28s,4m10s,5m0s,6m0s,7m12s,8m39s,10m23s,12m28s,14m58s,17m58s,21m34s,25m53s,31m4s,37m17s,44m45s,53m42s,1h4m27s,1h17m21s,1h32m50s,1h51m24s,2h13m41s,2h40m26s\n", rr.Body.String())
	})

	t.Run("PUT update timeouts with missing parameter", func(t *testing.T) {
		rr := setupTimeoutTest(t, ctx, http.MethodPut, "/feeder/timeouts", client)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Equal(t, "missing timeouts query parameter\n", rr.Body.String())
	})

	t.Run("PUT update single value timeout", func(t *testing.T) {
		rr := setupTimeoutTest(t, ctx, http.MethodPut, "/feeder/timeouts?timeouts=2s", client)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "Replaced timeouts with '2s' successfully\n", rr.Body.String())
		assert.Equal(t, []time.Duration{
			2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 32 * time.Second,
			64 * time.Second, 96 * time.Second, 144 * time.Second, 173 * time.Second, 208 * time.Second,
			250 * time.Second, 300 * time.Second, 360 * time.Second, 432 * time.Second, 519 * time.Second,
			623 * time.Second, 748 * time.Second, 898 * time.Second, 1078 * time.Second, 1294 * time.Second,
			1553 * time.Second, 1864 * time.Second, 2237 * time.Second, 2685 * time.Second, 3222 * time.Second,
			3867 * time.Second, 4641 * time.Second, 5570 * time.Second, 6684 * time.Second, 8021 * time.Second,
		}, client.timeouts.timeouts)
	})

	t.Run("PUT update single value timeout with trailing comma", func(t *testing.T) {
		rr := setupTimeoutTest(t, ctx, http.MethodPut, "/feeder/timeouts?timeouts=2s,", client)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "Replaced timeouts with '2s,' successfully\n", rr.Body.String())
		assert.Equal(t, []time.Duration{
			2 * time.Second,
		}, client.timeouts.timeouts)
	})

	t.Run("PUT update timeouts list", func(t *testing.T) {
		rr := setupTimeoutTest(t, ctx, http.MethodPut, "/feeder/timeouts?timeouts=5s,7s,10s", client)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "Replaced timeouts with '5s,7s,10s' successfully\n", rr.Body.String())
		assert.Equal(t, []time.Duration{
			5 * time.Second, 7 * time.Second, 10 * time.Second,
		}, client.timeouts.timeouts)
	})

	t.Run("PUT update timeouts with invalid value", func(t *testing.T) {
		rr := setupTimeoutTest(t, ctx, http.MethodPut, "/feeder/timeouts?timeouts=invalid", client)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Equal(t, "parsing timeouts at index 0: time: invalid duration \"invalid\"\n", rr.Body.String())
	})

	t.Run("PUT update timeouts with invalid order", func(t *testing.T) {
		rr := setupTimeoutTest(t, ctx, http.MethodPut, "/feeder/timeouts?timeouts=10s,5s,7s", client)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Equal(t, "timeouts must be in ascending order, got 5s <= 10s\n", rr.Body.String())
	})

	t.Run("Method not allowed", func(t *testing.T) {
		rr := setupTimeoutTest(t, ctx, http.MethodPost, "/feeder/timeouts", client)

		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
		assert.Equal(t, "Method not allowed\n", rr.Body.String())
	})
}
