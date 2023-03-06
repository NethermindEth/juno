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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPprofServerEnabled(t *testing.T) {
	port := uint16(9050)
	log := utils.NewNopZapLogger()
	url := fmt.Sprintf("http://localhost:%s/debug/pprof/", strconv.Itoa(int(port)))
	t.Run("create a new Pprof instance and run it", func(t *testing.T) {
		profiler := pprof.New(true, port, log)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		go func() {
			err := profiler.Run(ctx)
			require.NoError(t, err)
		}()

		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
		require.NoError(t, reqErr)
		resp, getErr := http.DefaultClient.Do(req)

		require.NoError(t, getErr)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
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
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
		require.NoError(t, reqErr)

		resp, getErr := http.DefaultClient.Do(req)

		require.Error(t, getErr)
		assert.Nil(t, resp)
		if resp != nil {
			resp.Body.Close()
		}
	})
}
