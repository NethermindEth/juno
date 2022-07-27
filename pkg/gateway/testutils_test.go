package gateway

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NethermindEth/juno/pkg/gateway/internal/models/stubs"
)

// testServer is a test server.
type testServer struct {
	*httptest.Server
}

// newTestServer creates a new test server.
func newTestServer(t *testing.T, h http.Handler) *testServer {
	return &testServer{httptest.NewServer(h)}
}

// get performs get requests on a testServer and returns the resulting
// HTTP status and response body as a string.
func (ts *testServer) get(t *testing.T, route string) (int, string) {
	res, err := ts.Client().Get(ts.URL + route)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	return res.StatusCode, string(bytes.TrimSpace(body))
}

// newTestGateway creates a new gateway that uses stubs instead of the
// database.
func newTestGateway(t *testing.T) *gateway {
	return &gateway{logger: NewNoOpLogger(), model: &stubs.Stub{}}
}
