package prof

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"gotest.tools/assert"
)

// testServer is a helper struct that represents a HTTP server.
type testServer struct {
	*httptest.Server
}

// newTestServer creates a new testServer with the given routes h.
func newTestServer(t *testing.T, h http.Handler) *testServer {
	return &testServer{httptest.NewServer(h)}
}

// get executes a GET request against the test server and returns the
// resulting HTTP status code.
func (ts *testServer) get(t *testing.T, url string) (code int) {
	t.Helper()

	r, err := ts.Client().Get(ts.URL + url)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	return r.StatusCode
}

// TestProfiles executes HTTP requests for the different profiles and
// checks the response for the expected status code.
func TestProfiles(t *testing.T) {
	const baseURL = "/debug/pprof/"

	tests := [...]struct {
		profile string
		want    int
	}{
		{
			"/", /* catch-all */
			http.StatusOK,
		},
		{
			baseURL + "nonexistent",
			http.StatusNotFound,
		},
		{
			baseURL + "allocs",
			http.StatusOK,
		},
		{
			baseURL + "block",
			http.StatusOK,
		},
		{
			baseURL + "cmdline",
			http.StatusOK,
		},
		{
			baseURL + "goroutine",
			http.StatusOK,
		},
		{
			baseURL + "heap",
			http.StatusOK,
		},
		{
			baseURL + "mutex",
			http.StatusOK,
		},
		{
			baseURL + "profile",
			http.StatusOK,
		},
		{
			baseURL + "threadcreate",
			http.StatusOK,
		},
		{
			baseURL + "trace",
			http.StatusOK,
		},
	}

	ts := newTestServer(t, register())
	defer ts.Close()

	for _, test := range tests {
		t.Run(strconv.Quote(test.profile), func(t *testing.T) {
			got := ts.get(t, test.profile)
			assert.Check(t, got == test.want, "%s status = %d, want %d", test.profile, got, test.want)
		})
	}
}
