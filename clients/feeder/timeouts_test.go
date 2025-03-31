package feeder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/NethermindEth/juno/utils"
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

//nolint:dupl
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
				timeouts: []time.Duration{
					5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second, 80 * time.Second,
					120 * time.Second, 144 * time.Second, 173 * time.Second, 208 * time.Second, 250 * time.Second,
					300 * time.Second, 360 * time.Second, 432 * time.Second, 519 * time.Second, 623 * time.Second,
					748 * time.Second, 898 * time.Second, 1078 * time.Second, 1294 * time.Second, 1553 * time.Second,
					1864 * time.Second, 2237 * time.Second, 2685 * time.Second, 3222 * time.Second, 3867 * time.Second,
					4641 * time.Second, 5570 * time.Second, 6684 * time.Second, 8021 * time.Second, 9626 * time.Second,
				},
			},
		},
		{
			name:       "single value input",
			input:      []time.Duration{5 * time.Second},
			maxRetries: 4,
			want: Timeouts{
				curTimeout: 0,
				timeouts: []time.Duration{
					5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second, 80 * time.Second,
					120 * time.Second, 144 * time.Second, 173 * time.Second, 208 * time.Second, 250 * time.Second,
					300 * time.Second, 360 * time.Second, 432 * time.Second, 519 * time.Second, 623 * time.Second,
					748 * time.Second, 898 * time.Second, 1078 * time.Second, 1294 * time.Second, 1553 * time.Second,
					1864 * time.Second, 2237 * time.Second, 2685 * time.Second, 3222 * time.Second, 3867 * time.Second,
					4641 * time.Second, 5570 * time.Second, 6684 * time.Second, 8021 * time.Second, 9626 * time.Second,
					11552 * time.Second,
				},
			},
		},
		{
			name:       "multiple values input",
			input:      []time.Duration{5 * time.Second, 7 * time.Second, 10 * time.Second},
			maxRetries: 4,
			want: Timeouts{
				curTimeout: 0,
				timeouts: []time.Duration{
					5 * time.Second, 7 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second,
					80 * time.Second, 120 * time.Second, 144 * time.Second, 173 * time.Second, 208 * time.Second,
					250 * time.Second, 300 * time.Second, 360 * time.Second, 432 * time.Second, 519 * time.Second,
					623 * time.Second, 748 * time.Second, 898 * time.Second, 1078 * time.Second, 1294 * time.Second,
					1553 * time.Second, 1864 * time.Second, 2237 * time.Second, 2685 * time.Second, 3222 * time.Second,
					3867 * time.Second, 4641 * time.Second, 5570 * time.Second, 6684 * time.Second, 8021 * time.Second,
					9626 * time.Second,
				},
			},
		},
		{
			name:       "random order input",
			input:      []time.Duration{10 * time.Second, 5 * time.Second, 7 * time.Second},
			maxRetries: 4,
			want: Timeouts{
				curTimeout: 0,
				timeouts: []time.Duration{
					5 * time.Second, 7 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second,
					80 * time.Second, 120 * time.Second, 144 * time.Second, 173 * time.Second, 208 * time.Second,
					250 * time.Second, 300 * time.Second, 360 * time.Second, 432 * time.Second, 519 * time.Second,
					623 * time.Second, 748 * time.Second, 898 * time.Second, 1078 * time.Second, 1294 * time.Second,
					1553 * time.Second, 1864 * time.Second, 2237 * time.Second, 2685 * time.Second, 3222 * time.Second,
					3867 * time.Second, 4641 * time.Second, 5570 * time.Second, 6684 * time.Second, 8021 * time.Second,
					9626 * time.Second,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTimeouts(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
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

func TestHTTPTimeoutsSettings(t *testing.T) {
	client := NewTestClient(t, &utils.Mainnet).WithMaxRetries(4).WithTimeouts([]time.Duration{defaultTimeout})
	ctx := t.Context()

	t.Run("GET current timeouts", func(t *testing.T) {
		rr := setupTimeoutTest(t, ctx, http.MethodGet, "/feeder/timeouts", client)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "5s,10s,20s,40s,1m20s\n", rr.Body.String())
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
			2 * time.Second,
			4 * time.Second,
			8 * time.Second,
			16 * time.Second,
			32 * time.Second,
		}, client.timeouts.timeouts)
	})

	t.Run("PUT update timeouts list", func(t *testing.T) {
		rr := setupTimeoutTest(t, ctx, http.MethodPut, "/feeder/timeouts?timeouts=5s,7s,10s", client)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "Replaced timeouts with '5s,7s,10s' successfully\n", rr.Body.String())
		assert.Equal(t, []time.Duration{
			5 * time.Second,
			7 * time.Second,
			10 * time.Second,
			20 * time.Second,
			40 * time.Second,
		}, client.timeouts.timeouts)
	})

	t.Run("PUT update timeouts with invalid value", func(t *testing.T) {
		rr := setupTimeoutTest(t, ctx, http.MethodPut, "/feeder/timeouts?timeouts=invalid", client)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Equal(t, "parsing timeouts at index 0: time: invalid duration \"invalid\"\n", rr.Body.String())
	})

	t.Run("Method not allowed", func(t *testing.T) {
		rr := setupTimeoutTest(t, ctx, http.MethodPost, "/feeder/timeouts", client)

		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
		assert.Equal(t, "Method not allowed\n", rr.Body.String())
	})
}
