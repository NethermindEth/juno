package feeder

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/NethermindEth/juno/clients/timeout"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTimeoutClients(t *testing.T, initialTimeouts []string) []timeout.Client {
	t.Helper()
	clients := make([]timeout.Client, len(initialTimeouts))
	for i, initial := range initialTimeouts {
		durations, fixed, err := timeout.ParseTimeouts(initial)
		require.NoError(t, err)
		clients[i] = NewClient(&url.URL{}, WithTimeouts(durations, fixed))
	}
	return clients
}

func TestHTTPTimeoutsSettings(t *testing.T) {
	const (
		fiveSecondTimeouts = "5s,10s,20s,40s,1m20s,2m0s,2m24s,2m53s,3m28s,4m10s,5m0s," +
			"6m0s,7m12s,8m39s,10m23s,12m28s,14m58s,17m58s,21m34s,25m53s,31m4s,37m17s,44m45s," +
			"53m42s,1h4m27s,1h17m21s,1h32m50s,1h51m24s,2h13m41s,2h40m26s"
		twoSecondTimeouts = "2s,4s,8s,16s,32s,1m4s,1m36s,2m24s,2m53s,3m28s,4m10s,5m0s," +
			"6m0s,7m12s,8m39s,10m23s,12m28s,14m58s,17m58s,21m34s,25m53s,31m4s,37m17s,44m45s," +
			"53m42s,1h4m27s,1h17m21s,1h32m50s,1h51m24s,2h13m41s"
		fixedTimeouts = "7s,9s"
	)

	tests := []struct {
		name            string
		method          string
		target          string
		initialTimeouts []string
		wantCode        int
		wantBody        string
		wantTimeouts    []string
	}{
		{
			name:            "GET expands a single value",
			method:          http.MethodGet,
			target:          "/feeder/timeouts",
			initialTimeouts: []string{timeout.DefaultTimeouts},
			wantCode:        http.StatusOK,
			wantBody:        fiveSecondTimeouts + "\n",
			wantTimeouts:    []string{fiveSecondTimeouts},
		},
		{
			name:            "GET keeps a fixed list",
			method:          http.MethodGet,
			target:          "/feeder/timeouts",
			initialTimeouts: []string{fixedTimeouts},
			wantCode:        http.StatusOK,
			wantBody:        fixedTimeouts + "\n",
			wantTimeouts:    []string{fixedTimeouts},
		},
		{
			name:            "GET reports the first client",
			method:          http.MethodGet,
			target:          "/feeder/timeouts",
			initialTimeouts: []string{timeout.DefaultTimeouts, fixedTimeouts},
			wantCode:        http.StatusOK,
			wantBody:        fiveSecondTimeouts + "\n",
			wantTimeouts:    []string{fiveSecondTimeouts, fixedTimeouts},
		},
		{
			name:            "PUT without timeouts parameter leaves every client untouched",
			method:          http.MethodPut,
			target:          "/feeder/timeouts",
			initialTimeouts: []string{timeout.DefaultTimeouts, fixedTimeouts},
			wantCode:        http.StatusBadRequest,
			wantBody:        "missing timeouts query parameter\n",
			wantTimeouts:    []string{fiveSecondTimeouts, fixedTimeouts},
		},
		{
			name:            "PUT single value updates every client",
			method:          http.MethodPut,
			target:          "/feeder/timeouts?timeouts=2s",
			initialTimeouts: []string{timeout.DefaultTimeouts, fixedTimeouts},
			wantCode:        http.StatusOK,
			wantBody:        "Replaced timeouts with '2s' successfully\n",
			wantTimeouts:    []string{twoSecondTimeouts, twoSecondTimeouts},
		},
		{
			name:            "PUT single value with trailing comma fixes every client",
			method:          http.MethodPut,
			target:          "/feeder/timeouts?timeouts=2s,",
			initialTimeouts: []string{timeout.DefaultTimeouts, fixedTimeouts},
			wantCode:        http.StatusOK,
			wantBody:        "Replaced timeouts with '2s,' successfully\n",
			wantTimeouts:    []string{"2s", "2s"},
		},
		{
			name:            "PUT list updates every client",
			method:          http.MethodPut,
			target:          "/feeder/timeouts?timeouts=5s,7s,10s",
			initialTimeouts: []string{timeout.DefaultTimeouts, fixedTimeouts},
			wantCode:        http.StatusOK,
			wantBody:        "Replaced timeouts with '5s,7s,10s' successfully\n",
			wantTimeouts:    []string{"5s,7s,10s", "5s,7s,10s"},
		},
		{
			name:            "PUT invalid value leaves every client untouched",
			method:          http.MethodPut,
			target:          "/feeder/timeouts?timeouts=invalid",
			initialTimeouts: []string{timeout.DefaultTimeouts, fixedTimeouts},
			wantCode:        http.StatusBadRequest,
			wantBody:        "parsing timeout parameter number 1: time: invalid duration \"invalid\"\n",
			wantTimeouts:    []string{fiveSecondTimeouts, fixedTimeouts},
		},
		{
			name:            "PUT unordered values leaves every client untouched",
			method:          http.MethodPut,
			target:          "/feeder/timeouts?timeouts=10s,5s,7s",
			initialTimeouts: []string{timeout.DefaultTimeouts, fixedTimeouts},
			wantCode:        http.StatusBadRequest,
			wantBody:        "timeout values must be in ascending order, got 5s <= 10s\n",
			wantTimeouts:    []string{fiveSecondTimeouts, fixedTimeouts},
		},
		{
			name:            "POST is not allowed",
			method:          http.MethodPost,
			target:          "/feeder/timeouts",
			initialTimeouts: []string{timeout.DefaultTimeouts, fixedTimeouts},
			wantCode:        http.StatusMethodNotAllowed,
			wantBody:        "Method not allowed\n",
			wantTimeouts:    []string{fiveSecondTimeouts, fixedTimeouts},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clients := newTimeoutClients(t, tt.initialTimeouts)
			req, err := http.NewRequestWithContext(t.Context(), tt.method, tt.target, http.NoBody)
			require.NoError(t, err)
			rr := httptest.NewRecorder()

			timeout.HTTPTimeoutsSettings(rr, req, clients...)

			assert.Equal(t, tt.wantCode, rr.Code)
			assert.Equal(t, tt.wantBody, rr.Body.String())
			for i, client := range clients {
				assert.Equal(t, tt.wantTimeouts[i], client.Timeouts(), "client %d", i)
			}
		})
	}
}

