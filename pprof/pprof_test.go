package pprof_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/NethermindEth/juno/pprof"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

func TestPprofServer(t *testing.T) {
	port := uint16(9050)
	log := utils.NewNopZapLogger()
	url := fmt.Sprintf("http://localhost:%d/debug/pprof/", port)
	profiler := pprof.New(port, log)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	go func() {
		err := profiler.Run(ctx)
		require.NoError(t, err)
	}()

	waitForServerReady(t, url, 5*time.Second)
}

func waitForServerReady(t *testing.T, url string, timeout time.Duration) {
	t.Helper()

	client := http.Client{}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	start := time.Now()

	for {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
		require.NoError(t, reqErr)

		resp, err := client.Do(req)
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		if err == nil && resp.StatusCode == http.StatusOK {
			return
		}

		elapsed := time.Since(start)
		require.Greaterf(t, timeout, elapsed, "server is not ready after %v", timeout)

		time.Sleep(100 * time.Millisecond)
	}
}
