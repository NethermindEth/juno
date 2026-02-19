package feeder

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NewTestClient returns a client and a function to close a test server.
func NewTestClient(t testing.TB, network *utils.Network) *Client {
	srv := newTestServer(t, network)
	t.Cleanup(srv.Close)
	ua := "Juno/v0.0.1-test Starknet Implementation"
	apiKey := "API_KEY"

	c := NewClient(srv.URL).WithBackoff(NopBackoff).WithMaxRetries(0).WithUserAgent(ua).WithAPIKey(apiKey)
	c.client = &http.Client{
		Transport: &http.Transport{
			// On macOS tests often fail with the following error:
			//
			// "Get "http://127.0.0.1:xxxx/get_{feeder gateway method}?{arg}={value}": dial tcp 127.0.0.1:xxxx:
			//    connect: can't assign requested address"
			//
			// This error makes running local tests, in quick succession, difficult because we have to wait for the OS to release ports.
			// Sometimes the sync tests will hang because sync process will keep making requests if there was some error.
			// This problem is further exacerbated by having parallel tests.
			//
			// Increasing test client's idle conns allows for large concurrent requests to be made from a single test client.
			MaxIdleConnsPerHost: 1000,
		},
	}
	return c
}

func newTestServer(t testing.TB, network *utils.Network) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryMap, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		assert.Equal(t, []string{"API_KEY"}, r.Header["X-Throttling-Bypass"])
		assert.Equal(t, []string{"Juno/v0.0.1-test Starknet Implementation"}, r.Header["User-Agent"])

		require.NoError(t, err)

		dir, queryArg, err := resolveDirAndQueryArg(t, r.URL.Path, queryMap)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		fileName, found := queryMap[queryArg]
		if !found {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		dataPath, err := findTargetDirectory("clients/feeder/testdata")
		if err != nil {
			t.Fatalf("failed to find testdata directory: %v", err)
		}
		path := filepath.Join(dataPath, network.String(), dir, fileName[0]+".json")
		read, err := os.ReadFile(path) //nolint:gosec // G703: no danger, test environment
		if err != nil {
			handleNotFound(dir, queryArg, w)
			return
		}
		_, err = w.Write(read) //nolint:gosec // G705: no danger, test environment
		require.NoError(t, err, "failed to write response")
	}))
}

func resolveDirAndQueryArg(t testing.TB, path string, queryMap url.Values) (string, string, error) {
	t.Helper()

	var dir, queryArg string
	var err error
	const blockNumberArg = "blockNumber"

	switch {
	case strings.HasSuffix(path, "get_block"):
		dir = "block"
		queryArg = blockNumberArg

	case strings.HasSuffix(path, "get_state_update"):
		queryArg = blockNumberArg
		if includeBlock, ok := queryMap["includeBlock"]; !ok || len(includeBlock) == 0 {
			dir = "state_update"
		} else {
			dir = "state_update_with_block"
		}

	case strings.HasSuffix(path, "get_transaction"):
		dir = "transaction"
		queryArg = "transactionHash"

	case strings.HasSuffix(path, "get_class_by_hash"):
		dir = "class"
		queryArg = "classHash"

	case strings.HasSuffix(path, "get_compiled_class_by_class_hash"):
		dir = "compiled_class"
		queryArg = "classHash"

	case strings.HasSuffix(path, "get_public_key"):
		dir = "public_key"
		queryArg = "pk"
		queryMap[queryArg] = []string{queryArg} // keep previous behaviour

	case strings.HasSuffix(path, "get_signature"):
		dir = "signature"
		queryArg = blockNumberArg

	case strings.HasSuffix(path, "get_block_traces"):
		dir = "traces"
		queryArg = "blockHash"

	case strings.HasSuffix(path, "get_preconfirmed_block"):
		dir = "pre_confirmed"
		queryArg = blockNumberArg

	default:
		err = errors.New("unknown endpoint")
	}
	return dir, queryArg, err
}

func handleNotFound(dir, queryArg string, w http.ResponseWriter) {
	// If a transaction data is missing, respond with
	// {"finality_status": "NOT_RECEIVED", "status": "NOT_RECEIVED"}
	// instead of 404 as per real test server behaviour.
	if dir == "transaction" && queryArg == "transactionHash" {
		w.Write([]byte("{\"finality_status\": \"NOT_RECEIVED\", \"status\": \"NOT_RECEIVED\"}")) //nolint:errcheck
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
}

// buildQueryString builds the query url with encoded parameters
func (c *Client) buildQueryString(endpoint string, args map[string]string) string {
	base, err := url.Parse(c.url)
	if err != nil {
		panic("Malformed feeder base URL")
	}

	base.Path += endpoint

	params := url.Values{}
	for k, v := range args {
		params.Add(k, v)
	}
	base.RawQuery = params.Encode()

	return base.String()
}

func findTargetDirectory(targetRelPath string) (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		targetPath := filepath.Join(root, targetRelPath)
		if _, err := os.Stat(targetPath); err == nil {
			return targetPath, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		newRoot := filepath.Dir(root)
		if newRoot == root {
			return "", os.ErrNotExist
		}
		root = newRoot
	}
}
