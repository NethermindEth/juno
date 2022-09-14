package prof

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

type testServer struct {
	*httptest.Server
}

func (ts *testServer) get(t *testing.T, url string) (code int) {
	t.Helper()

	r, err := ts.Client().Get(ts.URL + url)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	return r.StatusCode
}

func TestProfiles(t *testing.T) {
	const baseURL = "/debug/pprof/"

	tests := [...]struct {
		profile string
		want    int
	}{
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

	ts := testServer{httptest.NewServer(register())}
	defer ts.Close()

	for _, test := range tests {
		t.Run(strconv.Quote(test.profile), func(t *testing.T) {
			got := ts.get(t, test.profile)
			if got != test.want {
				t.Errorf("%s%s status = %d, want %d", baseURL, test.profile, got, test.want)
			}
		})
	}
}
