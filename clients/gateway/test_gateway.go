package gateway

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

// NewTestClient returns a client and a function to close a test server.
func NewTestClient(t *testing.T) *Client {
	srv := newTestServer(t)
	ua := "Juno/v0.0.1-test Starknet Implementation"
	apiKey := "API_KEY"
	t.Cleanup(srv.Close)

	return NewClient(srv.URL, utils.NewNopZapLogger()).WithUserAgent(ua).WithAPIKey(apiKey)
}

func newTestServer(t *testing.T) *httptest.Server {
	// As this is a test sever we are mimic response for one good and one bad request.
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, []string{"API_KEY"}, r.Header["X-Throttling-Bypass"])
		assert.Equal(t, []string{"Juno/v0.0.1-test Starknet Implementation"}, r.Header["User-Agent"])

		var bodyReader io.Reader = r.Body
		isGzip := r.Header.Get("Content-Encoding") == "gzip"
		if isGzip {
			gzReader, gzErr := gzip.NewReader(r.Body)
			if gzErr != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(gzErr.Error())) //nolint:errcheck // That's fine for the test server
				return
			}
			defer gzReader.Close()
			bodyReader = gzReader
		}

		b, err := io.ReadAll(bodyReader)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error())) //nolint:errcheck // Test code
			return
		}

		// Assert that large payloads are gzip-compressed
		if len(b) >= gzipMinSize {
			assert.True(t, isGzip, "expected Content-Encoding: gzip for payload of size %d", len(b))
		}

		// empty request: "{}"
		emptyReqLen := 4
		if string(b) == "null" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else if len(b) <= emptyReqLen {
			w.WriteHeader(http.StatusBadRequest)
			//nolint:errcheck // Test code
			w.Write([]byte(`{"code": "Malformed Request", "message": "empty request"}`))
			return
		}

		hash := felt.Random[felt.Felt]()
		resp := fmt.Sprintf(
			"{\"code\": \"TRANSACTION_RECEIVED\", \"transaction_hash\": %q, \"address\": %q}",
			hash.String(),
			hash.String(),
		)
		w.Write([]byte(resp)) //nolint:errcheck // Test code
	}))
}
