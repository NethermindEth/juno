package feeder

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTestServerRejectsPathTraversal(t *testing.T) {
	srv := newTestServer(t, &networks.Mainnet)
	t.Cleanup(srv.Close)
	ctx := t.Context()

	tests := []struct {
		name       string
		blockParam string
	}{
		// Traversal values that resolve to an existing fixture
		{"parent dir resolves to existing fixture", "../block/0"},
		{"deep traversal resolves to existing fixture", "../../mainnet/block/0"},
		// Percent-encoded traversal
		{"percent-encoded traversal", "..%2Fblock%2F0"},
		// Separators and backslash
		{"forward slash in name", "sub/dir"},
		{"backslash in name", "sub\\dir"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := fmt.Sprintf("%s/get_block?blockNumber=%s", srv.URL, tc.blockParam)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, http.NoBody)
			require.NoError(t, err)
			req.Header.Set("User-Agent", "Juno/v0.0.1-test Starknet Implementation")
			req.Header.Set("X-Throttling-Bypass", "API_KEY")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})
	}
}
