package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

// scripted answers the n-th request with the n-th status, repeating the last one.
type scripted struct {
	*httptest.Server
	statuses    []int
	body        []byte
	encoding    string
	mutex       sync.Mutex
	requests    int
	lastHeaders http.Header
}

func newScripted(t *testing.T, statuses []int, body []byte, encoding string) *scripted {
	t.Helper()
	scripted := &scripted{statuses: statuses, body: body, encoding: encoding}
	scripted.Server = httptest.NewServer(http.HandlerFunc(scripted.handle))
	t.Cleanup(scripted.Close)
	return scripted
}

func (scripted *scripted) handle(writer http.ResponseWriter, request *http.Request) {
	scripted.mutex.Lock()
	defer scripted.mutex.Unlock()
	status := scripted.statuses[min(scripted.requests, len(scripted.statuses)-1)]
	scripted.requests++
	scripted.lastHeaders = request.Header.Clone()

	if scripted.encoding != "" {
		writer.Header().Set("Content-Encoding", scripted.encoding)
	}
	writer.WriteHeader(status)
	if status == http.StatusOK {
		writer.Write(scripted.body) //nolint:errcheck // test server
	}
}

func (scripted *scripted) count() int {
	scripted.mutex.Lock()
	defer scripted.mutex.Unlock()
	return scripted.requests
}

func (scripted *scripted) headers() http.Header {
	scripted.mutex.Lock()
	defer scripted.mutex.Unlock()
	return scripted.lastHeaders
}

func (scripted *scripted) source(t *testing.T, retries int, apiKey string) *source {
	t.Helper()
	feederURL, err := url.Parse(scripted.URL + feederPrefix)
	require.NoError(t, err)

	config := testConfig(0, 0, 0)
	config.network = &networks.Network{FeederURL: feederURL}
	config.captureRetries = retries
	config.apiKey = apiKey
	return newSource(config, log.NewNopZapLogger())
}

func TestRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"bad request", &statusError{code: http.StatusBadRequest}, false},
		{"wrapped bad request", fmt.Errorf("x: %w", &statusError{code: http.StatusBadRequest}), false},
		{"server error", &statusError{code: http.StatusInternalServerError}, true},
		{"too many requests", &statusError{code: http.StatusTooManyRequests}, true},
		{"transport error", errors.New("connection reset"), true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, retryable(test.err))
		})
	}
}

