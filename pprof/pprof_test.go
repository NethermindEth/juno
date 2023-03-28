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

func TestPprofServerEnabled(t *testing.T) {
	port := uint16(9050)
	log := utils.NewNopZapLogger()
	url := fmt.Sprintf("http://localhost:%d/debug/pprof/", port)
	t.Run("create a new Pprof instance and run it", func(t *testing.T) {
		profiler := pprof.New(port, log)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)

		go func() {
			err := profiler.Run(ctx)
			require.NoError(t, err)
		}()

		waitForServerReady(t, url, time.Second)
	})
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
		require.NoError(t, err)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return
		}
		resp.Body.Close()

		require.Greaterf(t, timeout, start, "server is not ready after %v", timeout)

		time.Sleep(100 * time.Millisecond)
	}
}