// One successful call is enough to catch the old tryGet, which assigned
// Client.Timeout on the shared http.Client before Do.
func TestHTTPClientTimeoutNotMutated(t *testing.T) {
	const blockBody = `{"block_hash": "0x123", "block_number": 1}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(blockBody))
	}))
	t.Cleanup(srv.Close)

	serverURL, err := url.Parse(srv.URL)
	require.NoError(t, err)

	t.Run("default client", func(t *testing.T) {
		before := http.DefaultClient.Timeout
		client := NewClient(serverURL, WithMaxRetries(0), WithBackoff(NopBackoff))
		require.NotSame(t, http.DefaultClient, client.client)
		require.Equal(t, before, client.client.Timeout)

		block, err := client.Block(t.Context(), "1")
		require.NoError(t, err)
		require.Equal(t, uint64(1), block.Number)
		require.Equal(t, before, http.DefaultClient.Timeout)
		require.Equal(t, before, client.client.Timeout)
	})

	t.Run("caller client", func(t *testing.T) {
		const suppliedTimeout = 3 * time.Second
		supplied := &http.Client{Timeout: suppliedTimeout}
		client := NewClient(
			serverURL,
			WithHTTPClient(supplied),
			WithMaxRetries(0),
			WithBackoff(NopBackoff),
		)
		require.NotSame(t, supplied, client.client)
		require.Equal(t, suppliedTimeout, client.client.Timeout)

		block, err := client.Block(t.Context(), "1")
		require.NoError(t, err)
		require.Equal(t, uint64(1), block.Number)
		require.Equal(t, suppliedTimeout, supplied.Timeout)
		require.Equal(t, suppliedTimeout, client.client.Timeout)
	})
}