func TestEnsureGzipped(t *testing.T) {
	plain := []byte(`{"block_number": 1}`)
	tests := []struct {
		name     string
		body     []byte
		encoding string
		wantErr  string
	}{
		{"gzip kept as is", gzipped(t, plain), "gzip", ""},
		{"identity compressed", plain, "identity", ""},
		{"unset compressed", plain, "", ""},
		{"brotli rejected", plain, "br", `unsupported Content-Encoding "br"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ensureGzipped(test.body, test.encoding)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			unzipped, err := gunzip(got)
			require.NoError(t, err)
			require.Equal(t, plain, unzipped)
		})
	}
}

func TestDownload(t *testing.T) {
	plain := []byte(`{"block_number": 1}`)
	tests := []struct {
		name         string
		statuses     []int
		encoding     string
		body         []byte
		retries      int
		cancelled    bool
		wantErr      string
		wantErrIs    error
		wantRequests int
	}{
		{name: "first try", statuses: []int{http.StatusOK}, body: plain, retries: 2, wantRequests: 1},
		{
			name:         "retries transient failures",
			statuses:     []int{http.StatusInternalServerError, http.StatusTooManyRequests, http.StatusOK},
			body:         plain,
			retries:      2,
			wantRequests: 3,
		},
		{
			name:         "gives up after retries",
			statuses:     []int{http.StatusBadGateway},
			body:         plain,
			retries:      2,
			wantErr:      "after 3 attempts: unexpected status 502",
			wantRequests: 3,
		},
		{
			name:         "no retries",
			statuses:     []int{http.StatusBadGateway, http.StatusOK},
			body:         plain,
			wantErr:      "after 1 attempts: unexpected status 502",
			wantRequests: 1,
		},
		{
			name:         "bad request is permanent",
			statuses:     []int{http.StatusBadRequest, http.StatusOK},
			body:         plain,
			retries:      5,
			wantErr:      "unexpected status 400",
			wantRequests: 1,
		},
		{
			name:         "gzip passthrough",
			statuses:     []int{http.StatusOK},
			encoding:     "gzip",
			body:         gzipped(t, plain),
			wantRequests: 1,
		},
		{
			name:         "unsupported encoding",
			statuses:     []int{http.StatusOK},
			encoding:     "br",
			body:         plain,
			wantErr:      `unsupported Content-Encoding "br"`,
			wantRequests: 1,
		},
		{
			name:      "cancelled context",
			statuses:  []int{http.StatusOK},
			body:      plain,
			retries:   5,
			cancelled: true,
			wantErrIs: context.Canceled,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newScripted(t, test.statuses, test.body, test.encoding)
			source := server.source(t, test.retries, "")

			ctx := t.Context()
			if test.cancelled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			got, err := source.download(ctx, source.feederURL.JoinPath("get_block"))
			require.Equal(t, test.wantRequests, server.count())

			switch {
			case test.wantErrIs != nil:
				require.ErrorIs(t, err, test.wantErrIs)
			case test.wantErr != "":
				require.ErrorContains(t, err, test.wantErr)
			default:
				require.NoError(t, err)
				unzipped, err := gunzip(got)
				require.NoError(t, err)
				require.Equal(t, plain, unzipped)
			}
		})
	}
}

func TestDownloadHeaders(t *testing.T) {
	tests := []struct {
		name       string
		apiKey     string
		wantBypass []string
	}{
		{"without api key", "", nil},
		{"with api key", "secret", []string{"secret"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newScripted(t, []int{http.StatusOK}, []byte("{}"), "")
			source := server.source(t, 0, test.apiKey)

			_, err := source.download(t.Context(), source.feederURL.JoinPath("get_block"))
			require.NoError(t, err)

			headers := server.headers()
			require.Equal(t, []string{"gzip"}, headers.Values("Accept-Encoding"))
			require.Equal(t, test.wantBypass, headers.Values("X-Throttling-Bypass"))
		})
	}
}

func TestFetch(t *testing.T) {
	plain := []byte(`{"block_number": 5}`)
	resource := mustResource(t, block, blockKey{BlockNumber: 5})

	tests := []struct {
		name         string
		prepare      func(t *testing.T, dataset dataset)
		statuses     []int
		wantErr      string
		wantStored   bool
		wantRequests int
	}{
		{
			"dataset hit skips download",
			func(t *testing.T, dataset dataset) {
				t.Helper()
				require.NoError(t, dataset.write(resource.file, gzipped(t, plain)))
			},
			[]int{http.StatusBadGateway},
			"", true, 0,
		},
		{
			"miss downloads and stores",
			func(*testing.T, dataset) {},
			[]int{http.StatusOK},
			"", true, 1,
		},
		{
			"failed download stores nothing",
			func(*testing.T, dataset) {},
			[]int{http.StatusBadRequest},
			"get_block/5.json.gz: unexpected status 400", false, 1,
		},
		{
			"unreadable file is not masked",
			func(t *testing.T, dataset dataset) {
				t.Helper()
				require.NoError(t, os.MkdirAll(dataset.path(resource.file), directoryMode))
			},
			[]int{http.StatusOK},
			"is a directory", false, 0,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dataset := dataset{root: t.TempDir()}
			test.prepare(t, dataset)
			server := newScripted(t, test.statuses, plain, "")
			source := server.source(t, 0, "")

			got, err := source.fetch(t.Context(), dataset, resource)
			require.Equal(t, test.wantRequests, server.count())
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
			} else {
				require.NoError(t, err)
				unzipped, err := gunzip(got)
				require.NoError(t, err)
				require.Equal(t, plain, unzipped)
			}

			if !test.wantStored {
				require.NoFileExists(t, dataset.path(resource.file))
				return
			}
			stored, err := dataset.read(resource.file)
			require.NoError(t, err)
			require.Equal(t, got, stored)

			again, err := source.fetch(t.Context(), dataset, resource)
			require.NoError(t, err)
			require.Equal(t, got, again)
			require.Equal(t, test.wantRequests, server.count(), "second fetch must come from the dataset")
		})
	}
}

func TestNewSourceUsesConfiguredNetwork(t *testing.T) {
	feederURL, err := url.Parse("http://feeder.example/feeder_gateway/")
	require.NoError(t, err)
	config := testConfig(0, 0, 0)
	config.network = &networks.Network{FeederURL: feederURL}
	config.concurrency = 3
	config.captureRetries = 7
	config.apiKey = "k"

	source := newSource(config, log.NewNopZapLogger())
	require.Same(t, feederURL, source.feederURL)
	require.Equal(t, 7, source.retries)
	require.Equal(t, "k", source.apiKey)
	require.Equal(t, defaultCaptureTimeout, source.client.Timeout)

	transport, ok := source.client.Transport.(*http.Transport)
	require.True(t, ok)
	require.Equal(t, 3, transport.MaxIdleConnsPerHost)
	require.True(t, transport.DisableCompression, "gzip must reach the dataset untouched")
}
