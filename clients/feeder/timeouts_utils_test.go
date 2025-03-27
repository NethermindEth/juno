package feeder

import (
	"fmt"
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
		want    TimeoutConfig
		wantErr bool
	}{
		{
			name:  "empty input",
			input: []string{},
			want:  TimeoutConfig{defaultTimeout},
		},
		{
			name:  "single value",
			input: []string{"5s"},
			want:  TimeoutConfig{5 * time.Second},
		},
		{
			name:  "multiple values",
			input: []string{"5s", "7s", "10s"},
			want:  TimeoutConfig{5 * time.Second, 7 * time.Second, 10 * time.Second},
		},
		{
			name:  "cap at max timeout",
			input: []string{"3m"},
			want:  TimeoutConfig{maxTimeout},
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

func TestHTTPTimeoutsSettings(t *testing.T) {
	client := NewTestClient(t, &utils.Mainnet).WithMaxRetries(4).WithTimeouts(TimeoutConfig{defaultTimeout})
	ctx := t.Context()
	t.Run("GET current timeouts", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/feeder/timeouts", http.NoBody)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			HTTPTimeoutsSettings(w, r, client)
		})

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "5s,8s,12s,18s,27s\n", rr.Body.String())
	})

	t.Run("PUT update timeouts list and single timeout", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, "/feeder/timeouts?timeouts=5s,7s,10s", http.NoBody)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			HTTPTimeoutsSettings(w, r, client)
		})

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "Replaced timeouts with '5s,7s,10s' successfully\n", rr.Body.String())
		assert.Equal(t, TimeoutConfig{
			5 * time.Second,
			7 * time.Second,
			10 * time.Second,
			15 * time.Second,
			23 * time.Second,
		}, client.timeouts)

		rr = httptest.NewRecorder()

		req, err = http.NewRequestWithContext(ctx, http.MethodPut, "/feeder/timeouts?timeouts=2s", http.NoBody)
		require.NoError(t, err)

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "Replaced timeouts with '2s' successfully\n", rr.Body.String())
		assert.Equal(t, TimeoutConfig{2 * time.Second,
			3 * time.Second,
			5 * time.Second,
			8 * time.Second,
			12 * time.Second,
		}, client.timeouts)
	})

	t.Run("PUT update timeouts with missing parameter", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, "/feeder/timeouts", http.NoBody)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			HTTPTimeoutsSettings(w, r, client)
		})

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Equal(t, "missing timeouts query parameter\n", rr.Body.String())
	})

	t.Run("PUT update timeouts with invalid value", func(t *testing.T) {
		invalidValue := "invalid"
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, "/feeder/timeouts?timeouts="+invalidValue, http.NoBody)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			HTTPTimeoutsSettings(w, r, client)
		})

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Equal(t, rr.Body.String(), fmt.Sprintf(`parsing timeouts: time: invalid duration "%s"`, invalidValue)+"\n")
	})

	t.Run("Method not allowed", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/feeder/timeouts", http.NoBody)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			HTTPTimeoutsSettings(w, r, client)
		})

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
		assert.Equal(t, "Method not allowed\n", rr.Body.String())
	})
}
