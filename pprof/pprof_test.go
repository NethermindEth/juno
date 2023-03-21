package pprof_test

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
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
		profiler := pprof.New(true, port, log)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		go func() {
			err := profiler.Run(ctx)
			require.NoError(t, err)
		}()

		err := waitForServerReady(url, time.Second)
		require.NoError(t, err)
	})
}

func TestPprofServerDisabled(t *testing.T) {
	port := uint16(5555)
	log := utils.NewNopZapLogger()
	url := fmt.Sprintf("http://localhost:%s/debug/pprof/", strconv.Itoa(int(port)))
	t.Run("create a new Pprof instance with enabled set to false and ensure it doesn't start", func(t *testing.T) {
		profiler := pprof.New(false, port, log)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		go func() {
			err := profiler.Run(ctx)
			require.NoError(t, err)
		}()

		err := waitForServerReady(url, time.Second)
		require.Error(t, err)
	})
}

func waitForServerReady(url string, timeout time.Duration) error {
	client := http.Client{}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()

	for {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
		if reqErr != nil {
			return reqErr
		}

		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}

		if time.Since(start) > timeout {
			return fmt.Errorf("server is not ready after %v", timeout)
		}

		time.Sleep(100 * time.Millisecond)
	}
}
