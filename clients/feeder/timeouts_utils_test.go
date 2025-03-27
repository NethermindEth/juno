package feeder

import (
	"context"
	"net/http"
	"net/http/httptest"
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
		want    TimeoutsList
		wantErr bool
	}{
		{
			name:  "empty input",
			input: []string{},
			want:  TimeoutsList{defaultTimeout},
		},
		{
			name:  "single value",
			input: []string{"5s"},
			want:  TimeoutsList{5 * time.Second},
		},
		{
			name:  "multiple values",
			input: []string{"5s", "7s", "10s"},
			want:  TimeoutsList{5 * time.Second, 7 * time.Second, 10 * time.Second},
		},
		{
			name:  "cap at max timeout",
			input: []string{"3m"},
			want:  TimeoutsList{maxTimeout},
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
	client := NewTestClient(t, &utils.Mainnet).WithMaxRetries(4).WithTimeouts(TimeoutsList{defaultTimeout})
	ctx := t.Context()

	t.Run("GET current timeouts", func(t *testing.T) {
		rr := setupTimeoutTest(t, ctx, http.MethodGet, "/feeder/timeouts", client)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "5s,8s,12s,18s,27s\n", rr.Body.String())
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
		assert.Equal(t, TimeoutsList{
			2 * time.Second,
			3 * time.Second,
			5 * time.Second,
			8 * time.Second,
			12 * time.Second,
		}, client.timeouts)
	})

	t.Run("PUT update timeouts list", func(t *testing.T) {
		rr := setupTimeoutTest(t, ctx, http.MethodPut, "/feeder/timeouts?timeouts=5s,7s,10s", client)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "Replaced timeouts with '5s,7s,10s' successfully\n", rr.Body.String())
		assert.Equal(t, TimeoutsList{
			5 * time.Second,
			7 * time.Second,
			10 * time.Second,
			15 * time.Second,
			23 * time.Second,
		}, client.timeouts)
	})

	t.Run("PUT update timeouts with invalid value", func(t *testing.T) {
		rr := setupTimeoutTest(t, ctx, http.MethodPut, "/feeder/timeouts?timeouts=invalid", client)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Equal(t, "parsing timeouts: time: invalid duration \"invalid\"\n", rr.Body.String())
	})

	t.Run("Method not allowed", func(t *testing.T) {
		rr := setupTimeoutTest(t, ctx, http.MethodPost, "/feeder/timeouts", client)

		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
		assert.Equal(t, "Method not allowed\n", rr.Body.String())
	})
}
