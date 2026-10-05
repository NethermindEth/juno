package node

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/clients/timeout"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeHTTPUpdateService(t *testing.T) {
	tests := []struct {
		name        string
		clientCount int
		target      string
		wantCode    int
	}{
		{
			name:        "no clients: timeouts route is not registered",
			clientCount: 0,
			target:      "/feeder/timeouts",
			wantCode:    http.StatusNotFound,
		},
		{
			name:        "no clients: log level route is registered",
			clientCount: 0,
			target:      "/log/level",
			wantCode:    http.StatusOK,
		},
		{
			name:        "one client: timeouts route is registered",
			clientCount: 1,
			target:      "/feeder/timeouts",
			wantCode:    http.StatusOK,
		},
		{
			name:        "two clients: timeouts route is registered",
			clientCount: 2,
			target:      "/feeder/timeouts",
			wantCode:    http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clients := make([]timeout.Client, tt.clientCount)
			for i := range clients {
				clients[i] = feeder.NewClient(&url.URL{})
			}
			svc := makeHTTPUpdateService("localhost", 0, log.NewLevel(log.INFO), clients...)
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, tt.target, http.NoBody)
			require.NoError(t, err)
			rr := httptest.NewRecorder()

			svc.srv.Handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.wantCode, rr.Code)
		})
	}
}
