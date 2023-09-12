package node

import (
	"context"
	"net/http"
	"testing"

	"github.com/sourcegraph/conc"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := conc.NewWaitGroup()
	t.Cleanup(wg.Wait)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	httpService := makeHTTPService(6060, handler)
	wg.Go(func() {
		err := httpService.Run(ctx)
		// make sure http server shutdown properly
		require.NoError(t, err)
	})
}
